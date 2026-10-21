package engine

import (
	"slices"
	"strings"
	"testing"

	"paa/searchEngine/graph"
	"paa/searchEngine/types"
)

func loadWithGraph(t *testing.T, limit int) *Engine {
	t.Helper()
	opts := DefaultOptions()
	opts.Config = "indexed"
	opts.K = 5
	opts.Limit = limit
	opts.Graph = true
	e, err := Load(opts)
	if err != nil {
		t.Skipf("corpus não disponível (rode go run ./ingestDataPipeline): %v", err)
	}
	return e
}

func hitIDs(hits []types.Hit) []string {
	ids := make([]string, len(hits))
	for pos, hit := range hits {
		ids[pos] = hit.Doc.ID
	}
	return ids
}

func planFor(plans []graph.CallPlan, id string) (graph.CallPlan, bool) {
	for _, plan := range plans {
		if plan.Steps[len(plan.Steps)-1].ID == id {
			return plan, true
		}
	}
	return graph.CallPlan{}, false
}

func stepIDs(docs []*types.Document) []string {
	ids := make([]string, len(docs))
	for pos, doc := range docs {
		ids[pos] = doc.ID
	}
	return ids
}

func plansOf(t *testing.T, e *Engine, query string) ([]types.Hit, []graph.CallPlan) {
	t.Helper()
	hits, _ := e.Query(query)
	plans, _, err := e.Graph.Plans(hits)
	if err != nil {
		t.Fatal(err)
	}
	return hits, plans
}

func TestGraphNotBuiltByDefault(t *testing.T) {
	e := load(t, "indexed")
	if e.Graph != nil || e.GraphTime != 0 {
		t.Error("sem Options.Graph o grafo não deveria ser montado")
	}
}

func TestGraphOnRealCorpus(t *testing.T) {
	for _, limit := range []int{0, 612} {
		e := loadWithGraph(t, limit)
		g := e.Graph
		if e.GraphTime == 0 {
			t.Errorf("limit=%d: GraphTime não foi medido", limit)
		}

		operations, resources, dependencies := 0, 0, 0
		for _, vertex := range g.Vertices {
			if vertex.Kind == graph.Operation {
				operations++
			} else {
				resources++
			}
		}
		if operations != e.Corpus.N() {
			t.Errorf("limit=%d: %d operações no grafo para %d documentos", limit, operations, e.Corpus.N())
		}
		for _, doc := range e.Corpus.Docs {
			if _, ok := g.OperationVertex(doc.ID); !ok {
				t.Errorf("limit=%d: %s não virou vértice", limit, doc.ID)
			}
		}

		for from, edges := range g.Adj {
			for _, edge := range edges {
				if edge.Kind != graph.DependsOn {
					continue
				}
				dependencies++
				operation, list := g.Vertices[from].Doc, g.Vertices[edge.To].Doc
				if list.Method != "get" && list.Method != "GET" {
					t.Errorf("%s depende de %s, que não é GET", operation.ID, list.ID)
				}
				if !strings.HasPrefix(operation.Path, list.Path+"/") {
					t.Errorf("%s (%s) depende de %s (%s), que não é um path mais curto acima dele",
						operation.ID, operation.Path, list.ID, list.Path)
				}
			}
		}
		if want := 2*(resources-1) + 2*operations + dependencies; g.E() != want {
			t.Errorf("limit=%d: E=%d, esperava 2(R-1)+2·O+D=%d", limit, g.E(), want)
		}
		if _, err := graph.TopoOrder(g, graph.Only(graph.DependsOn)); err != nil {
			t.Errorf("limit=%d: %v", limit, err)
		}
	}
}

func TestEmptyQueryHasNoGraphWork(t *testing.T) {
	e := loadWithGraph(t, 0)
	hits, plans := plansOf(t, e, "the of")
	related, stats := e.Graph.Expand(hits, graph.ExpandOptions{Method: "bfs", MaxHops: 3, Limit: 10})
	if len(hits) != 0 || len(plans) != 0 || related != nil || stats.Visited != 0 {
		t.Errorf("consulta vazia: hits=%d planos=%d relacionadas=%d visitados=%d", len(hits), len(plans), len(related), stats.Visited)
	}
}

func TestPlanForRepositoryWebhook(t *testing.T) {
	e := loadWithGraph(t, 0)
	hits, plans := plansOf(t, e, "delete a repository webhook")
	if hits[0].Doc.ID != "repos/delete-webhook" {
		t.Fatalf("1º resultado mudou: %s", hits[0].Doc.ID)
	}
	plan, _ := planFor(plans, "repos/delete-webhook")
	if got, want := stepIDs(plan.Steps), []string{"repos/list-webhooks", "repos/delete-webhook"}; !slices.Equal(got, want) {
		t.Errorf("plano %v, esperava %v", got, want)
	}
}

func TestPlanCoversAnswerOutsideFirstPlace(t *testing.T) {
	e := loadWithGraph(t, 0)
	hits, plans := plansOf(t, e, "delete a release")
	if hits[0].Doc.ID == "repos/delete-release" {
		t.Skip("a resposta virou o 1º resultado; o caso deixou de testar o plano das 5")
	}
	if !slices.Contains(hitIDs(hits), "repos/delete-release") {
		t.Fatal("repos/delete-release saiu do top-5")
	}
	plan, ok := planFor(plans, "repos/delete-release")
	if !ok || !slices.Equal(stepIDs(plan.Prerequisites()), []string{"repos/list-releases"}) {
		t.Errorf("plano da resposta certa ausente ou errado: %v", stepIDs(plan.Steps))
	}
}

func TestExpandDoesNotRecoverIssueCreate(t *testing.T) {
	e := loadWithGraph(t, 0)
	hits, _ := e.Query("create an issue")
	if slices.Contains(hitIDs(hits), "issues/create") {
		t.Skip("issues/create entrou no top-5; o caso deixou de testar a expansão")
	}
	for _, method := range []string{"bfs", "dijkstra"} {
		related, _ := e.Graph.Expand(hits, graph.ExpandOptions{Method: method, MaxHops: 3, Budget: 3, Limit: 10})
		for _, item := range related {
			if item.Doc.ID == "issues/create" {
				t.Errorf("%s passou a trazer issues/create: atualize este teste e a tabela de testes", method)
			}
		}
	}
}

func TestPlanAddsPrerequisiteForUserKnownName(t *testing.T) {
	e := loadWithGraph(t, 0)
	_, plans := plansOf(t, e, "update a label")
	plan, ok := planFor(plans, "issues/update-label")
	if !ok || !slices.Equal(stepIDs(plan.Prerequisites()), []string{"issues/list-labels-for-repo"}) {
		t.Errorf("o plano de update-label mudou (era o falso pré-requisito list-labels-for-repo): atualize este teste e a tabela de testes")
	}
}

func TestPlanMissesIDInTheMiddleOfPath(t *testing.T) {
	e := loadWithGraph(t, 0)
	hits, plans := plansOf(t, e, "cancel a workflow run")
	if !slices.Contains(hitIDs(hits), "actions/cancel-workflow-run") {
		t.Fatal("actions/cancel-workflow-run saiu do top-5")
	}
	if plan, ok := planFor(plans, "actions/cancel-workflow-run"); ok && len(plan.Prerequisites()) > 0 {
		t.Errorf("cancel-workflow-run passou a ter plano %v: atualize este teste e a tabela de testes", stepIDs(plan.Steps))
	}
}
