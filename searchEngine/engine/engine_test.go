package engine

import (
	"testing"
)

func load(t *testing.T, config string) *Engine {
	t.Helper()
	opts := DefaultOptions()
	opts.Config = config
	opts.K = 5
	e, err := Load(opts)
	if err != nil {
		t.Skipf("corpus não disponível (rode go run ./ingestDataPipeline): %v", err)
	}
	return e
}

func TestLinearAndIndexedAgree(t *testing.T) {
	linear, indexed := load(t, "linear"), load(t, "indexed")
	if indexed.IndexTime == 0 {
		t.Fatal("indexed deveria construir o índice na carga")
	}
	for _, q := range []string{"create a repository in an organization", "cancel a workflow run", "list issues"} {
		a, _ := linear.Query(q)
		b, _ := indexed.Query(q)
		if len(a) != 5 || len(b) != 5 {
			t.Fatalf("%q: esperava 5 resultados, linear=%d indexed=%d", q, len(a), len(b))
		}
		for i := range a {
			if a[i].Doc.ID != b[i].Doc.ID {
				t.Errorf("%q posição %d: linear=%s indexed=%s", q, i, a[i].Doc.ID, b[i].Doc.ID)
			}
		}
	}
}

func TestQueryKOverridesK(t *testing.T) {
	e := load(t, "indexed")
	hits, stats := e.QueryK("cancel a workflow run", 2)
	if len(hits) != 2 {
		t.Fatalf("esperava 2 resultados, veio %d", len(hits))
	}
	if stats.Selection != "topk" {
		t.Errorf("selection=%q", stats.Selection)
	}
	if _, stats := e.QueryK("the of", 3); !stats.EmptyQuery {
		t.Error("consulta só de stopwords deveria ser vazia")
	}
}

func TestValidate(t *testing.T) {
	bad := []func(*Options){
		func(o *Options) { o.K = -1 },
		func(o *Options) { o.Config = "x" },
		func(o *Options) { o.Select = "x" },
		func(o *Options) { o.OrderStrategy = "x" },
	}
	for i, mutate := range bad {
		o := DefaultOptions()
		mutate(&o)
		if err := o.Validate(); err == nil {
			t.Errorf("caso %d: esperava erro", i)
		}
	}
	o := DefaultOptions()
	if err := o.ApplyPreset(9); err == nil {
		t.Error("preset 9 deveria falhar")
	}
	if err := o.ApplyPreset(3); err != nil || o.Select != "sort" {
		t.Errorf("preset 3: err=%v select=%s", err, o.Select)
	}
}

func TestSearchOverridesPerQuery(t *testing.T) {
	e := load(t, "linear")
	if e.Corpus.InverseIndex != nil {
		t.Fatal("linear não deveria construir o índice na carga")
	}
	hits, stats, err := e.Search("cancel a workflow run", QueryOptions{Config: "indexed", Select: "sort", OrderStrategy: "quick", K: 2})
	if err != nil {
		t.Fatal(err)
	}
	if e.Corpus.InverseIndex == nil || e.IndexTime == 0 {
		t.Error("Search com indexed deveria construir o índice sob demanda")
	}
	if len(hits) != 2 || stats.Selection != "sort/quick" {
		t.Errorf("hits=%d selection=%q", len(hits), stats.Selection)
	}
	base, _ := e.Query("cancel a workflow run")
	if base[0].Doc.ID != hits[0].Doc.ID {
		t.Errorf("linear e indexed divergem: %s vs %s", base[0].Doc.ID, hits[0].Doc.ID)
	}
	if _, _, err := e.Search("x", QueryOptions{Config: "nope"}); err == nil {
		t.Error("config inválida deveria falhar")
	}
}
