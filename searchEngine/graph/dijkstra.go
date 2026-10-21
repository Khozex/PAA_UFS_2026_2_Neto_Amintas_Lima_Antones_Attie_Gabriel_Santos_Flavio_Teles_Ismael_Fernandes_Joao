package graph

import (
	"container/heap"
	"math"
)

type DijkstraResult struct {
	Dist         []float64 // +Inf ou maior que maxCost se inalcançável
	Prev         []int
	Settled      []int // vértices fechados, em ordem de distância não decrescente
	EdgesScanned int
}

type item struct {
	v int
	d float64
}

type pq []item

func (p pq) Len() int { return len(p) }
func (p pq) Less(i, j int) bool {
	if p[i].d != p[j].d {
		return p[i].d < p[j].d
	}
	return p[i].v < p[j].v
}
func (p pq) Swap(i, j int) { p[i], p[j] = p[j], p[i] }
func (p *pq) Push(x any)   { *p = append(*p, x.(item)) }
func (p *pq) Pop() any {
	old := *p
	it := old[len(old)-1]
	*p = old[:len(old)-1]
	return it
}

func Dijkstra(g *Graph, sources []int, maxCost float64, f Filter) *DijkstraResult {
	r := &DijkstraResult{Dist: make([]float64, g.V()), Prev: make([]int, g.V())}
	for i := range r.Dist {
		r.Dist[i], r.Prev[i] = math.Inf(1), -1
	}
	q := &pq{}
	for _, s := range sources {
		if s >= 0 && s < g.V() && r.Dist[s] != 0 {
			r.Dist[s] = 0
			heap.Push(q, item{s, 0})
		}
	}
	done := make([]bool, g.V())
	for q.Len() > 0 {
		it := heap.Pop(q).(item)
		if done[it.v] || it.d > r.Dist[it.v] {
			continue
		}
		done[it.v] = true
		r.Settled = append(r.Settled, it.v)
		for _, e := range g.Adj[it.v] {
			r.EdgesScanned++
			if !f.ok(e) || done[e.To] {
				continue
			}
			nd := it.d + e.Weight
			if maxCost >= 0 && nd > maxCost {
				continue
			}
			if nd < r.Dist[e.To] { // relaxamento
				r.Dist[e.To], r.Prev[e.To] = nd, it.v
				heap.Push(q, item{e.To, nd})
			}
		}
	}
	return r
}

func (r *DijkstraResult) Reached(v int) bool {
	return v >= 0 && v < len(r.Dist) && !math.IsInf(r.Dist[v], 1)
}

func (r *DijkstraResult) Path(v int) ([]int, bool) {
	if !r.Reached(v) {
		return nil, false
	}
	return backtrack(r.Prev, v), true
}
