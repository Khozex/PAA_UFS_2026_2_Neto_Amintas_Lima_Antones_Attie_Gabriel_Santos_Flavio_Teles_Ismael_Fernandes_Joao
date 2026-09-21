// Package bridge liga um modelo do Ollama ao servidor MCP do searchEngine.
// O Ollama não fala MCP: o bridge sobe o servidor (ui/mcp) por stdio, anuncia
// as tools dele ao modelo, executa as tool calls que o modelo pedir e devolve
// o resultado até a resposta final. É usado por ui/chat (terminal) e ui/server (web).
package bridge

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"paa/searchEngine/utils"
)

const rewritePrompt = `Turn the user's latest question into a short English search query (2 to 4 words) for the GitHub REST API documentation: the action and the resource, nothing else. Examples: "cancel workflow run", "create organization repository", "list issues", "delete release". Do not add words like "repository", "GitHub" or "API" unless the question is about them. Use the earlier turns only to resolve references like "and to delete it?". Reply with the query only: no quotes, no punctuation, no explanation.`

const answerPrompt = `You are an assistant for the GitHub REST API. Answer using ONLY the documentation excerpts in the user's message; never invent endpoints or parameters. The excerpts come from a keyword search ranked by score, so the best match is not always the first one: pick the operation whose description actually does what the question asks.

Write the answer in Markdown, in this order, without numbering the sections and without repeating these instructions:
- One sentence with the endpoint as inline code (METHOD /path between single backticks) and what it does.
- A bulleted list of the parameters that matter, taken from the excerpt: path parameters first, then the main body or query parameters. One bullet per parameter: the name in inline code, then a short description. Skip parameters that are not in the excerpt.
- A fenced code block tagged bash with a minimal curl example. Use exactly this shape, filling in the method, path and, for POST/PUT/PATCH, a small JSON body with the required fields:
  curl -X METHOD -H "Authorization: Bearer <TOKEN>" -H "Accept: application/vnd.github+json" https://api.github.com/path
- A last line "Docs: <url>" with the URL copied exactly from the "Docs:" line of that excerpt.
Describe exactly one operation: the one that answers the question. Do not list, summarize or mention the other excerpts. If none of the excerpts does what was asked, say the search found nothing relevant and suggest a rephrasing; do not recommend an endpoint that is not in the excerpts.

The excerpts are in English; the answer must be in the language given at the end of this message.`

// MaxRounds é o número de chamadas ao modelo por pergunta: reescrita + resposta.
const MaxRounds = 2

