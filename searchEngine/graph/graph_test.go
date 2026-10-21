package graph

import (
	"math"
	"math/rand"
	"slices"
	"testing"

	"paa/searchEngine/corpus"
	"paa/searchEngine/ranking"
	"paa/searchEngine/types"
	"paa/searchEngine/utils"
)

func raw(id, method, path, summary string) types.RawDocument {
	return types.RawDocument{ID: id, Method: method, Path: path, Summary: summary, Description: summary}
}

func fixture() *types.Corpus {
	return corpus.Load([]types.RawDocument{
		raw("repos/list-releases", "get", "/repos/{owner}/{repo}/releases", "List releases"),
		raw("repos/create-release", "post", "/repos/{owner}/{repo}/releases", "Create a release"),
		raw("repos/get-release", "get", "/repos/{owner}/{repo}/releases/{release_id}", "Get a release"),
		raw("repos/delete-release", "delete", "/repos/{owner}/{repo}/releases/{release_id}", "Delete a release"),
		raw("repos/list-release-assets", "get", "/repos/{owner}/{repo}/releases/{release_id}/assets", "List release assets"),
		raw("repos/get", "get", "/repos/{owner}/{repo}", "Get a repository"),
		raw("issues/list", "get", "/repos/{owner}/{repo}/issues", "List repository issues"),
		raw("issues/get", "get", "/repos/{owner}/{repo}/issues/{issue_number}", "Get an issue"),
		raw("orgs/get", "get", "/orgs/{org}", "Get an organization"),
	})
}

func vertex(t *testing.T, g *Graph, id string) int {
	t.Helper()
	v, ok := g.OperationVertex(id)
	if !ok {
		t.Fatalf("operação %s não está no grafo", id)
	}
	return v
}

func TestBuildStructure(t *testing.T) {
	g := Build(fixture().GraphRoot)
	if g.V() == 0 || g.E() == 0 {
		t.Fatal("grafo vazio")
	}
	ops := 0
	for _, v := range g.Vertices {
		if v.Kind == Operation {
			ops++
		}
	}
	if ops != 9 {
		t.Fatalf("esperava 9 operações, obteve %d", ops)
	}

	del, lst := vertex(t, g, "repos/delete-release"), vertex(t, g, "repos/list-releases")
	found := false
	for _, e := range g.Adj[del] {
		if e.Kind == DependsOn && e.To == lst {
			found = true
		}
	}
	if !found {
		t.Error("faltou DependsOn delete-release → list-releases")
	}

	g2 := Build(fixture().GraphRoot)
	if g.V() != g2.V() || g.E() != g2.E() {
		t.Error("construção não determinística")
	}
	for v := range g.Adj {
		if !slices.Equal(g.Adj[v], g2.Adj[v]) {
			t.Fatalf("adjacência de %d difere entre construções", v)
		}
	}
}

func TestPlanOrdersPrerequisitesFirst(t *testing.T) {
	g := Build(fixture().GraphRoot)
	plan, err := Plan(g, vertex(t, g, "repos/delete-release"))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"repos/list-releases", "repos/delete-release"}
	if len(plan) != len(want) {
		t.Fatalf("plano %v", plan)
	}
	for i, v := range plan {
		if g.Vertices[v].Doc.ID != want[i] {
			t.Errorf("etapa %d = %s, esperava %s", i, g.Vertices[v].Doc.ID, want[i])
		}
	}

	p, _ := Plan(g, vertex(t, g, "orgs/get"))
	if len(p) != 1 {
		t.Errorf("plano de orgs/get = %v", p)
	}
}

func TestTopoOrderValid(t *testing.T) {
	g := Build(fixture().GraphRoot)
	order, err := TopoOrder(g, Only(DependsOn))
	if err != nil {
		t.Fatal(err)
	}
	pos := map[int]int{}
	for i, v := range order {
		pos[v] = i
	}
	if len(order) != g.V() {
		t.Fatalf("ordem cobre %d de %d vértices", len(order), g.V())
	}
	for u := range g.Adj {
		for _, e := range g.Adj[u] {
			if e.Kind == DependsOn && pos[u] >= pos[e.To] {
				t.Errorf("aresta %d→%d viola a ordem", u, e.To)
			}
		}
	}
}

