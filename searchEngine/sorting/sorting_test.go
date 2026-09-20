package sorting

import (
	"fmt"
	"math/rand"
	"slices"
	"testing"

	"paa/searchEngine/types"
	"paa/searchEngine/utils"
)

func hitsWithTies(n int) []types.Hit {
	hits := make([]types.Hit, n)
	for i := range hits {
		hits[i] = types.Hit{
			Doc:   &types.Document{ID: fmt.Sprintf("doc-%03d", i), Len: i % 7},
			Score: float64(i % 5),
		}
	}
	return hits
}

func expectedOrder(hits []types.Hit) []types.Hit {
	want := slices.Clone(hits)
	slices.SortFunc(want, func(a, b types.Hit) int {
		if utils.MoreRelevant(a, b) {
			return -1
		}
		if utils.MoreRelevant(b, a) {
			return 1
		}
		return 0
	})
	return want
}

func sameOrder(got, want []types.Hit) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i].Doc.ID != want[i].Doc.ID {
			return false
		}
	}
	return true
}

func TestAllAlgorithmsMatchReferenceOnShuffledInputWithTies(t *testing.T) {
	base := hitsWithTies(300)
	want := expectedOrder(base)
	rng := rand.New(rand.NewSource(1))
	for name, algo := range Registry {
		got := slices.Clone(base)
		rng.Shuffle(len(got), func(i, j int) { got[i], got[j] = got[j], got[i] })
		algo(got, utils.MoreRelevant)
		if !sameOrder(got, want) {
			t.Fatalf("%s: order differs from reference", name)
		}
	}
}

func TestAllAlgorithmsHandleSortedReversedEmptyAndSingle(t *testing.T) {
	base := hitsWithTies(64)
	sorted := expectedOrder(base)
	reversed := slices.Clone(sorted)
	slices.Reverse(reversed)
	inputs := map[string][]types.Hit{
		"sorted":   sorted,
		"reversed": reversed,
		"empty":    {},
		"single":   base[:1],
	}
	for name, algo := range Registry {
		for label, input := range inputs {
			got := slices.Clone(input)
			algo(got, utils.MoreRelevant)
			if !sameOrder(got, expectedOrder(input)) {
				t.Fatalf("%s on %s input: order differs from reference", name, label)
			}
		}
	}
}

func TestAlgorithmsReportComparisons(t *testing.T) {
	for name, algo := range Registry {
		got := slices.Clone(hitsWithTies(100))
		if algo(got, utils.MoreRelevant) <= 0 {
			t.Fatalf("%s: expected a positive comparison count", name)
		}
	}
}

func TestQuickSortAvoidsQuadraticOnSortedAndTiedInput(t *testing.T) {
	n := 1000
	sorted := make([]types.Hit, n)
	tied := make([]types.Hit, n)
	for i := range sorted {
		sorted[i] = types.Hit{Doc: &types.Document{ID: fmt.Sprintf("d%04d", i)}, Score: float64(n - i)}
		tied[i] = types.Hit{Doc: &types.Document{ID: fmt.Sprintf("d%04d", i)}, Score: 1}
	}
	limit := n * n / 4
	for name, in := range map[string][]types.Hit{"ordenada": sorted, "empatada": tied} {
		if got := QuickSort(slices.Clone(in), utils.MoreRelevant); got >= limit {
			t.Errorf("quick em entrada %s: %d comparações, esperado bem abaixo de %d", name, got, limit)
		}
	}
}