// Message é uma mensagem da conversa como o Ollama a vê.
type Message struct {
	Role      string     `json:"role"`
	Content   string     `json:"content"`
	ToolName  string     `json:"tool_name,omitempty"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

type ToolCall struct {
	Function struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	} `json:"function"`
}

// Event é o que o bridge emite durante uma pergunta, na ordem em que acontece.
type Event struct {
	Type   string         `json:"type"` // tool_call | tool_result | token | done | error
	Name   string         `json:"name,omitempty"`
	Args   map[string]any `json:"args,omitempty"`
	Text   string         `json:"text,omitempty"`
	Data   any            `json:"data,omitempty"` // tool_result: structuredContent do MCP
	Stats  *Stats         `json:"stats,omitempty"`
	Answer string         `json:"answer,omitempty"` // done: resposta final completa
}

type Stats struct {
	Rounds       int           `json:"rounds"`
	ToolCalls    int           `json:"tool_calls"`
	PromptTokens int           `json:"prompt_tokens"`
	OutputTokens int           `json:"output_tokens"`
	LLMTime      time.Duration `json:"llm_ms"`
	Note         string        `json:"note,omitempty"` // aviso: repetição ou timeout
}

func (s Stats) MarshalJSON() ([]byte, error) {
	type alias Stats
	return json.Marshal(struct {
		alias
		LLMTime int64 `json:"llm_ms"`
	}{alias(s), s.LLMTime.Milliseconds()})
}

type ollamaTool struct {
	Type     string `json:"type"`
	Function struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Parameters  any    `json:"parameters"`
	} `json:"function"`
}

// SearchOptions é o que a interface pode mudar por pergunta; vai direto
// como argumentos da tool MCP. Campos vazios usam o padrão do servidor MCP.
type SearchOptions struct {
	K             int    `json:"k,omitempty"`
	Config        string `json:"config,omitempty"`
	Select        string `json:"select,omitempty"`
	OrderStrategy string `json:"order_strategy,omitempty"`
}

func (o SearchOptions) arguments(query string) map[string]any {
	args := map[string]any{"query": query}
	if o.K > 0 {
		args["k"] = o.K
	}
	if o.Config != "" {
		args["config"] = o.Config
	}
	if o.Select != "" {
		args["select"] = o.Select
	}
	if o.OrderStrategy != "" {
		args["order_strategy"] = o.OrderStrategy
	}
	return args
}

type Bridge struct {
	OllamaURL string
	NumCtx    int
	K         int // k padrão quando a pergunta não traz SearchOptions.K
	client    *http.Client
	session   *mcp.ClientSession
	tools     []ollamaTool
	ToolNames []string
}

// Connect sobe o servidor MCP e lista as tools. Com mcpBin vazio roda
// `go run ./ui/mcp <serverArgs>` na raiz do módulo; senão executa o binário.
func Connect(ctx context.Context, ollamaURL string, numCtx int, mcpBin string, serverArgs []string) (*Bridge, error) {
	var cmd *exec.Cmd
	if mcpBin != "" {
		cmd = exec.Command(mcpBin, serverArgs...)
	} else {
		cmd = exec.Command(goBinary(), append([]string{"run", "./ui/mcp"}, serverArgs...)...)
	}
	cmd.Dir = utils.Resolve(".")
	cmd.Stderr = os.Stderr
	client := mcp.NewClient(&mcp.Implementation{Name: "paa-bridge", Version: "0.1.0"}, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		return nil, fmt.Errorf("subindo o servidor MCP: %w", err)
	}
	list, err := session.ListTools(ctx, nil)
	if err != nil {
		return nil, err
	}
	b := &Bridge{
		OllamaURL: strings.TrimRight(ollamaURL, "/"),
		NumCtx:    numCtx,
		K:         5,
		client:    &http.Client{Timeout: 10 * time.Minute},
		session:   session,
	}
	for _, t := range list.Tools {
		var ot ollamaTool
		ot.Type = "function"
		ot.Function.Name = t.Name
		ot.Function.Description = t.Description
		ot.Function.Parameters = t.InputSchema
		b.tools = append(b.tools, ot)
		b.ToolNames = append(b.ToolNames, t.Name)
	}
	if len(b.tools) == 0 {
		return nil, fmt.Errorf("o servidor MCP não expôs nenhuma tool")
	}
	return b, nil
}

func (b *Bridge) Close() error { return b.session.Close() }

// goBinary acha o go do PATH ou, se não houver, o do toolchain que compilou este programa.
func goBinary() string {
	if path, err := exec.LookPath("go"); err == nil {
		return path
	}
	return filepath.Join(runtime.GOROOT(), "bin", "go")
}

// Ask responde a última mensagem de user em history em duas etapas fixas:
//  1. o modelo reescreve a pergunta como consulta curta em inglês;
//  2. a ponte chama a tool MCP com essa consulta (texto completo das operações)
//     e o modelo responde usando só os trechos recuperados.
//
// A busca é obrigatória: não depende de o modelo decidir chamar a tool.
// Do histórico só as perguntas anteriores do usuário entram, como contexto para
// resolver referências; as respostas anteriores ficam de fora porque modelos
// pequenos copiam paths e idioma delas em vez de usar os trechos novos.
func (b *Bridge) Ask(ctx context.Context, model string, history []Message, search SearchOptions, emit func(Event)) (string, error) {
	if len(history) == 0 || history[len(history)-1].Role != "user" {
		return "", fmt.Errorf("a última mensagem precisa ser do usuário")
	}
	question := history[len(history)-1].Content
	var stats Stats

	// 1. reescrita da consulta
	stats.Rounds++
	start := time.Now()
	rewrite, err := b.stream(ctx, model, []Message{
		{Role: "system", Content: rewritePrompt},
		{Role: "user", Content: earlierQuestions(history) + "Latest question: " + question},
	}, rewriteGen, nil)
	stats.LLMTime += time.Since(start)
	if err == ErrRepetition {
		err = nil // a consulta parcial serve; cleanQuery fica com a primeira linha
	}
	if err != nil {
		emit(Event{Type: "error", Text: err.Error()})
		return "", err
	}
	stats.PromptTokens += rewrite.PromptEvalCount
	stats.OutputTokens += rewrite.EvalCount
	query := cleanQuery(rewrite.Message.Content)
	if query == "" {
		query = question
	}

	// 2. busca obrigatória via MCP
	if search.K <= 0 {
		search.K = b.K
	}
	var call ToolCall
	call.Function.Name = b.tools[0].Function.Name
	call.Function.Arguments = search.arguments(query)
	stats.ToolCalls++
	emit(Event{Type: "tool_call", Name: call.Function.Name, Args: call.Function.Arguments})
	excerpts, data := b.callTool(ctx, call)
	emit(Event{Type: "tool_result", Name: call.Function.Name, Text: excerpts, Data: data})

	// 3. resposta só com os trechos
	msgs := []Message{
		{Role: "system", Content: answerPrompt + "\n\n" + languageInstruction(question)},
		{Role: "user", Content: fmt.Sprintf(
			"%sQuestion: %s\n\nSearch query used: %s\n\nDocumentation excerpts, most relevant first:\n\n%s\nAnswer the question using only these excerpts. %s",
			earlierQuestions(history), question, query, excerpts, languageInstruction(question))},
	}
	stats.Rounds++
	start = time.Now()
	reply, err := b.stream(ctx, model, msgs, answerGen, func(token string) { emit(Event{Type: "token", Text: token}) })
	stats.LLMTime += time.Since(start)
	if err != nil && reply.Message.Content == "" {
		emit(Event{Type: "error", Text: err.Error()})
		return "", err
	}
	answer := reply.Message.Content
	if err != nil {
		// parcial: entrega o que saiu e avisa
		stats.Note = err.Error()
		answer = strings.TrimSpace(answer) + "\n\n*[" + err.Error() + "]*"
	}
	stats.PromptTokens += reply.PromptEvalCount
	stats.OutputTokens += reply.EvalCount
	emit(Event{Type: "done", Answer: answer, Stats: &stats})
	return answer, nil
}

// earlierQuestions lista as perguntas anteriores do usuário (até 5), ou "" se não houver.
func earlierQuestions(history []Message) string {
	var qs []string
	for _, m := range history[:len(history)-1] {
		if m.Role == "user" {
			qs = append(qs, m.Content)
		}
	}
	if len(qs) == 0 {
		return ""
	}
	if len(qs) > 5 {
		qs = qs[len(qs)-5:]
	}
	return "Earlier questions in this conversation, for context only: " + strings.Join(qs, " | ") + "\n\n"
}

// languageInstruction devolve a ordem de idioma da resposta. Modelos pequenos
// ignoram "responda no idioma da pergunta", então a instrução é explícita:
// português se a pergunta tiver marcas de português, inglês caso contrário.
func languageInstruction(question string) string {
	words := strings.Fields(strings.ToLower(question))
	for _, w := range words {
		w = strings.Trim(w, "?!.,;:")
		if portugueseWords[w] {
			return "IMPORTANT: write the whole answer in Brazilian Portuguese (português do Brasil)."
		}
	}
	if strings.ContainsAny(question, "ãõçáéíóúâêô") {
		return "IMPORTANT: write the whole answer in Brazilian Portuguese (português do Brasil)."
	}
	return "Write the answer in English."
}

// Só palavras que não existem em inglês; "do", "a", "um", "de" ficam de fora.
var portugueseWords = map[string]bool{
	"como": true, "qual": true, "quais": true, "eu": true, "uma": true, "pra": true, "para": true, "que": true,
	"meu": true, "minha": true, "não": true, "nao": true, "você": true, "voce": true, "é": true, "faço": true, "faco": true,
	"criar": true, "crio": true, "apagar": true, "apago": true, "deletar": true, "excluir": true, "listar": true, "listo": true,
	"buscar": true, "pegar": true, "editar": true, "atualizar": true, "fechar": true, "abrir": true, "cancelar": true,
	"repositório": true, "repositorio": true, "organização": true, "organizacao": true, "usuário": true, "usuario": true,
	"elas": true, "eles": true, "dela": true, "dele": true, "isso": true, "esse": true, "essa": true,
}

// cleanQuery tira aspas, pontuação e quebras da reescrita; fica só a primeira linha.
func cleanQuery(text string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(text), "\n")
	line = strings.Trim(line, "\"'`.“”")
	line = strings.TrimPrefix(strings.ToLower(line), "query:")
	return strings.TrimSpace(line)
}

