package sorting

import "paa/searchEngine/types"

type Comparator func(a, b types.Hit) bool

type Algorithm func(hits []types.Hit, less Comparator) int

var Registry = map[string]Algorithm{
	"quick": QuickSort,
	"heap":  HeapSort,
	"merge": MergeSort,
	"std":   StdSort,
}
