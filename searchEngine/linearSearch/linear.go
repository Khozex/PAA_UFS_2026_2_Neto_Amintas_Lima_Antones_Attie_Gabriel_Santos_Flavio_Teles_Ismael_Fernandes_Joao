package linearsearch

import (
	"time"

	"paa/searchEngine/ranking"
	"paa/searchEngine/types"
	"paa/searchEngine/utils"
)

func Search(corpus *types.Corpus, query string, k int) ([]types.Hit, types.Stats) {
	start := time.Now()
	stats := types.Stats{N: corpus.N()}
	ranking.Comparisons = 0

	terms := utils.Normalize(query)
	if len(terms) == 0 {
		stats.EmptyQuery = true
		stats.QueryTime = time.Since(start)
		return nil, stats
	}

	var top []types.Hit
	for pos := range corpus.Docs {
		doc := &corpus.Docs[pos]
		score := ranking.Score(corpus, doc, terms)
		if score == 0 {
			continue
		}
		stats.Candidates++
		top = ranking.InsertTopK(top, types.Hit{Doc: doc, Score: score}, k)
	}

	stats.Comparisons = ranking.Comparisons
	stats.QueryTime = time.Since(start)
	return top, stats
}
