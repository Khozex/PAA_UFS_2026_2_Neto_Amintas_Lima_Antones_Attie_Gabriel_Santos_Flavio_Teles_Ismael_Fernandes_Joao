package scenarios

import "fmt"

type Scenario struct {
	Name          string
	Preset        int
	Select        string
	OrderStrategy string
	K             int
	Limit         int
	Repetitions   int
	Expand        string
	Hops          int
	Budget        float64
	Plan          bool
}

const (
	FullCorpus  = 0
	HalfCorpus  = 612
	DefaultK    = 5
	Reps        = 2
	GraphReps   = 3
	GraphHops   = 3
	GraphBudget = 3
)

func All() []Scenario {
	seen := map[string]bool{}
	var out []Scenario
	for _, s := range append(Required(), ByK()...) {
		if seen[s.Name] {
			continue
		}
		seen[s.Name] = true
		out = append(out, s)
	}
	return out
}

func Required() []Scenario {
	var out []Scenario
	for _, preset := range []int{1, 2, 3} {
		for _, limit := range []int{FullCorpus, HalfCorpus} {
			out = append(out, Scenario{
				Name:        fmt.Sprintf("preset%d_%s_k%d", preset, sizeLabel(limit), DefaultK),
				Preset:      preset,
				K:           DefaultK,
				Limit:       limit,
				Repetitions: Reps,
			})
		}
	}
	return out
}

func ByK() []Scenario {
	var out []Scenario
	for _, k := range []int{5, 50, 500, 1224} {
		for _, preset := range []int{1, 2, 3} {
			out = append(out, Scenario{
				Name:        fmt.Sprintf("preset%d_full_k%d", preset, k),
				Preset:      preset,
				K:           k,
				Limit:       FullCorpus,
				Repetitions: Reps,
			})
		}
		out = append(out, Scenario{
			Name:        fmt.Sprintf("heap_full_k%d", k),
			Select:      "heap",
			K:           k,
			Limit:       FullCorpus,
			Repetitions: Reps,
		})
		for _, strategy := range []string{"quick", "std"} {
			out = append(out, Scenario{
				Name:          fmt.Sprintf("sort_%s_full_k%d", strategy, k),
				Select:        "sort",
				OrderStrategy: strategy,
				K:             k,
				Limit:         FullCorpus,
				Repetitions:   Reps,
			})
		}
	}
	return out
}

func Graph() []Scenario {
	approaches := []struct {
		name   string
		expand string
		plan   bool
	}{
		{"base", "", false},
		{"bfs", "bfs", false},
		{"dijkstra", "dijkstra", false},
		{"plan", "", true},
	}
	var out []Scenario
	for _, approach := range approaches {
		for _, limit := range []int{FullCorpus, HalfCorpus} {
			out = append(out, Scenario{
				Name:        fmt.Sprintf("grafo_%s_%s", approach.name, sizeLabel(limit)),
				Preset:      2,
				K:           DefaultK,
				Limit:       limit,
				Repetitions: GraphReps,
				Expand:      approach.expand,
				Hops:        GraphHops,
				Budget:      GraphBudget,
				Plan:        approach.plan,
			})
		}
	}
	return out
}

func (s Scenario) Args(query string) []string {
	args := []string{"-query", query, "-k", fmt.Sprint(s.K)}
	if s.Preset != 0 {
		args = append(args, "-preset", fmt.Sprint(s.Preset))
	}
	if s.Select != "" {
		args = append(args, "-select", s.Select)
	}
	if s.OrderStrategy != "" {
		args = append(args, "-order_strategy", s.OrderStrategy)
	}
	if s.Limit > 0 {
		args = append(args, "-limit", fmt.Sprint(s.Limit))
	}
	if s.Expand != "" {
		args = append(args, "-expand", s.Expand, "-hops", fmt.Sprint(s.Hops), "-budget", fmt.Sprint(s.Budget))
	}
	if s.Plan {
		args = append(args, "-plan")
	}
	return args
}

func sizeLabel(limit int) string {
	if limit == FullCorpus {
		return "full"
	}
	return fmt.Sprintf("n%d", limit)
}
