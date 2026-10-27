// Servidor MCP (Model Context Protocol) sobre stdin/stdout que expõe o
// searchEngine como uma tool. Qualquer cliente MCP (ui/chat, ollmcp, MCP
// Inspector, Claude Desktop...) pode ligar um modelo a ele.
//
//	go run ./ui/mcp [-preset 2] [-max_chars 0]
//
// Tudo que não é protocolo vai para stderr; stdout é reservado ao MCP.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"paa/searchEngine/engine"
	"paa/searchEngine/graph"
	"paa/searchEngine/types"
)

const (
	toolName        = "search_github_api"
	defaultK        = 3
	maxK            = 10
	maxRelated      = 10
	defaultHops     = 3
	maxHops         = 5
	defaultMaxChars = 0 // 0 = texto completo de cada operação (como -context no CLI)
	description     = "Search the GitHub REST API documentation. Given a natural-language " +
		"question about what an endpoint does (e.g. \"create a repository in an " +
		"organization\", \"cancel a workflow run\"), returns the most relevant " +
		"operations with method, path, description and documentation URL. " +
		"Use it before answering any question about GitHub API endpoints."
)

type Input struct {
	Query         string `json:"query" jsonschema:"question in natural language about a GitHub REST API operation"`
	K             int    `json:"k,omitempty" jsonschema:"how many operations to return, 1 to 10, default 3"`
	Config        string `json:"config,omitempty" jsonschema:"search configuration: linear (scan every document) or indexed (inverted index); default is the server's"`
	Select        string `json:"select,omitempty" jsonschema:"how the top k are selected: topk, heap or sort; default is the server's"`
	OrderStrategy string `json:"order_strategy,omitempty" jsonschema:"sorting algorithm when select=sort: quick, heap, merge or std"`
	Plan          bool   `json:"plan,omitempty" jsonschema:"also return, for each result that needs an id produced by another call, the call order with prerequisites first"`
	Related       int    `json:"related,omitempty" jsonschema:"also return up to N operations on the same resources as the results (graph neighbors), 0 to 10; default 0"`
	Hops          int    `json:"hops,omitempty" jsonschema:"how many graph edges the related search may walk from the results, 1 to 5; default 3"`
}

type Result struct {
	Rank    int     `json:"rank"`
	ID      string  `json:"id"`
	Method  string  `json:"method"`
	Path    string  `json:"path"`
	Summary string  `json:"summary"`
	Text    string  `json:"text"`
	DocsURL string  `json:"docs_url"`
	Score   float64 `json:"score"`
}

// SearchStats é o custo da consulta, o mesmo que a última linha do CLI.
type SearchStats struct {
	N           int    `json:"n"`
	Candidates  int    `json:"candidates"`
	Comparisons int    `json:"comparisons"`
	QueryUS     int64  `json:"query_us"`
	IndexUS     int64  `json:"index_us"`
	Config      string `json:"config"`
	Selection   string `json:"selection"`
	K           int    `json:"k"`
}

// RelatedOp é uma operação vizinha dos resultados no grafo; não vem do ranking.
type RelatedOp struct {
	ID      string `json:"id"`
	Method  string `json:"method"`
	Path    string `json:"path"`
	Summary string `json:"summary"`
	DocsURL string `json:"docs_url"`
	Hops    int    `json:"hops"`
}

// RelatedStats é o custo da expansão (BFS nas arestas estruturais do grafo).
type RelatedStats struct {
	Hops         int   `json:"hops"`
	Visited      int   `json:"visited"`
	EdgesScanned int   `json:"edges_scanned"`
	ExpandUS     int64 `json:"expand_us"`
	GraphUS      int64 `json:"graph_us"`
}

// PlanStep é uma chamada do plano; a última de cada plano é o próprio resultado.
type PlanStep struct {
	ID      string `json:"id"`
	Method  string `json:"method"`
	Path    string `json:"path"`
	Summary string `json:"summary"`
}

type CallPlan struct {
	Rank  int        `json:"rank"`
	Steps []PlanStep `json:"steps"`
}

// PlanStats é o custo dos planos (DFS nas arestas DependsOn do grafo).
type PlanStats struct {
	WithPrerequisites int   `json:"with_prerequisites"`
	Prerequisites     int   `json:"prerequisites"`
	EdgesScanned      int   `json:"edges_scanned"`
	PlanUS            int64 `json:"plan_us"`
	GraphUS           int64 `json:"graph_us"`
}

