package ranking

import "paa/searchEngine/types"

var Comparisons int

func Better(a, b types.Hit) bool {
	Comparisons++
	if a.Score != b.Score {
		return a.Score > b.Score
	}
	if a.Doc.Len != b.Doc.Len {
		return a.Doc.Len < b.Doc.Len
	}
	return a.Doc.ID < b.Doc.ID
}

func InsertTopK(top []types.Hit, hit types.Hit, k int) []types.Hit {
	if k <= 0 {
		return top
	}
	if len(top) == k {
		if !Better(hit, top[k-1]) {
			return top
		}
		top = top[:k-1]
	}
	top = append(top, hit)
	for pos := len(top) - 1; pos > 0 && Better(top[pos], top[pos-1]); pos-- {
		top[pos], top[pos-1] = top[pos-1], top[pos]
	}
	return top
}
