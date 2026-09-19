package sorting

import (
	"paa/searchEngine/types"
	"paa/searchEngine/utils"
)

type Order int

const (
	Desc Order = iota
	Asc
)

// Comparator retorna true se a deve vir antes de b.
type Comparator func(a, b types.Hit) bool

func LessFor(order Order) Comparator {
	if order == Asc {
		return func(a, b types.Hit) bool { return utils.MoreRelevant(b, a) }
	}
	return utils.MoreRelevant
}

// Algorithm ordena hits in-place e retorna o número de comparações feitas.
type Algorithm func(hits []types.Hit, less Comparator) int

var Registry = map[string]Algorithm{
	"quick": QuickSort,
	"heap":  HeapSort,
	"merge": MergeSort,
}