func (b *Bridge) callTool(ctx context.Context, call ToolCall) (string, any) {
	res, err := b.session.CallTool(ctx, &mcp.CallToolParams{Name: call.Function.Name, Arguments: call.Function.Arguments})
	if err != nil {
		return "tool error: " + err.Error(), nil
	}
	var text strings.Builder
	for _, c := range res.Content {
		if t, ok := c.(*mcp.TextContent); ok {
			text.WriteString(t.Text)
		}
	}
	return text.String(), res.StructuredContent
}

// --- Ollama -----------------------------------------------------------------

type chatRequest struct {
	Model    string         `json:"model"`
	Messages []Message      `json:"messages"`
	Tools    []ollamaTool   `json:"tools,omitempty"`
	Stream   bool           `json:"stream"`
	Options  map[string]any `json:"options,omitempty"`
}

type chatChunk struct {
	Message         Message `json:"message"`
	Done            bool    `json:"done"`
	Error           string  `json:"error"`
	PromptEvalCount int     `json:"prompt_eval_count"`
	EvalCount       int     `json:"eval_count"`
}

// genOptions limita uma chamada ao modelo. Modelos pequenos entram em repetição
// infinita com facilidade; o teto de tokens e a penalidade de repetição são a
// primeira barreira, o detector em stream é a segunda, o timeout a última.
type genOptions struct {
	MaxTokens   int
	Temperature float64
	Timeout     time.Duration
}

