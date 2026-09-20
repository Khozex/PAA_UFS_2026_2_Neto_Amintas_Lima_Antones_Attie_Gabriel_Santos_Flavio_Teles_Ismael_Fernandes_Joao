package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"slices"
	"time"

	"paa/searchEngine/corpus"
	"paa/searchEngine/eval"
	indexedsearch "paa/searchEngine/indexedSearch"
	linearsearch "paa/searchEngine/linearSearch"
	"paa/searchEngine/selection"
	"paa/searchEngine/sorting"
	"paa/searchEngine/types"
	"paa/searchEngine/utils"
)

var (
	defaultQuery = "create a repository in an organization"
	defaultK     = 5
)

type preset struct {
	config, selectMode, orderStrategy string
}

var presets = map[int]preset{
	1: {"linear", "topk", "merge"},
	2: {"indexed", "topk", "merge"},
	3: {"linear", "sort", "merge"},
}

func main() {
	corpusPath := flag.String("corpus", "data/processed/docs.jsonl", "")
	query := flag.String("query", defaultQuery, "pergunta")
	k := flag.Int("k", defaultK, "quantos resultados")
	limit := flag.Int("limit", 0, "usa só os N primeiros documentos")
	config := flag.String("config", "linear", "linear|indexed")
	selectMode := flag.String("select", "topk", "topk|sort|heap")
	orderStrategy := flag.String("order_strategy", "merge", "quick|heap|merge|std, só com -select sort")
	order := flag.String("order", "desc", "asc ou desc")
	presetID := flag.Int("preset", 0, "1=linear+topk 2=indexed+topk 3=linear+sort/merge")
	context := flag.Bool("context", false, "imprime o texto completo dos resultados")
	evalMode := flag.Bool("eval", false, "roda todas as queries do gabarito e imprime Precision@k")
	queriesPath := flag.String("queries", "data/queries.json", "gabarito: query e operationIds relevantes")
	flag.Parse()

	if *query == "" && !*evalMode {
		fmt.Fprintln(os.Stderr, `uso: go run ./searchEngine -query "..." [-k 5] [-limit N] [-preset 1|2|3]`)
		os.Exit(2)
	}
	if *k < 0 {
		fmt.Fprintf(os.Stderr, "k deve ser >= 0, recebido: %d\n", *k)
		os.Exit(2)
	}
	if *presetID != 0 {
		p, ok := presets[*presetID]
		if !ok {
			fmt.Fprintf(os.Stderr, "preset desconhecido: %d\n", *presetID)
			os.Exit(2)
		}
		*config, *selectMode, *orderStrategy = p.config, p.selectMode, p.orderStrategy
	}
	if *order != "asc" && *order != "desc" {
		fmt.Fprintf(os.Stderr, "order deve ser asc ou desc, recebido: %s\n", *order)
		os.Exit(2)
	}
	algo, ok := sorting.Registry[*orderStrategy]
	if !ok {
		fmt.Fprintf(os.Stderr, "order_strategy desconhecida: %s\n", *orderStrategy)
		os.Exit(2)
	}
	if _, err := selection.New(*selectMode, *k, algo); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if _, ok := searchers[*config]; !ok {
		fmt.Fprintf(os.Stderr, "config desconhecida: %s\n", *config)
		os.Exit(2)
	}

	start := time.Now()
	raws, err := utils.ReadJSONL(utils.Resolve(*corpusPath))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if *limit > 0 && *limit < len(raws) {
		raws = raws[:*limit]
	}
	c := corpus.Load(raws)
	loadTime := time.Since(start)
	indexTime := buildIndex(c, *config)
	memKB := heapKB()

	run := runner{
		corpus:  c,
		search:  searchers[*config],
		label:   selectionLabel(*selectMode, *orderStrategy),
		newSel:  func() selection.Selector { sel, _ := selection.New(*selectMode, *k, algo); return sel },
		reverse: *order == "asc",
	}

	if *evalMode {
		queries, err := utils.ReadQueries(utils.Resolve(*queriesPath))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		runEval(run, queries, *k, *config, loadTime, indexTime, memKB)
		return
	}

	hits, stats := run.query(*query)
	fmt.Printf("query: %q   k=%d   config=%s   select=%s\n\n", *query, *k, *config, stats.Selection)
	if stats.EmptyQuery {
		fmt.Println("consulta vazia após normalização")
	}
	printHits(hits)
	if *context {
		printContext(hits)
	}
	printStats(stats, loadTime, indexTime, memKB)
}

func heapKB() uint64 {
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.HeapAlloc / 1024
}

type searchFunc func(*types.Corpus, string, selection.Selector) ([]types.Hit, types.Stats)

var searchers = map[string]searchFunc{
	"linear":  linearsearch.Search,
	"indexed": indexedsearch.Search,
}

type runner struct {
	corpus  *types.Corpus
	search  searchFunc
	label   string
	newSel  func() selection.Selector
	reverse bool
}

func (r runner) query(text string) ([]types.Hit, types.Stats) {
	hits, stats := r.search(r.corpus, text, r.newSel())
	stats.Selection = r.label
	if r.reverse {
		slices.Reverse(hits)
	}
	return hits, stats
}

func buildIndex(c *types.Corpus, config string) time.Duration {
	if config != "indexed" {
		return 0
	}
	start := time.Now()
	indexedsearch.BuildIndex(c)
	return time.Since(start)
}

func runEval(run runner, queries []types.Query, k int, config string, loadTime, indexTime time.Duration, memKB uint64) {
	fmt.Printf("eval: %d queries   k=%d   config=%s   select=%s\n\n", len(queries), k, config, run.label)
	var results []eval.Result
	for _, q := range queries {
		hits, stats := run.query(q.Query)
		result := eval.Evaluate(q, hits, stats, k)
		results = append(results, result)
		printEvalLine(result)
	}
	printSummary(eval.Summarize(results, k), k, loadTime, indexTime, memKB)
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

func selectionLabel(mode, strategy string) string {
	if mode == "sort" {
		return mode + "/" + strategy
	}
	return mode
}

func printContext(hits []types.Hit) {
	fmt.Println()
	for pos, hit := range hits {
		fmt.Printf("[%d] %s %s\n%s\nFonte: %s\n\n", pos+1, hit.Doc.Method, hit.Doc.Path, hit.Doc.Text, hit.Doc.DocsURL)
	}
}
