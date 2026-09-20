package utils

import "paa/searchEngine/types"

func MoreRelevant(a, b types.Hit) bool {
	if a.Score != b.Score {
		return a.Score > b.Score
	}
	if a.Doc.Len != b.Doc.Len {
		return a.Doc.Len < b.Doc.Len
	}
	return a.Doc.ID < b.Doc.ID
}
