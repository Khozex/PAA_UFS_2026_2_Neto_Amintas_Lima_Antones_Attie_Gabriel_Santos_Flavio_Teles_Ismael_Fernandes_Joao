// searchEngine/sorting/merge.go
package sorting

import "paa/searchEngine/types"

func MergeSort(hits []types.Hit, less Comparator) int {
	comparisons := 0
	buf := make([]types.Hit, len(hits))

	var ms func(lo, hi int)
	ms = func(lo, hi int) {
		if hi-lo <= 1 {
			return
		}
		mid := (lo + hi) / 2
		ms(lo, mid)
		ms(mid, hi)

		i, j, k := lo, mid, lo
		for i < mid && j < hi {
			comparisons++
			if less(hits[i], hits[j]) {
				buf[k] = hits[i]
				i++
			} else {
				buf[k] = hits[j]
				j++
			}
			k++
		}
		for i < mid {
			buf[k] = hits[i]
			i++
			k++
		}
		for j < hi {
			buf[k] = hits[j]
			j++
			k++
		}
		copy(hits[lo:hi], buf[lo:hi])
	}
	ms(0, len(hits))
	return comparisons
}