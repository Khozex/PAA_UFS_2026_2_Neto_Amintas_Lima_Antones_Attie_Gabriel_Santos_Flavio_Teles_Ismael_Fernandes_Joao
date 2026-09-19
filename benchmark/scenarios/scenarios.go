package scenarios

import "fmt"

type Scenario struct {
	Name        string
	Preset      int
	Select      string
	K           int
	Limit       int
	Repetitions int
}

const (
	FullCorpus = 0
	HalfCorpus = 612
	DefaultK   = 5
	Reps       = 2
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
	if s.Limit > 0 {
		args = append(args, "-limit", fmt.Sprint(s.Limit))
	}
	return args
}

func sizeLabel(limit int) string {
	if limit == FullCorpus {
		return "full"
	}
	return fmt.Sprintf("n%d", limit)
}