var (
	rewriteGen = genOptions{MaxTokens: 32, Temperature: 0, Timeout: 90 * time.Second}
	answerGen  = genOptions{MaxTokens: 900, Temperature: 0.1, Timeout: 4 * time.Minute}
)

// ErrRepetition é devolvido junto com o texto parcial quando a geração entrou em loop.
var ErrRepetition = fmt.Errorf("o modelo entrou em repetição e a geração foi interrompida")

// stream chama /api/chat com stream=true, repassa cada pedaço de texto a onToken
// (se não for nil) e devolve a mensagem completa com as contagens de tokens.
// Se detectar repetição, cancela a geração e devolve o parcial com ErrRepetition.
func (b *Bridge) stream(ctx context.Context, model string, msgs []Message, gen genOptions, onToken func(string)) (chatChunk, error) {
	ctx, cancel := context.WithTimeout(ctx, gen.Timeout)
	defer cancel()
	body, _ := json.Marshal(chatRequest{
		Model: model, Messages: msgs, Stream: true,
		Options: map[string]any{
			"num_ctx":        b.NumCtx,
			"num_predict":    gen.MaxTokens,
			"temperature":    gen.Temperature,
			"repeat_penalty": 1.15,
			"repeat_last_n":  256,
		},
	})
	req, err := http.NewRequestWithContext(ctx, "POST", b.OllamaURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return chatChunk{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := b.client.Do(req)
	if err != nil {
		return chatChunk{}, fmt.Errorf("ollama em %s: %w", b.OllamaURL, err)
	}
	defer resp.Body.Close()

	var full chatChunk
	full.Message.Role = "assistant"
	var content strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	for scanner.Scan() {
		var chunk chatChunk
		if err := json.Unmarshal(scanner.Bytes(), &chunk); err != nil {
			return chatChunk{}, fmt.Errorf("resposta inválida do ollama: %s", scanner.Bytes())
		}
		if chunk.Error != "" {
			return chatChunk{}, fmt.Errorf("ollama: %s", chunk.Error)
		}
		if chunk.Message.Content != "" {
			content.WriteString(chunk.Message.Content)
			if onToken != nil {
				onToken(chunk.Message.Content)
			}
			if looping(content.String()) {
				cancel() // fecha a conexão; o Ollama para de gerar
				full.Message.Content = content.String()
				return full, ErrRepetition
			}
		}
		full.Message.ToolCalls = append(full.Message.ToolCalls, chunk.Message.ToolCalls...)
		if chunk.Done {
			full.Done = true
			full.PromptEvalCount = chunk.PromptEvalCount
			full.EvalCount = chunk.EvalCount
		}
	}
	if err := scanner.Err(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			full.Message.Content = content.String()
			return full, fmt.Errorf("o modelo passou de %s e foi interrompido", gen.Timeout)
		}
		return chatChunk{}, err
	}
	full.Message.Content = content.String()
	return full, nil
}

// looping detecta repetição degenerada: o trecho final do texto já apareceu
// pelo menos duas outras vezes nos últimos caracteres. Roda a cada pedaço
// recebido, mas só olha uma janela pequena, então é barato.
func looping(text string) bool {
	const window, tail = 1600, 60
	if len(text) < tail*3 {
		return false
	}
	if len(text) > window {
		text = text[len(text)-window:]
	}
	last := text[len(text)-tail:]
	return strings.Count(text, last) >= 3
}

// ModelInfo é um modelo instalado no Ollama e se ele aceita tools.
type ModelInfo struct {
	Name  string `json:"name"`
	Size  int64  `json:"size"`
	Tools bool   `json:"tools"`
}

func (b *Bridge) Models(ctx context.Context) ([]ModelInfo, error) {
	var tags struct {
		Models []struct {
			Name string `json:"name"`
			Size int64  `json:"size"`
		} `json:"models"`
	}
	if err := b.getJSON(ctx, "GET", "/api/tags", nil, &tags); err != nil {
		return nil, err
	}
	models := make([]ModelInfo, 0, len(tags.Models))
	for _, m := range tags.Models {
		var show struct {
			Capabilities []string `json:"capabilities"`
		}
		info := ModelInfo{Name: m.Name, Size: m.Size}
		if err := b.getJSON(ctx, "POST", "/api/show", map[string]any{"model": m.Name}, &show); err == nil {
			for _, c := range show.Capabilities {
				if c == "tools" {
					info.Tools = true
				}
			}
		}
		models = append(models, info)
	}
	return models, nil
}

func (b *Bridge) getJSON(ctx context.Context, method, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, b.OllamaURL+path, reader)
	if err != nil {
		return err
	}
	resp, err := b.client.Do(req)
	if err != nil {
		return fmt.Errorf("ollama em %s: %w", b.OllamaURL, err)
	}
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(out)
}
