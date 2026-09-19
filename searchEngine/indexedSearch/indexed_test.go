package indexedsearch

import (
	"fmt"
	"slices"
	"testing"

	"paa/searchEngine/corpus"
	linearsearch "paa/searchEngine/linearSearch"
	"paa/searchEngine/selection"
	"paa/searchEngine/sorting"
	"paa/searchEngine/types"
)

func syntheticCorpus() *types.Corpus {
	verbs := []string{"create", "list", "delete", "update", "get"}
	nouns := []string{"repository", "issue", "webhook", "organization", "secret", "runner"}
	var raws []types.RawDocument
	for i := 0; i < 90; i++ {
		verb, noun, other := verbs[i%len(verbs)], nouns[i%len(nouns)], nouns[(i*7)%len(nouns)]
		raws = append(raws, types.RawDocument{
			ID:          fmt.Sprintf("op-%03d", i),
			Summary:     verb + " " + noun,
			Path:        "/" + noun + "s",
			Description: fmt.Sprintf("%s a %s for an %s", verb, noun, other),
		})
	}
	return corpus.Load(raws)
}

func ids(hits []types.Hit) []string {
	out := make([]string, len(hits))
	for i, h := range hits {
		out[i] = h.Doc.ID
	}
	return out
}

func TestIndexedMatchesLinearForEveryQueryKAndSelector(t *testing.T) {
	c := syntheticCorpus()
	BuildIndex(c)
	queries := []string{"create a repository in an organization", "delete webhook", "runner", "", "the of", "xyzzy"}
	for _, query := range queries {
		for _, k := range []int{0, 1, 5, 40, 500} {
			want, wantStats := linearsearch.Search(c, query, selection.NewTopK(k))
			for _, mode := range []string{"topk", "heap", "sort"} {
				sel, _ := selection.New(mode, k, sorting.MergeSort)
				got, gotStats := Search(c, query, sel)
				if !slices.Equal(ids(got), ids(want)) {
					t.Fatalf("query=%q k=%d %s: got %v, want %v", query, k, mode, ids(got), ids(want))
				}
				if gotStats.Candidates != wantStats.Candidates {
					t.Fatalf("query=%q %s: candidates %d, linear %d", query, mode, gotStats.Candidates, wantStats.Candidates)
				}
			}
		}
	}
}

func TestEmptyQueryIsFlaggedAndReturnsNothing(t *testing.T) {
	c := syntheticCorpus()
	BuildIndex(c)
	hits, stats := Search(c, "the of a", selection.NewTopK(5))
	if !stats.EmptyQuery || len(hits) != 0 || stats.Comparisons != 0 {
		t.Fatalf("hits=%d empty=%v comparisons=%d", len(hits), stats.EmptyQuery, stats.Comparisons)
	}
}

func TestEveryPostingListIsIncreasingAndUnique(t *testing.T) {
	c := syntheticCorpus()
	BuildIndex(c)
	for term, postings := range c.InverseIndex {
		for i := 1; i < len(postings); i++ {
			if postings[i] <= postings[i-1] {
				t.Fatalf("term %q: postings not strictly increasing at %d", term, i)
			}
		}
	}
}