type Output struct {
	Query        string        `json:"query"`
	Results      []Result      `json:"results"`
	Stats        SearchStats   `json:"stats"`
	Related      []RelatedOp   `json:"related,omitempty"`
	RelatedStats *RelatedStats `json:"related_stats,omitempty"`
	Plans        []CallPlan    `json:"plans,omitempty"`
	PlanStats    *PlanStats    `json:"plan_stats,omitempty"`
}

type server struct {
	engine   *engine.Engine
	maxChars int
}

func (s *server) search(ctx context.Context, req *mcp.CallToolRequest, in Input) (*mcp.CallToolResult, Output, error) {
	k := in.K
	if k <= 0 {
		k = defaultK
	}
	if k > maxK {
		k = maxK
	}
	q := engine.QueryOptions{Config: in.Config, Select: in.Select, OrderStrategy: in.OrderStrategy, K: k}
	hits, stats, err := s.engine.Search(in.Query, q)
	if err != nil {
		return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}}}, Output{}, nil
	}
	config := s.engine.Options().Config
	if in.Config != "" {
		config = in.Config
	}
	log.Printf("query=%q k=%d config=%s seleção=%s candidatos=%d comparações=%d consulta=%s",
		in.Query, k, config, stats.Selection, stats.Candidates, stats.Comparisons, stats.QueryTime)

	out := Output{Query: in.Query, Results: toResults(hits, s.maxChars), Stats: SearchStats{
		N: stats.N, Candidates: stats.Candidates, Comparisons: stats.Comparisons,
		QueryUS: stats.QueryTime.Microseconds(), IndexUS: s.engine.IndexTime.Microseconds(),
		Config: config, Selection: stats.Selection, K: k,
	}}
	text := render(in.Query, stats, out.Results)
	if related := min(in.Related, maxRelated); related > 0 {
		hops := in.Hops
		if hops <= 0 {
			hops = defaultHops
		}
		hops = min(hops, maxHops)
		neighbors, expandStats := s.engine.Graph.Expand(hits, graph.ExpandOptions{Method: "bfs", MaxHops: hops, Limit: related})
		out.Related = toRelated(neighbors)
		out.RelatedStats = &RelatedStats{
			Hops: hops, Visited: expandStats.Visited, EdgesScanned: expandStats.EdgesScanned,
			ExpandUS: expandStats.Time.Microseconds(), GraphUS: s.engine.GraphTime.Microseconds(),
		}
		text += renderRelated(out.Related)
		log.Printf("relacionadas=%d hops=%d visitados=%d arestas=%d expansão=%s",
			len(out.Related), hops, expandStats.Visited, expandStats.EdgesScanned, expandStats.Time)
	}
	if in.Plan {
		plans, planStats, err := s.engine.Graph.Plans(hits)
		if err != nil {
			return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}}}, Output{}, nil
		}
		out.Plans = toPlans(plans)
		out.PlanStats = &PlanStats{
			WithPrerequisites: planStats.WithPrerequisites, Prerequisites: planStats.Prerequisites,
			EdgesScanned: planStats.EdgesScanned, PlanUS: planStats.Time.Microseconds(),
			GraphUS: s.engine.GraphTime.Microseconds(),
		}
		text += renderPlans(out.Plans)
		log.Printf("planos=%d pré-requisitos=%d arestas=%d plano=%s",
			planStats.WithPrerequisites, planStats.Prerequisites, planStats.EdgesScanned, planStats.Time)
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}, out, nil
}

func toRelated(neighbors []graph.Related) []RelatedOp {
	out := make([]RelatedOp, 0, len(neighbors))
	for _, item := range neighbors {
		out = append(out, RelatedOp{ID: item.Doc.ID, Method: item.Doc.Method, Path: item.Doc.Path, Summary: item.Doc.Summary, DocsURL: item.Doc.DocsURL, Hops: item.Hops})
	}
	return out
}

// renderRelated é o bloco curto que o modelo lê depois dos trechos: uma linha
// por operação, sem o texto completo, para custar poucos tokens.
func renderRelated(related []RelatedOp) string {
	if len(related) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("Related operations (same resources as the results above, not ranked by the search):\n\n")
	for _, op := range related {
		fmt.Fprintf(&b, "- %s %s: %s. Docs: %s\n", op.Method, op.Path, op.Summary, op.DocsURL)
	}
	b.WriteString("\n")
	return b.String()
}