func TestDFSDetectsCycle(t *testing.T) {
	g := New()
	a, b, c := g.AddVertex(Operation, "a", nil), g.AddVertex(Operation, "b", nil), g.AddVertex(Operation, "c", nil)
	g.AddEdge(a, b, 1, DependsOn)
	g.AddEdge(b, c, 1, DependsOn)
	g.AddEdge(c, a, 1, DependsOn)
	if r := DFS(g, nil, nil); r.Cycle == nil || r.Cycle[0] != r.Cycle[len(r.Cycle)-1] {
		t.Errorf("ciclo não detectado: %v", r.Cycle)
	}
	if _, err := TopoOrder(g, nil); err == nil {
		t.Error("TopoOrder deveria falhar com ciclo")
	}
	if _, err := Plan(g, a); err == nil {
		t.Error("Plan deveria falhar com ciclo")
	}
}

func TestEmptyAndDisconnected(t *testing.T) {
	empty := Build(nil)
	if empty.V() != 0 || len(BFS(empty, nil, -1, nil).Order) != 0 {
		t.Error("grafo vazio")
	}
	if len(Dijkstra(empty, []int{0}, -1, nil).Settled) != 0 {
		t.Error("semente inválida deveria ser ignorada")
	}
	if o, err := TopoOrder(empty, nil); err != nil || len(o) != 0 {
		t.Error("topo de grafo vazio")
	}
	g := New()
	a, b := g.AddVertex(Operation, "a", nil), g.AddVertex(Operation, "b", nil)
	if _, ok := BFS(g, []int{a}, -1, nil).Path(b); ok {
		t.Error("BFS: b deveria ser inalcançável")
	}
	d := Dijkstra(g, []int{a}, -1, nil)
	if _, ok := d.Path(b); ok || !math.IsInf(d.Dist[b], 1) {
		t.Error("Dijkstra: b deveria ser inalcançável")
	}
}

func TestAddEdgeRejectsInvalid(t *testing.T) {
	g := New()
	a, b := g.AddVertex(Operation, "a", nil), g.AddVertex(Operation, "b", nil)
	if g.AddEdge(a, b, -1, Contains) == nil || g.AddEdge(a, b, math.NaN(), Contains) == nil || g.AddEdge(a, 9, 1, Contains) == nil {
		t.Error("aresta inválida aceita")
	}
}

func TestDuplicateDocumentCollapses(t *testing.T) {
	c := corpus.Load([]types.RawDocument{
		raw("x", "get", "/a", "one"),
		raw("x2", "get", "/a", "one again"),
	})
	g := Build(c.GraphRoot)
	n := 0
	for _, v := range g.Vertices {
		if v.Kind == Operation {
			n++
		}
	}
	if n != 1 {
		t.Errorf("duplicata deveria colapsar em 1 operação, obteve %d", n)
	}
}

func randomGraph(r *rand.Rand, n, m int, unit bool) *Graph {
	g := New()
	for i := 0; i < n; i++ {
		g.AddVertex(Operation, "v", nil)
	}
	for i := 0; i < m; i++ {
		w := 1.0
		if !unit {
			w = float64(r.Intn(8)) / 2
		}
		g.AddEdge(r.Intn(n), r.Intn(n), w, Contains)
	}
	return g
}

func bellmanFord(g *Graph, src int) []float64 {
	d := make([]float64, g.V())
	for i := range d {
		d[i] = math.Inf(1)
	}
	d[src] = 0
	for it := 0; it < g.V(); it++ {
		for u := range g.Adj {
			for _, e := range g.Adj[u] {
				if d[u]+e.Weight < d[e.To] {
					d[e.To] = d[u] + e.Weight
				}
			}
		}
	}
	return d
}

func TestDijkstraMatchesBellmanFord(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	for trial := 0; trial < 300; trial++ {
		g := randomGraph(r, 2+r.Intn(30), r.Intn(120), false)
		src := r.Intn(g.V())
		got, want := Dijkstra(g, []int{src}, -1, nil), bellmanFord(g, src)
		for v := range want {
			if got.Dist[v] != want[v] {
				t.Fatalf("trial %d vértice %d: %v ≠ %v", trial, v, got.Dist[v], want[v])
			}
			if p, ok := got.Path(v); ok {
				sum := 0.0
				for i := 0; i+1 < len(p); i++ {
					best := math.Inf(1)
					for _, e := range g.Adj[p[i]] {
						if e.To == p[i+1] && e.Weight < best {
							best = e.Weight
						}
					}
					sum += best
				}
				if sum != want[v] || p[0] != src {
					t.Fatalf("trial %d: caminho inconsistente com a distância", trial)
				}
			}
		}
		if !slices.IsSorted(settledDists(got)) {
			t.Fatal("Settled fora de ordem de distância")
		}
	}
}

