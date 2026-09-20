package sorting

import (
	"slices"

	"paa/searchEngine/types"
)

func StdSort(hits []types.Hit, less Comparator) int {
	comparisons := 0
	slices.SortFunc(hits, func(a, b types.Hit) int {
		if a.Doc == b.Doc {
			return 0
		}
		comparisons++
		if less(a, b) {
			return -1
		}
		return 1
	})
	return comparisons
}