// toPlans guarda só os planos com pré-requisito; os outros seriam só o próprio resultado.
func toPlans(plans []graph.CallPlan) []CallPlan {
	var out []CallPlan
	for _, plan := range plans {
		if len(plan.Prerequisites()) == 0 {
			continue
		}
		steps := make([]PlanStep, 0, len(plan.Steps))
		for _, doc := range plan.Steps {
			steps = append(steps, PlanStep{ID: doc.ID, Method: doc.Method, Path: doc.Path, Summary: doc.Summary})
		}
		out = append(out, CallPlan{Rank: plan.Rank, Steps: steps})
	}
	return out
}

// renderPlans é o bloco que o modelo lê depois dos trechos: uma linha por chamada.
func renderPlans(plans []CallPlan) string {
	if len(plans) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("Call order (prerequisites first; earlier calls return the ids used by later ones):\n\n")
	for _, plan := range plans {
		last := plan.Steps[len(plan.Steps)-1]
		fmt.Fprintf(&b, "For [%d] %s %s:\n", plan.Rank, last.Method, last.Path)
		for step, call := range plan.Steps {
			fmt.Fprintf(&b, "  %d. %s %s\n", step+1, call.Method, call.Path)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func toResults(hits []types.Hit, maxChars int) []Result {
	results := make([]Result, 0, len(hits))
	for pos, hit := range hits {
		results = append(results, Result{
			Rank: pos + 1, ID: hit.Doc.ID, Method: hit.Doc.Method, Path: hit.Doc.Path,
			Summary: hit.Doc.Summary, Text: truncate(hit.Doc.Text, maxChars), DocsURL: hit.Doc.DocsURL, Score: hit.Score,
		})
	}
	return results
}

func truncate(text string, max int) string {
	if max <= 0 || len(text) <= max {
		return text
	}
	cut := strings.LastIndexByte(text[:max], ' ')
	if cut <= 0 {
		cut = max
	}
	return text[:cut] + " […]"
}

// render é o texto que o modelo lê: um bloco por operação, com a fonte.
func render(query string, stats types.Stats, results []Result) string {
	if stats.EmptyQuery {
		return "The query has no searchable terms after normalization. Rephrase it with concrete words (resource name and action)."
	}
	if len(results) == 0 {
		return fmt.Sprintf("No GitHub API operation matches %q. Try other words for the resource or the action.", query)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d GitHub REST API operations for %q, most relevant first:\n\n", len(results), query)
	for _, r := range results {
		fmt.Fprintf(&b, "[%d] %s %s\n%s\nDocs: %s\n\n", r.Rank, r.Method, r.Path, r.Text, r.DocsURL)
	}
	return b.String()
}

func main() {
	log.SetFlags(0)
	log.SetOutput(os.Stderr)

	opts := engine.DefaultOptions()
	opts.K = defaultK
	opts.Graph = true
	flag.StringVar(&opts.CorpusPath, "corpus", opts.CorpusPath, "caminho do docs.jsonl")
	flag.StringVar(&opts.Config, "config", opts.Config, "linear|indexed")
	flag.StringVar(&opts.Select, "select", opts.Select, "topk|sort|heap")
	flag.StringVar(&opts.OrderStrategy, "order_strategy", opts.OrderStrategy, "quick|heap|merge|std, só com -select sort")
	flag.IntVar(&opts.Limit, "limit", 0, "usa só os N primeiros documentos")
	presetID := flag.Int("preset", 2, "1=linear+topk 2=indexed+topk 3=linear+sort/merge")
	maxChars := flag.Int("max_chars", defaultMaxChars, "corta o texto de cada operação em N caracteres; 0 = texto completo")
	flag.Parse()

	if err := opts.ApplyPreset(*presetID); err != nil {
		log.Fatal(err)
	}
	eng, err := engine.Load(opts)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("mcp: N=%d config=%s seleção=%s carga=%s índice=%s grafo=%s V=%d E=%d",
		eng.Corpus.N(), opts.Config, eng.Label, eng.LoadTime, eng.IndexTime, eng.GraphTime, eng.Graph.V(), eng.Graph.E())

	srv := mcp.NewServer(&mcp.Implementation{Name: "github-api-search", Version: "0.1.0"}, nil)
	mcp.AddTool(srv, &mcp.Tool{
		Name:        toolName,
		Description: description,
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true},
	}, (&server{engine: eng, maxChars: *maxChars}).search)

	if err := srv.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}