func settledDists(r *DijkstraResult) []float64 {
	var d []float64
	for _, v := range r.Settled {
		d = append(d, r.Dist[v])
	}
	return d
}

func TestBFSEqualsDijkstraOnUnitWeights(t *testing.T) {
	r := rand.New(rand.NewSource(2))
	for trial := 0; trial < 200; trial++ {
		g := randomGraph(r, 2+r.Intn(30), r.Intn(100), true)
		src := r.Intn(g.V())
		b, d := BFS(g, []int{src}, -1, nil), Dijkstra(g, []int{src}, -1, nil)
		for v := 0; v < g.V(); v++ {
			if b.Reached(v) != d.Reached(v) {
				t.Fatalf("alcançabilidade difere em %d", v)
			}
			if b.Reached(v) && float64(b.depth[v]) != d.Dist[v] {
				t.Fatalf("BFS %d ≠ Dijkstra %v", b.depth[v], d.Dist[v])
			}
		}
	}
}

func TestBFSDepthLimitAndDijkstraBudget(t *testing.T) {
	g := New()
	prev := g.AddVertex(Operation, "0", nil)
	for i := 1; i < 6; i++ {
		v := g.AddVertex(Operation, "n", nil)
		g.AddEdge(prev, v, 1, Contains)
		prev = v
	}
	if n := len(BFS(g, []int{0}, 2, nil).Order); n != 3 {
		t.Errorf("BFS depth 2: %d vértices, esperava 3", n)
	}
	if n := len(Dijkstra(g, []int{0}, 2.5, nil).Settled); n != 3 {
		t.Errorf("Dijkstra budget 2.5: %d vértices, esperava 3", n)
	}
}

func TestDijkstraBeatsBFSWhenWeightsDiffer(t *testing.T) {

	g := New()
	for i := 0; i < 4; i++ {
		g.AddVertex(Operation, "v", nil)
	}
	g.AddEdge(0, 3, 10, Contains)
	g.AddEdge(0, 1, 1, Contains)
	g.AddEdge(1, 2, 1, Contains)
	g.AddEdge(2, 3, 1, Contains)
	pb, _ := BFS(g, []int{0}, -1, nil).Path(3)
	pd, _ := Dijkstra(g, []int{0}, -1, nil).Path(3)
	if len(pb) != 2 || len(pd) != 4 {
		t.Errorf("BFS=%v Dijkstra=%v", pb, pd)
	}
}

func TestExpand(t *testing.T) {
	c := fixture()
	g := Build(c.GraphRoot)
	query := utils.Normalize("delete a release")
	var hits []types.Hit
	for i := range c.Docs {
		if s := ranking.Score(c, &c.Docs[i], query); s > 0 {
			hits = append(hits, types.Hit{Doc: &c.Docs[i], Score: s})
		}
	}
	slices.SortFunc(hits, func(a, b types.Hit) int {
		if utils.MoreRelevant(a, b) {
			return -1
		}
		return 1
	})
	hits = hits[:1]
	if hits[0].Doc.ID != "repos/delete-release" {
		t.Fatalf("semente inesperada %s", hits[0].Doc.ID)
	}
	for _, method := range []string{"bfs", "dijkstra"} {
		rel, stats := g.Expand(hits, ExpandOptions{Method: method, MaxHops: 2, Budget: 2})
		if len(rel) == 0 || rel[0].Doc.ID != "repos/get-release" {
			t.Fatalf("%s: vizinho mais próximo deveria ser get-release, obteve %v", method, rel)
		}
		if stats.Visited <= len(rel) || stats.EdgesScanned == 0 {
			t.Errorf("%s: contadores incoerentes: %+v para %d relacionadas", method, stats, len(rel))
		}
		for _, r := range rel {
			if r.Doc.ID == "repos/delete-release" {
				t.Errorf("%s: semente não deve voltar como relacionada", method)
			}
		}
	}
	if rel, stats := g.Expand(nil, ExpandOptions{Method: "bfs", MaxHops: 3}); rel != nil || stats.Visited != 0 {
		t.Error("sem hits não há expansão")
	}
	if got, _ := g.Expand(hits, ExpandOptions{Method: "dijkstra", Budget: -1, Limit: 2}); len(got) != 2 {
		t.Errorf("Limit=2 devolveu %d", len(got))
	}
}
