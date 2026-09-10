package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"paa/searchEngine/corpus"
	indexedsearch "paa/searchEngine/indexedSearch"
	linearsearch "paa/searchEngine/linearSearch"
	"paa/searchEngine/types"
	"paa/searchEngine/utils"
)

var (
	defaultQuery = "create a repository in an organization"
	defaultK     = 5
)

func main() {
	corpusPath := flag.String("corpus", "data/processed/docs.jsonl", "")
	query := flag.String("query", defaultQuery, "pergunta")
	k := flag.Int("k", defaultK, "quantos resultados")
	limit := flag.Int("limit", 0, "usa só os N primeiros documentos")
	config := flag.String("config", "linear", "linear")
	flag.Parse()

	if *query == "" {
		fmt.Fprintln(os.Stderr, `uso: go run ./searchEngine -query "..." [-k 5] [-limit N] [-config linear]`)
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
		hits, stats = linearsearch.Search(c, *query, *k)
	case "indexed":
		indexedsearch.BuildIndex(c)
		hits, stats = indexedsearch.Search(c, *query, *k)
	default:
		fmt.Fprintf(os.Stderr, "config desconhecida: %s\n", *config)
		os.Exit(2)
	}

	fmt.Printf("query: %q   k=%d   config=%s\n\n", *query, *k, *config)
	if stats.EmptyQuery {
		fmt.Println("consulta vazia após normalização")
	}
	for pos, hit := range hits {
		fmt.Printf("%2d. %-42s %-6s %-45s %6.2f\n    %s\n",
			pos+1, hit.Doc.ID, hit.Doc.Method, hit.Doc.Path, hit.Score, hit.Doc.Summary)
	}
	fmt.Printf("\nN=%d  candidatos=%d  comparações=%d  carga=%s  consulta=%s\n",
		stats.N, stats.Candidates, stats.Comparisons,
		loadTime.Round(time.Millisecond), stats.QueryTime.Round(time.Microsecond))
}
