package eval

import (
	"testing"
	"time"

	"paa/searchEngine/types"
)

func hits(ids ...string) []string {
	return ids
}

func TestIDs(t *testing.T) {
	in := []types.Hit{{Doc: &types.Document{ID: "a"}}, {Doc: &types.Document{ID: "b"}}}
	got := IDs(in)
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("IDs = %v", got)
	}
}

func TestRank(t *testing.T) {
	cases := []struct {
		hits     []string
		relevant []string
		want     int
	}{
		{hits("a", "b", "c"), []string{"a"}, 1},
		{hits("a", "b", "c"), []string{"c"}, 3},
		{hits("a", "b", "c"), []string{"z"}, 0},
		{hits("a", "b", "c"), []string{"c", "b"}, 2},
		{hits(), []string{"a"}, 0},
	}
	for _, c := range cases {
		if got := Rank(c.hits, c.relevant); got != c.want {
			t.Errorf("Rank(%v) = %d, want %d", c.relevant, got, c.want)
		}
	}
}

func TestPrecisionAtK(t *testing.T) {
	cases := []struct {
		hits     []string
		relevant []string
		k        int
		want     float64
	}{
		{hits("a", "b", "c", "d", "e"), []string{"a"}, 5, 0.2},
		{hits("a", "b", "c", "d", "e"), []string{"a", "b"}, 5, 0.4},
		{hits("a", "b", "c", "d", "e"), []string{"e"}, 4, 0},
		{hits("a", "b"), []string{"a"}, 5, 0.2},
		{hits("a"), []string{"a"}, 0, 0},
		{hits(), []string{"a"}, 5, 0},
	}
	for _, c := range cases {
		if got := PrecisionAtK(c.hits, c.relevant, c.k); got != c.want {
			t.Errorf("PrecisionAtK(%v, k=%d) = %v, want %v", c.relevant, c.k, got, c.want)
		}
	}
}

func TestSummarize(t *testing.T) {
	results := []Result{
		{Rank: 1, PrecisionAtK: 0.2, Stats: types.Stats{Candidates: 10, QueryTime: 100 * time.Microsecond}},
		{Rank: 7, PrecisionAtK: 0, Stats: types.Stats{Candidates: 10, QueryTime: 300 * time.Microsecond}},
		{Rank: 0, PrecisionAtK: 0, Stats: types.Stats{Candidates: 0, QueryTime: 200 * time.Microsecond}},
	}
	s := Summarize(results, 5)
	if s.Queries != 3 || s.HitsAtK != 1 || s.Empty != 1 {
		t.Fatalf("contagens erradas: %+v", s)
	}
	if s.MeanPrecisionAtK < 0.066 || s.MeanPrecisionAtK > 0.067 {
		t.Errorf("P@k médio = %v", s.MeanPrecisionAtK)
	}
	if s.MeanRank != 4 {
		t.Errorf("posição média = %v, want 4", s.MeanRank)
	}
	if s.MeanQueryTime != 200*time.Microsecond {
		t.Errorf("consulta média = %v", s.MeanQueryTime)
	}
	if empty := Summarize(nil, 5); empty.Queries != 0 || empty.MeanPrecisionAtK != 0 {
		t.Errorf("resumo vazio: %+v", empty)
	}
}
