package indexedsearch

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

	candidates := make(map[int]bool)

	// Retrieve candidates from the inverted index for each term in the query
	for _, term := range terms {
		postings, ok := corpus.InverseIndex[term]
		if !ok {
			continue
		}

		for _, docID := range postings {
			candidates[docID] = true
		}
	}

	var top []types.Hit
	for docID := range candidates {
		doc := &corpus.Docs[docID]
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

func BuildIndex(corpus *types.Corpus) {
	corpus.InverseIndex = make(map[string][]int)

	for docID := range corpus.Docs {
		doc := &corpus.Docs[docID]
		seen := make(map[string]bool) // To track seen terms for this document

		for _, terms := range doc.TF {
			for term := range terms {
				if seen[term] {
					continue
				}

				seen[term] = true // Marks the term as seen for this document
				corpus.InverseIndex[term] = append(corpus.InverseIndex[term], docID)
			}
		}
	}
}
