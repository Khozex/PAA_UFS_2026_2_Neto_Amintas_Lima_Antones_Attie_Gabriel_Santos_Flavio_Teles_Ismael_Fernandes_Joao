package stats

import (
	"math"
	"slices"
	"strings"

	"paa/ingestDataPipeline/types"
)

func Words(docs []types.Document) (int, types.WordStats) {
	counts := wordCounts(docs)
	if len(counts) == 0 {
		return 0, types.WordStats{}
	}
	total := sum(counts)
	return total, types.WordStats{
		Min:    slices.Min(counts),
		Median: median(counts),
		Mean:   round2(float64(total) / float64(len(counts))),
		Max:    slices.Max(counts),
	}
}

func wordCounts(docs []types.Document) []int {
	counts := make([]int, len(docs))
	for i, d := range docs {
		counts[i] = len(strings.Fields(d.Text))
	}
	return counts
}

func sum(counts []int) int {
	total := 0
	for _, c := range counts {
		total += c
	}
	return total
}

func median(counts []int) float64 {
	sorted := slices.Clone(counts)
	slices.Sort(sorted)
	n := len(sorted)
	if n%2 == 1 {
		return float64(sorted[n/2])
	}
	return float64(sorted[n/2-1]+sorted[n/2]) / 2
}

func round2(x float64) float64 {
	return math.Round(x*100) / 100
}
