package report

import (
	"fmt"
	"regexp"
	"strings"
)

type Run struct {
	Scenario    string
	Preset      int
	K           int
	Repetition  int
	Config      string
	Selection   string
	N           string
	Candidates  string
	Comparisons string
	LoadMs      string
	IndexUs     string
	QueryUs     string
}

var (
	reHeader = regexp.MustCompile(`config=(\S+)\s+select=(\S+)`)
	reStats  = regexp.MustCompile(`N=(\d+)\s+candidatos=(\d+)\s+comparações=(\d+)\s+seleção=\S+\s+carga=(\S+)\s+índice=(\S+)\s+consulta=(\S+)`)
)

func Parse(output string) (Run, error) {
	header := reHeader.FindStringSubmatch(output)
	stats := reStats.FindStringSubmatch(output)
	if header == nil || stats == nil {
		return Run{}, fmt.Errorf("saída sem linha de estatísticas")
	}
	return Run{
		Config:      header[1],
		Selection:   header[2],
		N:           stats[1],
		Candidates:  stats[2],
		Comparisons: stats[3],
		LoadMs:      toUnit(stats[4], "ms"),
		IndexUs:     toUnit(stats[5], "us"),
		QueryUs:     toUnit(stats[6], "us"),
	}, nil
}

func CSVHeader() string {
	return "cenario,preset,config,selecao,k,N,repeticao,candidatos,comparacoes,carga_ms,indice_us,consulta_us"
}

func (r Run) CSVLine() string {
	return strings.Join([]string{
		r.Scenario, presetLabel(r.Preset), r.Config, r.Selection, fmt.Sprint(r.K), r.N,
		fmt.Sprint(r.Repetition), r.Candidates, r.Comparisons, r.LoadMs, r.IndexUs, r.QueryUs,
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
