package linearsearch

import (
	"time"

	"paa/searchEngine/ranking"
	"paa/searchEngine/selection"
	"paa/searchEngine/types"
	"paa/searchEngine/utils"
)

func Search(corpus *types.Corpus, query string, sel selection.Selector) ([]types.Hit, types.Stats) {
	start := time.Now()
	stats := types.Stats{N: corpus.N()}

	terms := utils.Normalize(query)
	if len(terms) == 0 {
		stats.EmptyQuery = true
		stats.QueryTime = time.Since(start)
		return nil, stats
	}

	for pos := range corpus.Docs {
		doc := &corpus.Docs[pos]
		score := ranking.Score(corpus, doc, terms)
		if score == 0 {
			continue
		}
		stats.Candidates++
		sel.Add(types.Hit{Doc: doc, Score: score})
	}

	hits := sel.Result()
	stats.Comparisons = sel.Comparisons()
	stats.QueryTime = time.Since(start)
	return hits, stats
}
