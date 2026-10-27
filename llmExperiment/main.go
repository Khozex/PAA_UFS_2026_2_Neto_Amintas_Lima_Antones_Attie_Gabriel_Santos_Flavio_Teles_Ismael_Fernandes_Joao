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

type question struct {
	Pergunta string   `json:"pergunta"`
	Query    string   `json:"query"`
	Relevant []string `json:"relevant"`
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
	Model           string   `json:"modelo"`
	Condition       string   `json:"condicao"`
	Rep             int      `json:"repeticao"`
	Question        string   `json:"pergunta"`
	Relevant        []string `json:"relevantes"`
	Query           string   `json:"consulta"`
	ContextIDs      []string `json:"contexto"`
	ContextChars    int      `json:"contexto_chars"`
	AnswerInContext bool     `json:"resposta_no_contexto"`
	Cited           string   `json:"endpoint_citado"`
	CitedID         string   `json:"id_citado"`
	Correct         bool     `json:"acertou"`
	Hallucinated    bool     `json:"alucinou"`
	NeedsPrereq     bool     `json:"precisa_pre_requisito"`
	ExplainedPrereq bool     `json:"explicou_pre_requisito"`
	PromptTokens    int      `json:"tokens_entrada"`
	OutputTokens    int      `json:"tokens_saida"`
	LLMMillis       int64    `json:"llm_ms"`
	Note            string   `json:"aviso,omitempty"`
	Answer          string   `json:"resposta"`
}

var endpointPattern = regexp.MustCompile(`\b(GET|POST|PUT|PATCH|DELETE)\s+(/[^\s` + "`" + `)\]*,]+)`)

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
	flag.Parse()

	questions := loadQuestions(utils.Resolve(*questionsPath))
	eng := loadEngine()
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
					fmt.Fprintf(os.Stderr, "[%d/%d] %s %s rep%d acertou=%v alucinou=%v %dms  %s\n",
						done, total, model, cond.Name, rep, r.Correct, r.Hallucinated, r.LLMMillis, q.Pergunta)
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
	r := record{Model: model, Condition: cond.Name, Rep: rep, Question: q.Pergunta, Relevant: q.Relevant}
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
	evaluate(&r, output, eng)
	return r
}

// evaluate preenche as métricas automáticas. O endpoint citado é o primeiro
// "MÉTODO /path" da resposta, a mesma regra que a interface usa.
func evaluate(r *record, output toolOutput, eng *engine.Engine) {
	inContext := map[string]bool{}
	add := func(id, method, path string) {
		key := strings.ToUpper(method) + " " + path
		if !inContext[key] {
			inContext[key] = true
			r.ContextIDs = append(r.ContextIDs, id)
		}
	}
	for _, x := range output.Results {
		add(x.ID, x.Method, x.Path)
	}
	for _, x := range output.Related {
		add(x.ID, x.Method, x.Path)
	}
	for _, plan := range output.Plans {
		for _, x := range plan.Steps {
			add(x.ID, x.Method, x.Path)
		}
	}
	for _, id := range r.Relevant {
		if slices.Contains(r.ContextIDs, id) {
			r.AnswerInContext = true
		}
	}

	if m := endpointPattern.FindStringSubmatch(r.Answer); m != nil {
		r.Cited = m[1] + " " + strings.TrimRight(m[2], ".:;")
		r.CitedID = docByEndpoint(eng, r.Cited)
		r.Hallucinated = !inContext[r.Cited]
	}
	if len(r.Relevant) == 0 {
		r.Correct = r.Cited == ""
	} else {
		r.Correct = slices.Contains(r.Relevant, r.CitedID)
	}

	if prereqs := prerequisitePaths(eng, r.CitedID); len(prereqs) > 0 {
		r.NeedsPrereq = true
		for _, path := range prereqs {
			if mentionsCall(r.Answer, path) {
				r.ExplainedPrereq = true
			}
		}
	}
}

// mentionsCall diz se a resposta cita a chamada pelo path inteiro, com ou sem
// "GET" antes. O path tem que terminar ali: o da própria operação começa com o
// do pré-requisito (…/releases/{release_id} contém …/releases), e só procurar o
// texto marcava toda resposta como "explicou".
func mentionsCall(answer, path string) bool {
	pattern := regexp.MustCompile(`(?:GET\s+)?` + regexp.QuoteMeta(path) + `(?:[^/\w{]|$)`)
	return pattern.MatchString(answer)
}

func docByEndpoint(eng *engine.Engine, endpoint string) string {
	for _, doc := range eng.Corpus.Docs {
		if strings.ToUpper(doc.Method)+" "+doc.Path == endpoint {
			return doc.ID
		}
	}
	return ""
}

// prerequisitePaths devolve os paths das chamadas que o plano do grafo põe
// antes da operação citada; vazio se ela não precisa de nenhum id de outra chamada.
func prerequisitePaths(eng *engine.Engine, id string) []string {
	if id == "" {
		return nil
	}
	for i := range eng.Corpus.Docs {
		if eng.Corpus.Docs[i].ID != id {
			continue
		}
		plans, _, err := eng.Graph.Plans([]types.Hit{{Doc: &eng.Corpus.Docs[i]}})
		if err != nil || len(plans) == 0 {
			return nil
		}
		var paths []string
		for _, doc := range plans[0].Prerequisites() {
			paths = append(paths, doc.Path)
		}
		return paths
	}
	return nil
}

func csvHeader() string {
	return "modelo,condicao,repeticao,pergunta,consulta,resposta_no_contexto,endpoint_citado,id_citado,acertou,alucinou,precisa_pre_requisito,explicou_pre_requisito,contexto_itens,contexto_chars,tokens_entrada,tokens_saida,llm_ms,aviso"
}

func (r record) csvLine() string {
	fields := []string{
		r.Model, r.Condition, fmt.Sprint(r.Rep), r.Question, r.Query,
		fmt.Sprint(r.AnswerInContext), r.Cited, r.CitedID, fmt.Sprint(r.Correct), fmt.Sprint(r.Hallucinated),
		fmt.Sprint(r.NeedsPrereq), fmt.Sprint(r.ExplainedPrereq), fmt.Sprint(len(r.ContextIDs)), fmt.Sprint(r.ContextChars),
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
