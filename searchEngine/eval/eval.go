package eval

import (
	"slices"
	"time"

	"paa/searchEngine/types"
)

type Result struct {
	Query        string
	Stats        types.Stats
	Rank         int
	PrecisionAtK float64
}

type Summary struct {
	Queries          int
	HitsAtK          int
	Empty            int
	MeanPrecisionAtK float64
	MeanRank         float64
	MeanQueryTime    time.Duration
}

func IDs(hits []types.Hit) []string {
	ids := make([]string, len(hits))
	for i, hit := range hits {
		ids[i] = hit.Doc.ID
	}
	return ids
}

func Rank(ids []string, relevant []string) int {
	for pos, id := range ids {
		if slices.Contains(relevant, id) {
			return pos + 1
		}
	}
	return 0
}

func PrecisionAtK(ids []string, relevant []string, k int) float64 {
	if k <= 0 {
		return 0
	}
	found := 0
	for _, id := range ids[:min(k, len(ids))] {
		if slices.Contains(relevant, id) {
			found++
		}
	}
	return float64(found) / float64(k)
}

func Evaluate(query types.Query, hits []types.Hit, stats types.Stats, k int) Result {
	ids := IDs(hits)
	return Result{
		Query:        query.Query,
		Stats:        stats,
		Rank:         Rank(ids, query.Relevant),
		PrecisionAtK: PrecisionAtK(ids, query.Relevant, k),
	}
}

func Summarize(results []Result, k int) Summary {
	s := Summary{Queries: len(results)}
	if len(results) == 0 {
		return s
	}
	var precision, rank float64
	var queryTime time.Duration
	ranked := 0
	for _, r := range results {
		precision += r.PrecisionAtK
		queryTime += r.Stats.QueryTime
		if r.Stats.Candidates == 0 {
			s.Empty++
		}
		if r.Rank > 0 {
			rank += float64(r.Rank)
			ranked++
		}
		if r.Rank > 0 && r.Rank <= k {
			s.HitsAtK++
		}
	}
	s.MeanPrecisionAtK = precision / float64(len(results))
	s.MeanQueryTime = queryTime / time.Duration(len(results))
	if ranked > 0 {
		s.MeanRank = rank / float64(ranked)
	}
	return s
}
