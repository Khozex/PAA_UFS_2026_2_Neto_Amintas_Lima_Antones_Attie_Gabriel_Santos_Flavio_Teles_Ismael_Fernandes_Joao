package selection

import (
	"paa/searchEngine/types"
	"paa/searchEngine/utils"
)

type KHeap struct {
	k           int
	hits        []types.Hit
	comparisons int
}

func NewKHeap(k int) *KHeap {
	return &KHeap{k: k}
}

func (s *KHeap) Add(hit types.Hit) {
	if s.k <= 0 {
		return
	}
	if len(s.hits) < s.k {
		s.hits = append(s.hits, hit)
		s.siftUp(len(s.hits) - 1)
		return
	}
	if !s.worse(s.hits[0], hit) {
		return
	}
	s.hits[0] = hit
	s.siftDown(0)
}

func (s *KHeap) Result() []types.Hit {
	out := make([]types.Hit, len(s.hits))
	for pos := len(out) - 1; pos >= 0; pos-- {
		out[pos] = s.popWorst()
	}
	return out
}

func (s *KHeap) Comparisons() int {
	return s.comparisons
}

func (s *KHeap) popWorst() types.Hit {
	worst := s.hits[0]
	last := len(s.hits) - 1
	s.hits[0] = s.hits[last]
	s.hits = s.hits[:last]
	if last > 0 {
		s.siftDown(0)
	}
	return worst
}

func (s *KHeap) siftUp(pos int) {
	for pos > 0 {
		parent := (pos - 1) / 2
		if !s.worse(s.hits[pos], s.hits[parent]) {
			return
		}
		s.hits[pos], s.hits[parent] = s.hits[parent], s.hits[pos]
		pos = parent
	}
}

func (s *KHeap) siftDown(pos int) {
	n := len(s.hits)
	for {
		child := 2*pos + 1
		if child >= n {
			return
		}
		if child+1 < n && s.worse(s.hits[child+1], s.hits[child]) {
			child++
		}
		if !s.worse(s.hits[child], s.hits[pos]) {
			return
		}
		s.hits[pos], s.hits[child] = s.hits[child], s.hits[pos]
		pos = child
	}
}

func (s *KHeap) worse(a, b types.Hit) bool {
	s.comparisons++
	return utils.MoreRelevant(b, a)
}
