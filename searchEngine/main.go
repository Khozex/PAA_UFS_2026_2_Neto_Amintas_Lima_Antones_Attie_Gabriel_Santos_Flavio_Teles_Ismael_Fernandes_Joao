package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"slices"
	"time"

	"paa/searchEngine/engine"
	"paa/searchEngine/eval"
	"paa/searchEngine/graph"
	"paa/searchEngine/types"
	"paa/searchEngine/utils"
)

var defaultQuery = "create a repository in an organization"

func main() {
	opts := engine.DefaultOptions()
	flag.StringVar(&opts.CorpusPath, "corpus", opts.CorpusPath, "")
	query := flag.String("query", defaultQuery, "pergunta")
	flag.IntVar(&opts.K, "k", opts.K, "quantos resultados")
	flag.IntVar(&opts.Limit, "limit", 0, "usa só os N primeiros documentos")
	flag.StringVar(&opts.Config, "config", opts.Config, "linear|indexed")
	flag.StringVar(&opts.Select, "select", opts.Select, "topk|sort|heap")
	flag.StringVar(&opts.OrderStrategy, "order_strategy", opts.OrderStrategy, "quick|heap|merge|std, só com -select sort")
	order := flag.String("order", "desc", "asc ou desc")
	presetID := flag.Int("preset", 0, "1=linear+topk 2=indexed+topk 3=linear+sort/merge")
	context := flag.Bool("context", false, "imprime o texto completo dos resultados")
	evalMode := flag.Bool("eval", false, "roda todas as queries do gabarito e imprime Precision@k")
	queriesPath := flag.String("queries", "data/queries.json", "gabarito: query e operationIds relevantes")
	expand := flag.String("expand", "", "expande os resultados pelo grafo: bfs|dijkstra")
	hops := flag.Int("hops", 3, "profundidade máxima da expansão BFS")
	budget := flag.Float64("budget", 3, "custo máximo da expansão Dijkstra")
	plan := flag.Bool("plan", false, "ordem de chamadas (pré-requisitos primeiro) para cada resultado, via DFS")
	flag.Parse()

	if *expand != "" && *expand != "bfs" && *expand != "dijkstra" {
		fmt.Fprintln(os.Stderr, "expand deve ser bfs ou dijkstra")
		os.Exit(2)
	}
	opts.Graph = *expand != "" || *plan

	if *query == "" && !*evalMode {
		fmt.Fprintln(os.Stderr, `uso: go run ./searchEngine -query "..." [-k 5] [-limit N] [-preset 1|2|3]`)
		os.Exit(2)
	}
	if err := opts.ApplyPreset(*presetID); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if *order != "asc" && *order != "desc" {
		fmt.Fprintf(os.Stderr, "order deve ser asc ou desc, recebido: %s\n", *order)
		os.Exit(2)
	}
	if err := opts.Validate(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	eng, err := engine.Load(opts)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	memKB := heapKB()

	run := runner{engine: eng, reverse: *order == "asc"}

	if *evalMode {
		queries, err := utils.ReadQueries(utils.Resolve(*queriesPath))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		runEval(run, queries, opts.K, opts.Config, memKB)
		return
	}

	hits, stats := run.query(*query)
	fmt.Printf("query: %q   k=%d   config=%s   select=%s\n\n", *query, opts.K, opts.Config, stats.Selection)
	if stats.EmptyQuery {
		fmt.Println("consulta vazia após normalização")
	}
	printHits(hits)
	if *context {
		printContext(hits)
	}
	var related []graph.Related
	var expandStats graph.ExpandStats
	if *expand != "" {
		related, expandStats = eng.Graph.Expand(hits, graph.ExpandOptions{Method: *expand, MaxHops: *hops, Budget: *budget, Limit: 10})
		printRelated(related)
	}
	var planStats graph.PlanStats
	if *plan {
		var plans []graph.CallPlan
		plans, planStats, err = eng.Graph.Plans(hits)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		printPlans(plans)
	}
	printStats(stats, eng.LoadTime, eng.IndexTime, memKB)
	if eng.Graph != nil {
		printGraphStats(eng, *expand, len(related), expandStats, *plan, planStats)
	}
}

func heapKB() uint64 {
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.HeapAlloc / 1024
}

type runner struct {
	engine  *engine.Engine
	reverse bool
}

func (r runner) query(text string) ([]types.Hit, types.Stats) {
	hits, stats := r.engine.Query(text)
	if r.reverse {
		slices.Reverse(hits)
	}
	return hits, stats
}

func runEval(run runner, queries []types.Query, k int, config string, memKB uint64) {
	fmt.Printf("eval: %d queries   k=%d   config=%s   select=%s\n\n", len(queries), k, config, run.engine.Label)
	var results []eval.Result
	for _, q := range queries {
		hits, stats := run.query(q.Query)
		result := eval.Evaluate(q, hits, stats, k)
		results = append(results, result)
		printEvalLine(result)
	}
	printSummary(eval.Summarize(results, k), k, run.engine.LoadTime, run.engine.IndexTime, memKB)
}

func printEvalLine(r eval.Result) {
	fmt.Printf("query=%-45q candidatos=%4d  comparações=%6d  consulta=%-10s posição=%2d  p@k=%.2f\n",
		r.Query, r.Stats.Candidates, r.Stats.Comparisons, r.Stats.QueryTime.Round(time.Microsecond), r.Rank, r.PrecisionAtK)
}

func printSummary(s eval.Summary, k int, loadTime, indexTime time.Duration, memKB uint64) {
	fmt.Printf("\nqueries=%d  acertos@%d=%d  vazias=%d  p@k_médio=%.3f  posição_média=%.1f  carga=%s  índice=%s  mem=%dkB  consulta_média=%s\n",
		s.Queries, k, s.HitsAtK, s.Empty, s.MeanPrecisionAtK, s.MeanRank,
		loadTime.Round(time.Millisecond), indexTime.Round(time.Microsecond), memKB, s.MeanQueryTime.Round(time.Microsecond))
}

func printHits(hits []types.Hit) {
	for pos, hit := range hits {
		fmt.Printf("%2d. %-42s %-6s %-75s %6.2f\n    %s\n",
			pos+1, hit.Doc.ID, hit.Doc.Method, hit.Doc.Path, hit.Score, hit.Doc.Summary)
	}
}

func printStats(stats types.Stats, loadTime, indexTime time.Duration, memKB uint64) {
	fmt.Printf("\nN=%d  candidatos=%d  comparações=%d  seleção=%s  carga=%s  índice=%s  mem=%dkB  consulta=%s\n",
		stats.N, stats.Candidates, stats.Comparisons, stats.Selection,
		loadTime.Round(time.Millisecond), indexTime.Round(time.Microsecond), memKB, stats.QueryTime.Round(time.Microsecond))
}

func printContext(hits []types.Hit) {
	fmt.Println()
	for pos, hit := range hits {
		fmt.Printf("[%d] %s %s\n%s\nFonte: %s\n\n", pos+1, hit.Doc.Method, hit.Doc.Path, hit.Doc.Text, hit.Doc.DocsURL)
	}
}

func printRelated(rel []graph.Related) {
	fmt.Printf("\nrelacionadas pelo grafo (%d):\n", len(rel))
	for _, r := range rel {
		fmt.Printf("  custo=%-4.1f %-42s %-6s %s\n", r.Cost, r.Doc.ID, r.Doc.Method, r.Doc.Path)
	}
}

func printPlans(plans []graph.CallPlan) {
	fmt.Println("\nplano de chamadas (pré-requisitos primeiro):")
	for _, plan := range plans {
		if len(plan.Prerequisites()) == 0 {
			continue
		}
		fmt.Printf("  [%d] %s\n", plan.Rank, plan.Steps[len(plan.Steps)-1].ID)
		for step, doc := range plan.Steps {
			fmt.Printf("      %d. %-42s %-6s %s\n", step+1, doc.ID, doc.Method, doc.Path)
		}
	}
}

func printGraphStats(eng *engine.Engine, expandMethod string, related int, expandStats graph.ExpandStats, plan bool, planStats graph.PlanStats) {
	if expandMethod == "" {
		expandMethod = "-"
	}
	fmt.Printf("grafo: V=%d  E=%d  construção=%s  expansão=%s  visitados=%d  arestas=%d  relacionadas=%d  expansão_tempo=%s",
		eng.Graph.V(), eng.Graph.E(), eng.GraphTime.Round(time.Microsecond),
		expandMethod, expandStats.Visited, expandStats.EdgesScanned, related, expandStats.Time.Round(time.Microsecond))
	if plan {
		fmt.Printf("  planos=%d  pré_requisitos=%d  plano_arestas=%d  plano_tempo=%s",
			planStats.WithPrerequisites, planStats.Prerequisites, planStats.EdgesScanned, planStats.Time.Round(time.Microsecond))
	}
	fmt.Println()
}
