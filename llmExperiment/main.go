// Experimento da AV2: o grafo ajuda a LLM a responder? Roda as perguntas do
// gabarito (data/queries_av2.json) pela mesma bridge da interface, em quatro
// condições de contexto, e grava uma linha por resposta.
//
//	A  top-5 do TF-IDF (o sistema da AV1)
//	B  top-5 + 5 relacionadas pelo grafo (uma linha cada)
//	C  top-10 do TF-IDF (controle: mais contexto sem grafo)
//	D  top-5 + ordem de chamadas
//
//	go build -o bin/mcp ./ui/mcp
//	go run ./llmExperiment -models qwen2.5:3b,qwen2.5:7b,qwen2.5:14b
//	go run ./llmExperiment -models qwen2.5:3b -reps 3 -out artefatos/experimento_llm_tempo
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strings"
	"time"

	"paa/benchmark/hardware"
	"paa/searchEngine/engine"
	"paa/searchEngine/types"
	"paa/searchEngine/utils"
	"paa/ui/bridge"
)

// question é uma pergunta do gabarito. Prerequisites são os passos que vêm
// antes da operação; cada passo lista as chamadas que servem para ele.
type question struct {
	Pergunta      string     `json:"pergunta"`
	Query         string     `json:"query"`
	Relevant      []string   `json:"relevant"`
	Prerequisites [][]string `json:"prerequisites"`
}

type condition struct {
	Name    string
	Options bridge.SearchOptions
}

var conditions = []condition{
	{"A", bridge.SearchOptions{K: 5}},
	{"B", bridge.SearchOptions{K: 5, Related: 5, Hops: 3}},
	{"C", bridge.SearchOptions{K: 10}},
	{"D", bridge.SearchOptions{K: 5, Plan: true}},
}

// toolOutput é a parte da saída estruturada da tool MCP que o experimento usa.
type toolOutput struct {
	Results []struct{ ID, Method, Path string } `json:"results"`
	Related []struct{ ID, Method, Path string } `json:"related"`
	Plans   []struct {
		Steps []struct{ ID, Method, Path string } `json:"steps"`
	} `json:"plans"`
}

type record struct {
	Model           string     `json:"modelo"`
	Condition       string     `json:"condicao"`
	Rep             int        `json:"repeticao"`
	Question        string     `json:"pergunta"`
	Relevant        []string   `json:"relevantes"`
	Prerequisites   [][]string `json:"pre_requisitos"`
	Query           string     `json:"consulta"`
	ContextIDs      []string   `json:"contexto"`
	ContextChars    int        `json:"contexto_chars"`
	AnswerInContext bool       `json:"resposta_no_contexto"`
	Cited           []string   `json:"endpoints_citados"`
	Correct         bool       `json:"acertou"`
	Invented        []string   `json:"inventados"`
	NeedsPrereq     bool       `json:"precisa_pre_requisito"`
	CitedPrereq     bool       `json:"citou_pre_requisito"`
	ContextUse      string     `json:"uso_do_contexto"`
	PromptTokens    int        `json:"tokens_entrada"`
	OutputTokens    int        `json:"tokens_saida"`
	LLMMillis       int64      `json:"llm_ms"`
	Note            string     `json:"aviso,omitempty"`
	Answer          string     `json:"resposta"`
}

// call é uma chamada à API escrita na resposta, como "MÉTODO /path" ou como
// "curl -X MÉTODO https://api.github.com/path".
type call struct{ Method, Path string }

var (
	endpointPattern = regexp.MustCompile(`\b(GET|POST|PUT|PATCH|DELETE)\s+(/[^\s` + "`" + `)\]*,"']+)`)
	curlPattern     = regexp.MustCompile(`(?:-X\s*|--request\s+)(GET|POST|PUT|PATCH|DELETE)\b[^\n]{0,120}?https?://api\.github\.com(/[^\s"'` + "`" + `)?]+)|https?://api\.github\.com(/[^\s"'` + "`" + `)?]+)[^\n]{0,60}?-X\s*(GET|POST|PUT|PATCH|DELETE)`)
)

