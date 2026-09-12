package sorting

import "paa/searchEngine/types"

func QuickSort(hits []types.Hit, less Comparator) int {
	comparisons := 0
	var partition func(lo, hi int) int
	partition = func(lo, hi int) int {
		pivot := hits[hi]
		i := lo - 1
		for j := lo; j < hi; j++ {
			comparisons++
			if less(hits[j], pivot) {
				i++
				hits[i], hits[j] = hits[j], hits[i]
			}
		}
		hits[i+1], hits[hi] = hits[hi], hits[i+1]
		return i + 1
	}
	var qs func(lo, hi int)
	qs = func(lo, hi int) {
		if lo >= hi {
			return
		}
		p := partition(lo, hi)
		qs(lo, p-1)
		qs(p+1, hi)
	}
	qs(0, len(hits)-1)
	return comparisons
}
