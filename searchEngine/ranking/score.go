package ranking

import (
	"math"

	"paa/searchEngine/types"
)

var fields = []struct {
	name   string
	weight float64
}{
	{"summary", 3},
	{"path", 2},
	{"description", 1},
	{"params", 1},
}

func Score(corpus *types.Corpus, doc *types.Document, query []string) float64 {
	score := 0.0
	for _, term := range query {
		idf := corpus.IDF[term]
		if idf == 0 {
			continue
		}
		for _, field := range fields {
			tf := doc.TF[field.name][term]
			if tf == 0 {
				continue
			}
			score += field.weight * (1 + math.Log(float64(tf))) * idf
		}
	}
	return score
}
