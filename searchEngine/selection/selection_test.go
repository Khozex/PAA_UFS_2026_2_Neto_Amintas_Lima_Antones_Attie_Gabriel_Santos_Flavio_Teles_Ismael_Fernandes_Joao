package selection

import (
	"fmt"
	"slices"
	"testing"

	"paa/searchEngine/sorting"
	"paa/searchEngine/types"
	"paa/searchEngine/utils"
)

func candidates(n int) []types.Hit {
	hits := make([]types.Hit, n)
	for i := range hits {
		hits[i] = types.Hit{
			Doc:   &types.Document{ID: fmt.Sprintf("doc-%03d", (i*37)%n), Len: i % 4},
			Score: float64(i % 6),
		}
	}
	return hits
}

func expectedTop(hits []types.Hit, k int) []types.Hit {
	want := slices.Clone(hits)
	slices.SortFunc(want, func(a, b types.Hit) int {
		if utils.MoreRelevant(a, b) {
			return -1
		}
		return 1
	})
	return want[:min(max(k, 0), len(want))]
}

func ids(hits []types.Hit) []string {
	out := make([]string, len(hits))
	for i, h := range hits {
		out[i] = h.Doc.ID
	}
	return out
}

func selectors(k int) map[string]Selector {
	out := map[string]Selector{}
	out["topk"] = NewTopK(k)
	out["heap"] = NewKHeap(k)
	for name, algo := range sorting.Registry {
		out["sort/"+name] = NewFullSort(k, algo)
	}
	return out
}

func TestAllSelectorsReturnTheSameTopK(t *testing.T) {
	hits := candidates(200)
	for _, k := range []int{0, 1, 5, 50, 200, 500} {
		want := ids(expectedTop(hits, k))
		for name, sel := range selectors(k) {
			for _, h := range hits {
				sel.Add(h)
			}
			got := ids(sel.Result())
			if !slices.Equal(got, want) {
				t.Fatalf("k=%d %s: got %v, want %v", k, name, got, want)
			}
		}
	}
}

func TestSelectorsWithNoCandidatesReturnEmpty(t *testing.T) {
	for name, sel := range selectors(5) {
		if got := sel.Result(); len(got) != 0 {
			t.Fatalf("%s: got %d hits, want 0", name, len(got))
		}
	}
}

func TestKZeroMakesNoComparisonsInTopKAndHeap(t *testing.T) {
	for _, name := range []string{"topk", "heap"} {
		sel := selectors(0)[name]
		for _, h := range candidates(50) {
			sel.Add(h)
		}
		if sel.Comparisons() != 0 {
			t.Fatalf("%s: %d comparisons with k=0", name, sel.Comparisons())
		}
	}
}

func TestNewRejectsUnknownMode(t *testing.T) {
	if _, err := New("bogus", 5, sorting.MergeSort); err == nil {
		t.Fatal("expected an error for unknown mode")
	}
}
