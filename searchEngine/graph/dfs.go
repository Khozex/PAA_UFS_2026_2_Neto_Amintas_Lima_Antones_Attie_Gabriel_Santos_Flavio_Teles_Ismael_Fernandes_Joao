package graph

import "fmt"

type DFSResult struct {
	Pre          []int // ordem de descoberta
	Post         []int // ordem de finalização
	Cycle        []int
	Parent       []int
	EdgesScanned int
}

const (
	white = iota
	gray
	black
)

func DFS(g *Graph, seeds []int, f Filter) *DFSResult {
	r := &DFSResult{Parent: make([]int, g.V())}
	color := make([]uint8, g.V())
	next := make([]int, g.V())
	for i := range r.Parent {
		r.Parent[i] = -1
	}
	if seeds == nil {
		seeds = make([]int, g.V())
		for i := range seeds {
			seeds[i] = i
		}
	}
	for _, s := range seeds {
		if s < 0 || s >= g.V() || color[s] != white {
			continue
		}
		stack := []int{s}
		color[s] = gray
		r.Pre = append(r.Pre, s)
		for len(stack) > 0 {
			u := stack[len(stack)-1]
			if next[u] == len(g.Adj[u]) {
				color[u] = black
				r.Post = append(r.Post, u)
				stack = stack[:len(stack)-1]
				continue
			}
			e := g.Adj[u][next[u]]
			next[u]++
			r.EdgesScanned++
			if !f.ok(e) {
				continue
			}
			switch color[e.To] {
			case white:
				color[e.To], r.Parent[e.To] = gray, u
				r.Pre = append(r.Pre, e.To)
				stack = append(stack, e.To)
			case gray:
				if r.Cycle == nil {
					r.Cycle = cycleFrom(stack, e.To)
				}
			}
		}
	}
	return r
}

func cycleFrom(stack []int, v int) []int {
	i := len(stack) - 1
	for stack[i] != v {
		i--
	}
	c := append([]int(nil), stack[i:]...)
	return append(c, v)
}

func TopoOrder(g *Graph, f Filter) ([]int, error) {
	r := DFS(g, nil, f)
	if r.Cycle != nil {
		return nil, fmt.Errorf("ciclo de dependência: %v", r.Cycle)
	}
	order := make([]int, len(r.Post))
	for i, v := range r.Post {
		order[len(order)-1-i] = v
	}
	return order, nil
}

// Retorna as etapas para executar a operação target: pré-requisitos primeiro e
// target por último (ordem de finalização da DFS sobre DependsOn).
func Plan(g *Graph, target int) ([]int, error) {
	r := DFS(g, []int{target}, Only(DependsOn))
	if r.Cycle != nil {
		return nil, fmt.Errorf("ciclo de dependência: %v", r.Cycle)
	}
	return r.Post, nil
}
