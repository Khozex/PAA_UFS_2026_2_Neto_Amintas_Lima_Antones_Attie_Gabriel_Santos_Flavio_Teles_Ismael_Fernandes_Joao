package selection

import (
	"paa/searchEngine/types"
	"paa/searchEngine/utils"
)

type TopK struct {
	k           int
	hits        []types.Hit
	comparisons int
}

func NewTopK(k int) *TopK {
	return &TopK{k: k}
}

func (s *TopK) Add(hit types.Hit) {
	if s.k <= 0 {
		return
	}
	if len(s.hits) == s.k {
		if !s.better(hit, s.hits[s.k-1]) {
			return
		}
		s.hits = s.hits[:s.k-1]
	}
	s.hits = append(s.hits, hit)
	for pos := len(s.hits) - 1; pos > 0 && s.better(s.hits[pos], s.hits[pos-1]); pos-- {
		s.hits[pos], s.hits[pos-1] = s.hits[pos-1], s.hits[pos]
	}
}

func (s *TopK) Result() []types.Hit {
	return s.hits
}

func (s *TopK) Comparisons() int {
	return s.comparisons
}

func (s *TopK) better(a, b types.Hit) bool {
	s.comparisons++
	return utils.MoreRelevant(a, b)
}