func main() {
	models := flag.String("models", "qwen2.5:3b,qwen2.5:7b,qwen2.5:14b", "modelos do Ollama, separados por vírgula")
	only := flag.String("conditions", "A,B,C,D", "condições a rodar")
	reps := flag.Int("reps", 1, "repetições de cada pergunta")
	seed := flag.Int("seed", 42, "semente da geração; 0 = aleatória")
	questionsPath := flag.String("questions", "data/queries_av2.json", "gabarito")
	out := flag.String("out", "artefatos/experimento_llm", "prefixo dos arquivos de saída (.csv, .jsonl, .txt)")
	ollama := flag.String("ollama", "http://localhost:11434", "endereço do Ollama")
	numCtx := flag.Int("num_ctx", 8192, "janela de contexto do modelo")
	mcpBin := flag.String("mcp", "bin/mcp", "binário do servidor MCP")
	rescore := flag.Bool("rescore", false, "recalcula as métricas das respostas já gravadas em <out>.jsonl, sem chamar a LLM")
	flag.Parse()

	questions := loadQuestions(utils.Resolve(*questionsPath))
	eng := loadEngine()
	if *rescore {
		rescoreRecords(*out, questions, eng.Corpus.Docs)
		return
	}
	ctx := context.Background()
	b, err := bridge.Connect(ctx, *ollama, *numCtx, utils.Resolve(*mcpBin), nil)
	check(err)
	defer b.Close()
	b.Seed = *seed

	selected := selectConditions(*only)
	modelList := strings.Split(*models, ",")
	total := len(modelList) * len(selected) * len(questions) * *reps
	header := environment(modelList, selected, len(questions), *reps, *seed, total)
	check(os.WriteFile(utils.Resolve(*out+".txt"), []byte(header), 0o644))
	fmt.Print(header)

	csvFile, jsonlFile := create(*out+".csv"), create(*out+".jsonl")
	defer csvFile.Close()
	defer jsonlFile.Close()
	fmt.Fprintln(csvFile, csvHeader())

	done := 0
	for _, model := range modelList {
		warmUp(ctx, b, model)
		for _, cond := range selected {
			for rep := 1; rep <= *reps; rep++ {
				for _, q := range questions {
					r := ask(ctx, b, eng, model, cond, rep, q)
					fmt.Fprintln(csvFile, r.csvLine())
					line, _ := json.Marshal(r)
					fmt.Fprintln(jsonlFile, string(line))
					done++
					fmt.Fprintf(os.Stderr, "[%d/%d] %s %s rep%d acertou=%v inventou=%v %dms  %s\n",
						done, total, model, cond.Name, rep, r.Correct, len(r.Invented) > 0, r.LLMMillis, q.Pergunta)
				}
			}
		}
	}
}

func loadQuestions(path string) []question {
	raw, err := os.ReadFile(path)
	check(err)
	var qs []question
	check(json.Unmarshal(raw, &qs))
	return qs
}

func loadEngine() *engine.Engine {
	opts := engine.DefaultOptions()
	opts.Graph = true
	e, err := engine.Load(opts)
	check(err)
	return e
}

func selectConditions(list string) []condition {
	var out []condition
	for _, c := range conditions {
		if slices.Contains(strings.Split(list, ","), c.Name) {
			out = append(out, c)
		}
	}
	return out
}

// warmUp carrega o modelo na GPU antes de medir, para a primeira resposta não
// pagar o tempo de carga.
func warmUp(ctx context.Context, b *bridge.Bridge, model string) {
	history := []bridge.Message{{Role: "user", Content: "list branches"}}
	_, err := b.Ask(ctx, model, history, bridge.SearchOptions{K: 1}, func(bridge.Event) {})
	check(err)
}

func ask(ctx context.Context, b *bridge.Bridge, eng *engine.Engine, model string, cond condition, rep int, q question) record {
	r := record{Model: model, Condition: cond.Name, Rep: rep, Question: q.Pergunta, Relevant: q.Relevant, Prerequisites: q.Prerequisites}
	var output toolOutput
	var stats *bridge.Stats
	history := []bridge.Message{{Role: "user", Content: q.Pergunta}}
	answer, err := b.Ask(ctx, model, history, cond.Options, func(e bridge.Event) {
		switch e.Type {
		case "tool_call":
			r.Query, _ = e.Args["query"].(string)
		case "tool_result":
			r.ContextChars = len(e.Text)
			raw, _ := json.Marshal(e.Data)
			_ = json.Unmarshal(raw, &output)
		case "done":
			stats = e.Stats
		}
	})
	if err != nil {
		r.Note = err.Error()
	}
	r.Answer = answer
	if stats != nil {
		r.PromptTokens, r.OutputTokens, r.LLMMillis = stats.PromptTokens, stats.OutputTokens, stats.LLMTime.Milliseconds()
		if stats.Note != "" {
			r.Note = stats.Note
		}
	}
	r.ContextIDs = contextIDs(output)
	score(&r, eng.Corpus.Docs)
	return r
}

