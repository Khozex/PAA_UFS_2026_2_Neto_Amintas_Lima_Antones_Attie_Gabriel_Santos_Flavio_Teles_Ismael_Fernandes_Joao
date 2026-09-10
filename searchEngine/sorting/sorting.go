// searchEngine/sorting/sorting.go
package sorting

import "paa/searchEngine/types"

type Order int

const (
	Desc Order = iota
	Asc
)

// Comparator retorna true se a deve vir antes de b.
type Comparator func(a, b types.Hit) bool

func LessFor(order Order) Comparator {
	if order == Asc {
		return func(a, b types.Hit) bool { return a.Score < b.Score }
	}
	return func(a, b types.Hit) bool { return a.Score > b.Score }
}

// Algorithm ordena hits in-place e retorna o número de comparações feitas.
type Algorithm func(hits []types.Hit, less Comparator) int

var Registry = map[string]Algorithm{
	"quick":     QuickSort,
	"heap":      HeapSort,
	"merge":     MergeSort,
}