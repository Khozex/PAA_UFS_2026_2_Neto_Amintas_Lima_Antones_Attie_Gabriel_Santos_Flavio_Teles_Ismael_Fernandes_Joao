package indexedsearch

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

	for _, docID := range candidates(corpus, terms) {
		doc := &corpus.Docs[docID]
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

func candidates(corpus *types.Corpus, terms []string) []int {
	seen := make(map[int]bool)
	var ids []int
	for _, term := range terms {
		for _, docID := range corpus.InverseIndex[term] {
			if seen[docID] {
				continue
			}
			seen[docID] = true
			ids = append(ids, docID)
		}
	}
	return ids
}

func BuildIndex(corpus *types.Corpus) {
	corpus.InverseIndex = make(map[string][]int)

	for docID := range corpus.Docs {
		doc := &corpus.Docs[docID]
		seen := make(map[string]bool)

		for _, terms := range doc.TF {
			for term := range terms {
				if seen[term] {
					continue
				}

				seen[term] = true
				corpus.InverseIndex[term] = append(corpus.InverseIndex[term], docID)
			}
		}
	}
}
