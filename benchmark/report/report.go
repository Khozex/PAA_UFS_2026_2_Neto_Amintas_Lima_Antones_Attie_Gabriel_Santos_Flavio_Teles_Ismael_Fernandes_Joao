package report

import (
	"fmt"
	"regexp"
	"strings"

	"paa/searchEngine/eval"
)

type Run struct {
	Scenario     string
	Preset       int
	K            int
	Repetition   int
	Query        string
	Config       string
	Selection    string
	N            string
	Candidates   string
	Comparisons  string
	LoadMs       string
	IndexUs      string
	QueryUs      string
	MemKB        string
	Rank         int
	PrecisionAtK float64
}

var (
	reHeader = regexp.MustCompile(`config=(\S+)\s+select=(\S+)`)
	reStats  = regexp.MustCompile(`N=(\d+)\s+candidatos=(\d+)\s+comparações=(\d+)\s+seleção=\S+\s+carga=(\S+)\s+índice=(\S+)\s+mem=(\d+)kB\s+consulta=(\S+)`)
	reHit    = regexp.MustCompile(`(?m)^\s*\d+\. (\S+)`)
)

func Parse(output string, relevant []string, k int) (Run, error) {
	header := reHeader.FindStringSubmatch(output)
	stats := reStats.FindStringSubmatch(output)
	if header == nil || stats == nil {
		return Run{}, fmt.Errorf("saída sem linha de estatísticas")
	}
	ids := hitIDs(output)
	return Run{
		Config:       header[1],
		Selection:    header[2],
		N:            stats[1],
		Candidates:   stats[2],
		Comparisons:  stats[3],
		LoadMs:       toUnit(stats[4], "ms"),
		IndexUs:      toUnit(stats[5], "us"),
		MemKB:        stats[6],
		QueryUs:      toUnit(stats[7], "us"),
		Rank:         eval.Rank(ids, relevant),
		PrecisionAtK: eval.PrecisionAtK(ids, relevant, k),
	}, nil
}

func hitIDs(output string) []string {
	var ids []string
	for _, match := range reHit.FindAllStringSubmatch(output, -1) {
		ids = append(ids, match[1])
	}
	return ids
}

func CSVHeader() string {
	return "cenario,preset,config,selecao,k,N,query,repeticao,candidatos,comparacoes,carga_ms,indice_us,mem_kb,consulta_us,posicao,p_at_k"
}

func (r Run) CSVLine() string {
	return strings.Join([]string{
		r.Scenario, presetLabel(r.Preset), r.Config, r.Selection, fmt.Sprint(r.K), r.N, fmt.Sprintf("%q", r.Query),
		fmt.Sprint(r.Repetition), r.Candidates, r.Comparisons, r.LoadMs, r.IndexUs, r.MemKB, r.QueryUs,
		fmt.Sprint(r.Rank), fmt.Sprintf("%.2f", r.PrecisionAtK),
	}, ",")
}

func presetLabel(preset int) string {
	if preset == 0 {
		return ""
	}
	return fmt.Sprint(preset)
}

func toUnit(duration, unit string) string {
	value, suffix := splitDuration(duration)
	factor := map[string]map[string]float64{
		"ms": {"ns": 1e-6, "µs": 1e-3, "ms": 1, "s": 1e3},
		"us": {"ns": 1e-3, "µs": 1, "ms": 1e3, "s": 1e6},
	}[unit][suffix]
	return fmt.Sprintf("%.3f", value*factor)
}

func splitDuration(duration string) (float64, string) {
	suffix := strings.TrimLeft(duration, "0123456789.")
	var value float64
	fmt.Sscanf(strings.TrimSuffix(duration, suffix), "%g", &value)
	return value, suffix
}
