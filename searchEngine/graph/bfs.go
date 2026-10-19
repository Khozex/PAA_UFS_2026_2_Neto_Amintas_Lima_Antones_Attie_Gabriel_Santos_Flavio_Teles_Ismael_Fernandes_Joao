package graph

// Visit é um vértice alcançado pela BFS.
type Visit struct {
	Vertex int
	Depth  int // número de arestas desde a semente mais próxima
	Parent int // -1 para sementes
}

type BFSResult struct {
	Order        []Visit // ordem de descoberta
	EdgesScanned int
	parent       []int
	depth        []int
}

func BFS(g *Graph, seeds []int, maxDepth int, f Filter) *BFSResult {
	r := &BFSResult{parent: make([]int, g.V()), depth: make([]int, g.V())}
	for i := range r.depth {
		r.depth[i], r.parent[i] = -1, -1
	}
	queue := make([]int, 0, len(seeds))
	for _, s := range seeds {
		if s < 0 || s >= g.V() || r.depth[s] == 0 {
			continue
		}
		r.depth[s] = 0
		queue = append(queue, s)
		r.Order = append(r.Order, Visit{Vertex: s, Depth: 0, Parent: -1})
	}
	for head := 0; head < len(queue); head++ {
		u := queue[head]
		if maxDepth >= 0 && r.depth[u] >= maxDepth {
			continue
		}
		for _, e := range g.Adj[u] {
			r.EdgesScanned++
			if !f.ok(e) || r.depth[e.To] != -1 {
				continue
			}
			r.depth[e.To], r.parent[e.To] = r.depth[u]+1, u
			queue = append(queue, e.To)
			r.Order = append(r.Order, Visit{Vertex: e.To, Depth: r.depth[e.To], Parent: u})
		}
	}
	return r
}

func (r *BFSResult) Reached(v int) bool { return v >= 0 && v < len(r.depth) && r.depth[v] >= 0 }

func (r *BFSResult) Path(v int) ([]int, bool) {
	if !r.Reached(v) {
		return nil, false
	}
	return backtrack(r.parent, v), true
}

func backtrack(parent []int, v int) []int {
	var path []int
	for ; v != -1; v = parent[v] {
		path = append(path, v)
	}
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}
	return path
}
