package selection

import (
	"paa/searchEngine/sorting"
	"paa/searchEngine/types"
	"paa/searchEngine/utils"
)

type FullSort struct {
	k           int
	algo        sorting.Algorithm
	hits        []types.Hit
	comparisons int
}

func NewFullSort(k int, algo sorting.Algorithm) *FullSort {
	return &FullSort{k: k, algo: algo}
}

func (s *FullSort) Add(hit types.Hit) {
	s.hits = append(s.hits, hit)
}

func (s *FullSort) Result() []types.Hit {
	s.comparisons = s.algo(s.hits, utils.MoreRelevant)
	return s.hits[:s.cut()]
}

func (s *FullSort) Comparisons() int {
	return s.comparisons
}

func (s *FullSort) cut() int {
	if s.k < 0 {
		return 0
	}
	return min(s.k, len(s.hits))
}