// contextIDs lista, sem repetir, as operações que foram para o contexto da LLM:
// os resultados, as relacionadas e os passos dos planos.
func contextIDs(output toolOutput) []string {
	var ids []string
	add := func(id string) {
		if !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}
	for _, x := range output.Results {
		add(x.ID)
	}
	for _, x := range output.Related {
		add(x.ID)
	}
	for _, plan := range output.Plans {
		for _, x := range plan.Steps {
			add(x.ID)
		}
	}
	return ids
}

// score aplica os quatro critérios determinísticos à resposta:
//
//	acertou              uma operação do gabarito aparece em qualquer lugar do texto;
//	                     sem resposta no corpus, acerta quem não cita nenhuma chamada
//	inventados           chamadas do texto que não existem no corpus (método + path)
//	citou_pre_requisito  com a operação certa, cada passo do pré-requisito aparece
//	uso_do_contexto      cruza o acerto com a operação certa estar no contexto
func score(r *record, docs []types.Document) {
	calls := callsIn(r.Answer)
	r.Cited, r.Invented = nil, nil
	for _, c := range calls {
		r.Cited = append(r.Cited, c.Method+" "+c.Path)
		if !existsInCorpus(c, docs) {
			r.Invented = append(r.Invented, c.Method+" "+c.Path)
		}
	}

	r.AnswerInContext = false
	for _, id := range r.Relevant {
		if slices.Contains(r.ContextIDs, id) {
			r.AnswerInContext = true
		}
	}

	if len(r.Relevant) == 0 {
		r.Correct = len(calls) == 0
	} else {
		r.Correct = anyAppears(r.Relevant, calls, docs)
	}

	r.NeedsPrereq = len(r.Prerequisites) > 0
	r.CitedPrereq = r.NeedsPrereq && r.Correct
	for _, step := range r.Prerequisites {
		if !anyAppears(step, calls, docs) {
			r.CitedPrereq = false
		}
	}

	r.ContextUse = contextUse(r)
}

func contextUse(r *record) string {
	switch {
	case len(r.Relevant) == 0:
		return ""
	case r.Correct && r.AnswerInContext:
		return "trouxe_e_usou"
	case r.Correct:
		return "de_memoria"
	case r.AnswerInContext:
		return "trouxe_e_nao_usou"
	default:
		return "busca_falhou"
	}
}

// callsIn extrai as chamadas escritas como "MÉTODO /path" e as dos exemplos de
// curl, na ordem em que aparecem e sem repetir.
func callsIn(answer string) []call {
	var calls []call
	add := func(method, path string) {
		path = strings.TrimRight(strings.SplitN(path, "?", 2)[0], ".:;")
		c := call{Method: method, Path: path}
		if !slices.Contains(calls, c) {
			calls = append(calls, c)
		}
	}
	for _, m := range endpointPattern.FindAllStringSubmatch(answer, -1) {
		add(m[1], m[2])
	}
	for _, m := range curlPattern.FindAllStringSubmatch(answer, -1) {
		if m[1] != "" {
			add(m[1], m[2])
		} else {
			add(m[4], m[3])
		}
	}
	return calls
}

// matchesTemplate compara o path escrito com o do corpus segmento a segmento:
// os segmentos fixos têm que ser iguais e "{parâmetro}" aceita qualquer valor
// ({owner}, <OWNER>, octocat).
func matchesTemplate(template, path string) bool {
	want := strings.Split(strings.Trim(template, "/"), "/")
	got := strings.Split(strings.Trim(path, "/"), "/")
	if len(want) != len(got) {
		return false
	}
	for i := range want {
		if !strings.HasPrefix(want[i], "{") && want[i] != got[i] {
			return false
		}
	}
	return true
}

func isCall(doc types.Document, c call) bool {
	return strings.EqualFold(doc.Method, c.Method) && matchesTemplate(doc.Path, c.Path)
}

func existsInCorpus(c call, docs []types.Document) bool {
	for _, doc := range docs {
		if isCall(doc, c) {
			return true
		}
	}
	return false
}

// anyAppears diz se alguma das operações aparece entre as chamadas do texto.
func anyAppears(ids []string, calls []call, docs []types.Document) bool {
	for _, doc := range docs {
		if !slices.Contains(ids, doc.ID) {
			continue
		}
		for _, c := range calls {
			if isCall(doc, c) {
				return true
			}
		}
	}
	return false
}

