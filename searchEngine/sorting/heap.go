package sorting

import "paa/searchEngine/types"

func HeapSort(hits []types.Hit, less Comparator) int {
	comparisons := 0
	n := len(hits)

	var siftDown func(root, end int)
	siftDown = func(root, end int) {
		for {
			child := 2*root + 1
			if child > end {
				return
			}
			if child+1 <= end {
				comparisons++
				if less(hits[child], hits[child+1]) {
					child++
				}
			}
			comparisons++
			if less(hits[root], hits[child]) {
				hits[root], hits[child] = hits[child], hits[root]
				root = child
				continue
			}
			return
		}
	}

	for start := n/2 - 1; start >= 0; start-- {
		siftDown(start, n-1)
	}
	for end := n - 1; end > 0; end-- {
		hits[0], hits[end] = hits[end], hits[0]
		siftDown(0, end-1)
	}
	return comparisons
}
