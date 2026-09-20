package sorting

import "paa/searchEngine/types"

func QuickSort(hits []types.Hit, less Comparator) int {
	comparisons := 0
	medianToEnd := func(lo, hi int) {
		mid := lo + (hi-lo)/2
		comparisons += 3
		if less(hits[mid], hits[lo]) {
			hits[mid], hits[lo] = hits[lo], hits[mid]
		}
		if less(hits[hi], hits[lo]) {
			hits[hi], hits[lo] = hits[lo], hits[hi]
		}
		if less(hits[hi], hits[mid]) {
			hits[hi], hits[mid] = hits[mid], hits[hi]
		}
		hits[mid], hits[hi] = hits[hi], hits[mid]
	}
	partition := func(lo, hi int) int {
		medianToEnd(lo, hi)
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
		if hi-lo < 2 {
			if hi-lo == 1 {
				comparisons++
				if less(hits[hi], hits[lo]) {
					hits[lo], hits[hi] = hits[hi], hits[lo]
				}
			}
			return
		}
		p := partition(lo, hi)
		qs(lo, p-1)
		qs(p+1, hi)
	}
	qs(0, len(hits)-1)
	return comparisons
}