// rescoreRecords relê <out>.jsonl, aplica de novo os critérios com o gabarito
// atual e regrava o .jsonl e o .csv. O cabeçalho .txt ganha uma linha dizendo
// quando e com que commit a reavaliação foi feita.
func rescoreRecords(out string, questions []question, docs []types.Document) {
	byQuestion := map[string]question{}
	for _, q := range questions {
		byQuestion[q.Pergunta] = q
	}
	raw, err := os.ReadFile(utils.Resolve(out + ".jsonl"))
	check(err)
	var records []record
	for line := range strings.SplitSeq(strings.TrimSpace(string(raw)), "\n") {
		var r record
		check(json.Unmarshal([]byte(line), &r))
		q, ok := byQuestion[r.Question]
		if !ok {
			check(fmt.Errorf("pergunta fora do gabarito: %q", r.Question))
		}
		r.Relevant, r.Prerequisites = q.Relevant, q.Prerequisites
		score(&r, docs)
		records = append(records, r)
	}

	csvFile, jsonlFile := create(out+".csv"), create(out+".jsonl")
	defer csvFile.Close()
	defer jsonlFile.Close()
	fmt.Fprintln(csvFile, csvHeader())
	for _, r := range records {
		fmt.Fprintln(csvFile, r.csvLine())
		line, _ := json.Marshal(r)
		fmt.Fprintln(jsonlFile, string(line))
	}

	txt, err := os.OpenFile(utils.Resolve(out+".txt"), os.O_APPEND|os.O_WRONLY, 0o644)
	check(err)
	defer txt.Close()
	fmt.Fprintf(txt, "reavaliacao: %s, commit %s, comando: %s, %d respostas\n",
		time.Now().Format(time.RFC3339), commit(), strings.Join(os.Args, " "), len(records))
	fmt.Fprintf(os.Stderr, "%d respostas reavaliadas\n", len(records))
}

func csvHeader() string {
	return "modelo,condicao,repeticao,pergunta,consulta,resposta_no_contexto,endpoints_citados,acertou,inventados,precisa_pre_requisito,citou_pre_requisito,uso_do_contexto,contexto_itens,contexto_chars,tokens_entrada,tokens_saida,llm_ms,aviso"
}

func (r record) csvLine() string {
	fields := []string{
		r.Model, r.Condition, fmt.Sprint(r.Rep), r.Question, r.Query,
		fmt.Sprint(r.AnswerInContext), strings.Join(r.Cited, " | "), fmt.Sprint(r.Correct), strings.Join(r.Invented, " | "),
		fmt.Sprint(r.NeedsPrereq), fmt.Sprint(r.CitedPrereq), r.ContextUse, fmt.Sprint(len(r.ContextIDs)), fmt.Sprint(r.ContextChars),
		fmt.Sprint(r.PromptTokens), fmt.Sprint(r.OutputTokens), fmt.Sprint(r.LLMMillis), r.Note,
	}
	for i, f := range fields {
		fields[i] = csvQuote(f)
	}
	return strings.Join(fields, ",")
}

func csvQuote(s string) string {
	if strings.ContainsAny(s, ",\"\n") {
		return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
	}
	return s
}

func environment(models []string, conds []condition, questions, reps, seed, total int) string {
	hw := hardware.Detect()
	var names []string
	for _, c := range conds {
		names = append(names, c.Name)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "=== Experimento LLM (AV2, trilha de grafos) ===\n")
	fmt.Fprintf(&b, "timestamp: %s\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(&b, "commit: %s\n", commit())
	fmt.Fprintf(&b, "os: %s/%s\n", hw.OS, hw.Arch)
	fmt.Fprintf(&b, "go_version: %s\n", hw.Go)
	fmt.Fprintf(&b, "cpu: %s\n", hw.CPU)
	fmt.Fprintf(&b, "ram: %s\n", hw.RAMGiB)
	fmt.Fprintf(&b, "comando: %s\n", strings.Join(os.Args, " "))
	fmt.Fprintf(&b, "modelos: %s\n", strings.Join(models, ", "))
	fmt.Fprintf(&b, "condicoes: %s (A=top-5, B=top-5+5 relacionadas, C=top-10, D=top-5+plano)\n", strings.Join(names, ","))
	fmt.Fprintf(&b, "perguntas: %d   repeticoes: %d   semente: %d   respostas: %d\n\n", questions, reps, seed, total)
	return b.String()
}

// commit é o commit atual, marcado com "+alterações" se houver mudanças não commitadas.
func commit() string {
	root := utils.Resolve(".")
	hash, err := exec.Command("git", "-C", root, "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return "desconhecido"
	}
	status, _ := exec.Command("git", "-C", root, "status", "--porcelain", "--untracked-files=no").Output()
	if len(strings.TrimSpace(string(status))) > 0 {
		return strings.TrimSpace(string(hash)) + "+alterações"
	}
	return strings.TrimSpace(string(hash))
}

func create(path string) *os.File {
	f, err := os.Create(utils.Resolve(path))
	check(err)
	return f
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
