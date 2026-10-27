package graph

import (
	"sort"
	"time"

	"paa/searchEngine/types"
)

type Related struct {
	Doc   *types.Document
	Cost  float64
	Hops  int
	Path  []string
	Order int
}

type ExpandOptions struct {
	Method  string
	MaxHops int
	Budget  float64
	Limit   int
	Filter  Filter
}

type ExpandStats struct {
	Visited      int
	EdgesScanned int
	Time         time.Duration
}

func (g *Graph) Expand(hits []types.Hit, o ExpandOptions) ([]Related, ExpandStats) {
	start := time.Now()
	var stats ExpandStats
	var seeds []int
	inHits := map[int]bool{}
	for _, h := range hits {
		if v, ok := g.OperationVertex(h.Doc.ID); ok && !inHits[v] {
			inHits[v] = true
			seeds = append(seeds, v)
		}
	}
	if len(seeds) == 0 {
		stats.Time = time.Since(start)
		return nil, stats
	}
	f := o.Filter
	if f == nil {
		f = Structural
	}

	var out []Related
	add := func(v int, cost float64, path []int) {
		if g.Vertices[v].Kind != Operation || inHits[v] {
			return
		}
		labels := make([]string, len(path))
		for i, p := range path {
			labels[i] = g.Vertices[p].Label
		}
		out = append(out, Related{Doc: g.Vertices[v].Doc, Cost: cost, Hops: len(path) - 1, Path: labels})
	}

	if o.Method == "dijkstra" {
		r := Dijkstra(g, seeds, o.Budget, f)
		stats.Visited, stats.EdgesScanned = len(r.Settled), r.EdgesScanned
		for _, v := range r.Settled {
			p, _ := r.Path(v)
			add(v, r.Dist[v], p)
		}
	} else {
		r := BFS(g, seeds, o.MaxHops, f)
		stats.Visited, stats.EdgesScanned = len(r.Order), r.EdgesScanned
		for _, vis := range r.Order {
			p, _ := r.Path(vis.Vertex)
			add(vis.Vertex, float64(vis.Depth), p)
		}
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Cost != out[j].Cost {
			return out[i].Cost < out[j].Cost
		}
		return out[i].Doc.ID < out[j].Doc.ID
	})
	if o.Limit > 0 && len(out) > o.Limit {
		out = out[:o.Limit]
	}
	stats.Time = time.Since(start)
	return out, stats
}
