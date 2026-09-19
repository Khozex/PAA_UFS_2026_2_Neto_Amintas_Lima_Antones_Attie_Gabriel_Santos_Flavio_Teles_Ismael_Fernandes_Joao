package main

import (
	"flag"
	"fmt"
	"os"
	"slices"
	"time"

	"paa/searchEngine/corpus"
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
	orderStrategy := flag.String("order_strategy", "merge", "quick|heap|merge, só com -select sort")
	order := flag.String("order", "desc", "asc ou desc")
	presetID := flag.Int("preset", 0, "1=linear+topk 2=indexed+topk 3=linear+sort/merge")
	context := flag.Bool("context", false, "imprime o texto completo dos resultados")
	flag.Parse()

	if *query == "" {
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
	sel, err := selection.New(*selectMode, *k, algo)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
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

	var hits []types.Hit
	var stats types.Stats
	switch *config {
	case "linear":
		hits, stats = linearsearch.Search(c, *query, sel)
	case "indexed":
		indexedsearch.BuildIndex(c)
		hits, stats = indexedsearch.Search(c, *query, sel)
	default:
		fmt.Fprintf(os.Stderr, "config desconhecida: %s\n", *config)
		os.Exit(2)
	}
	stats.Selection = selectionLabel(*selectMode, *orderStrategy)

	if *order == "asc" {
		slices.Reverse(hits)
	}

	fmt.Printf("query: %q   k=%d   config=%s   select=%s\n\n", *query, *k, *config, stats.Selection)

	if stats.EmptyQuery {
		fmt.Println("consulta vazia após normalização")
	}

	for pos, hit := range hits {
		fmt.Printf("%2d. %-42s %-6s %-75s %6.2f\n    %s\n",
			pos+1, hit.Doc.ID, hit.Doc.Method, hit.Doc.Path, hit.Score, hit.Doc.Summary)
	}
	if *context {
		printContext(hits)
	}

	fmt.Printf("\nN=%d  candidatos=%d  comparações=%d  seleção=%s  carga=%s  consulta=%s\n",
		stats.N, stats.Candidates, stats.Comparisons, stats.Selection,
		loadTime.Round(time.Millisecond), stats.QueryTime.Round(time.Microsecond))
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
