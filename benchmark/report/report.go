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
	Graph        GraphRun
}

type GraphRun struct {
	Vertices      string
	Edges         string
	BuildUs       string
	Expand        string
	Visited       string
	EdgesScanned  string
	Related       string
	ExpandUs      string
	FoundExpanded bool
	Plans         string
	Prerequisites string
	PlanEdges     string
	PlanUs        string
}

var (
	reHeader  = regexp.MustCompile(`config=(\S+)\s+select=(\S+)`)
	reStats   = regexp.MustCompile(`N=(\d+)\s+candidatos=(\d+)\s+comparações=(\d+)\s+seleção=\S+\s+carga=(\S+)\s+índice=(\S+)\s+mem=(\d+)kB\s+consulta=(\S+)`)
	reHit     = regexp.MustCompile(`(?m)^\s*\d+\. (\S+)`)
	reGraph   = regexp.MustCompile(`grafo: V=(\d+)\s+E=(\d+)\s+construção=(\S+)\s+expansão=(\S+)\s+visitados=(\d+)\s+arestas=(\d+)\s+relacionadas=(\d+)\s+expansão_tempo=(\S+)(?:\s+planos=(\d+)\s+pré_requisitos=(\d+)\s+plano_arestas=(\d+)\s+plano_tempo=(\S+))?`)
	reRelated = regexp.MustCompile(`(?m)^\s+custo=\S+\s+(\S+)`)
)

var graphSections = []string{"\nrelacionadas pelo grafo", "\nplano de chamadas"}

func Parse(output string, relevant []string, k int) (Run, error) {
	header := reHeader.FindStringSubmatch(output)
	stats := reStats.FindStringSubmatch(output)
	if header == nil || stats == nil {
		return Run{}, fmt.Errorf("saída sem linha de estatísticas")
	}
	ids := hitIDs(searchSection(output))
	rank := eval.Rank(ids, relevant)
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
		Rank:         rank,
		PrecisionAtK: eval.PrecisionAtK(ids, relevant, k),
		Graph:        parseGraph(output, relevant, rank > 0),
	}, nil
}

func searchSection(output string) string {
	for _, marker := range graphSections {
		if cut := strings.Index(output, marker); cut >= 0 {
			output = output[:cut]
		}
	}
	return output
}

func parseGraph(output string, relevant []string, foundInHits bool) GraphRun {
	graph := reGraph.FindStringSubmatch(output)
	if graph == nil {
		return GraphRun{FoundExpanded: foundInHits}
	}
	run := GraphRun{
		Vertices:      graph[1],
		Edges:         graph[2],
		BuildUs:       toUnit(graph[3], "us"),
		Expand:        graph[4],
		Visited:       graph[5],
		EdgesScanned:  graph[6],
		Related:       graph[7],
		ExpandUs:      toUnit(graph[8], "us"),
		FoundExpanded: foundInHits || eval.Rank(relatedIDs(output), relevant) > 0,
	}
	if graph[9] != "" {
		run.Plans, run.Prerequisites, run.PlanEdges, run.PlanUs = graph[9], graph[10], graph[11], toUnit(graph[12], "us")
	}
	return run
}

func relatedIDs(output string) []string {
	var ids []string
	for _, match := range reRelated.FindAllStringSubmatch(output, -1) {
		ids = append(ids, match[1])
	}
	return ids
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

func GraphCSVHeader() string {
	return "cenario,k,N,query,repeticao,carga_ms,indice_us,mem_kb,consulta_us,posicao," +
		"V,E,construcao_grafo_us,expansao,visitados,arestas,relacionadas,expansao_us,achou_expandido," +
		"planos,pre_requisitos,plano_arestas,plano_us"
}

func (r Run) GraphCSVLine() string {
	graph := r.Graph
	return strings.Join([]string{
		r.Scenario, fmt.Sprint(r.K), r.N, fmt.Sprintf("%q", r.Query), fmt.Sprint(r.Repetition),
		r.LoadMs, r.IndexUs, r.MemKB, r.QueryUs, fmt.Sprint(r.Rank),
		graph.Vertices, graph.Edges, graph.BuildUs, graph.Expand, graph.Visited, graph.EdgesScanned,
		graph.Related, graph.ExpandUs, fmt.Sprint(graph.FoundExpanded),
		graph.Plans, graph.Prerequisites, graph.PlanEdges, graph.PlanUs,
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
