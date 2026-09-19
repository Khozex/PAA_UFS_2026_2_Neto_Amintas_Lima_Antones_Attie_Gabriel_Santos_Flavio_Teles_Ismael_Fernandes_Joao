package selection

import (
	"fmt"

	"paa/searchEngine/sorting"
	"paa/searchEngine/types"
)

type Selector interface {
	Add(types.Hit)
	Result() []types.Hit
	Comparisons() int
}

func New(mode string, k int, algo sorting.Algorithm) (Selector, error) {
	switch mode {
	case "topk":
		return NewTopK(k), nil
	case "heap":
		return NewKHeap(k), nil
	case "sort":
		return NewFullSort(k, algo), nil
	}
	return nil, fmt.Errorf("select desconhecido: %s", mode)
}
