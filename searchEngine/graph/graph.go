package graph

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"paa/searchEngine/types"
)

type VertexKind uint8

const (
	Resource VertexKind = iota
	Operation
)

type EdgeKind uint8

const (
	Contains EdgeKind = iota
	Within
	Exposes
	ExposedBy
	DependsOn
)

func (k EdgeKind) String() string {
	return [...]string{"contains", "within", "exposes", "exposed_by", "depends_on"}[k]
}

const (
	WeightContains  = 1.0
	WeightExposes   = 0.5
	WeightDependsOn = 1.0
)

type Edge struct {
	To     int
	Weight float64
	Kind   EdgeKind
}

type Vertex struct {
	ID    int
	Kind  VertexKind
	Label string
	Doc   *types.Document
}

type Graph struct {
	Vertices []Vertex
	Adj      [][]Edge
	byDocID  map[string]int
}

func New() *Graph { return &Graph{byDocID: map[string]int{}} }

func (g *Graph) AddVertex(kind VertexKind, label string, doc *types.Document) int {
	id := len(g.Vertices)
	g.Vertices = append(g.Vertices, Vertex{ID: id, Kind: kind, Label: label, Doc: doc})
	g.Adj = append(g.Adj, nil)
	if doc != nil {
		if _, dup := g.byDocID[doc.ID]; !dup { // documento duplicado: vale o primeiro
			g.byDocID[doc.ID] = id
		}
	}
	return id
}

func (g *Graph) AddEdge(from, to int, w float64, kind EdgeKind) error {
	if from < 0 || from >= len(g.Vertices) || to < 0 || to >= len(g.Vertices) {
		return fmt.Errorf("aresta fora do grafo: %d→%d", from, to)
	}
	if math.IsNaN(w) || w < 0 {
		return fmt.Errorf("peso inválido %v na aresta %d→%d", w, from, to)
	}
	g.Adj[from] = append(g.Adj[from], Edge{To: to, Weight: w, Kind: kind})
	return nil
}

func (g *Graph) V() int { return len(g.Vertices) }

func (g *Graph) E() int {
	n := 0
	for _, a := range g.Adj {
		n += len(a)
	}
	return n
}

func (g *Graph) OperationVertex(docID string) (int, bool) {
	v, ok := g.byDocID[docID]
	return v, ok
}

func Build(root *types.DocumentNode) *Graph {
	g := New()
	if root == nil {
		return g
	}
	ids := map[*types.DocumentNode]int{}

	var walk func(n *types.DocumentNode)
	walk = func(n *types.DocumentNode) {
		label := n.Path
		if label == "" {
			label = "/"
		}
		rv := g.AddVertex(Resource, label, nil)
		ids[n] = rv
		if n.Parent != nil {
			pv := ids[n.Parent]
			_ = g.AddEdge(pv, rv, WeightContains, Contains)
			_ = g.AddEdge(rv, pv, WeightContains, Within)
		}
		for _, method := range sortedKeys(n.Operations) {
			doc := n.Operations[method]
			ov := g.AddVertex(Operation, method+" "+label, doc)
			_ = g.AddEdge(rv, ov, WeightExposes, Exposes)
			_ = g.AddEdge(ov, rv, WeightExposes, ExposedBy)
		}
		for _, s := range sortedKeys(n.Children) {
			walk(n.Children[s])
		}
	}
	walk(root)

	for node, resourceVertex := range ids {
		for _, method := range sortedKeys(node.Operations) {
			operationVertex := g.opVertexOf(node.Operations[method], resourceVertex)
			if operationVertex < 0 {
				continue
			}
			for ancestor := node; ancestor.Parent != nil; ancestor = ancestor.Parent {
				if !isGeneratedID(lastSegment(ancestor.Path)) {
					continue
				}
				list, ok := ancestor.Parent.Operations["GET"]
				if !ok {
					continue
				}
				if listVertex, ok := g.byDocID[list.ID]; ok {
					_ = g.AddEdge(operationVertex, listVertex, WeightDependsOn, DependsOn)
				}
			}
		}
	}

	for v := range g.Adj {
		sort.SliceStable(g.Adj[v], func(i, j int) bool {
			a, b := g.Adj[v][i], g.Adj[v][j]
			if a.Kind != b.Kind {
				return a.Kind < b.Kind
			}
			return a.To < b.To
		})
	}
	return g
}

func (g *Graph) opVertexOf(doc *types.Document, rv int) int {
	for _, e := range g.Adj[rv] {
		if e.Kind == Exposes && g.Vertices[e.To].Doc == doc {
			return e.To
		}
	}
	return -1
}

func lastSegment(path string) string {
	return path[strings.LastIndex(path, "/")+1:]
}

func isParam(seg string) bool { return strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}") }

func isGeneratedID(segment string) bool {
	return isParam(segment) && strings.HasSuffix(segment, "_id}")
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

type Filter func(Edge) bool

func Only(kinds ...EdgeKind) Filter {
	return func(e Edge) bool {
		for _, k := range kinds {
			if e.Kind == k {
				return true
			}
		}
		return false
	}
}

func Except(kinds ...EdgeKind) Filter {
	only := Only(kinds...)
	return func(e Edge) bool { return !only(e) }
}

func (f Filter) ok(e Edge) bool { return f == nil || f(e) }

var Structural = Except(DependsOn)
