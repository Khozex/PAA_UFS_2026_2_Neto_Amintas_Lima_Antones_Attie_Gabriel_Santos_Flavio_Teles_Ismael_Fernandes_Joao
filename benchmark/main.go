package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"paa/benchmark/hardware"
	"paa/benchmark/report"
	"paa/benchmark/scenarios"
	"paa/searchEngine/types"
	"paa/searchEngine/utils"
)

const queriesPath = "data/queries.json"

var reHitLine = regexp.MustCompile(`^\s*\d+\. \S+`)

func main() {
	root := utils.Resolve(".")
	binary := filepath.Join(root, "bin", "search")
	check(build(root, binary))

	queries, err := utils.ReadQueries(utils.Resolve(queriesPath))
	check(err)

	all := scenarios.All()
	var log, csv strings.Builder
	writeHeader(&log, all, queries)
	csv.WriteString(report.CSVHeader() + "\n")

	for _, scenario := range all {
		for _, query := range queries {
			for rep := 1; rep <= scenario.Repetitions; rep++ {
				args := scenario.Args(query.Query)
				output, elapsed, err := execute(root, binary, args)
				writeExecution(&log, scenario.Name, query.Query, rep, elapsed, args, output, err)
				run, parseErr := report.Parse(output, query.Relevant, scenario.K)
				if err != nil || parseErr != nil {
					continue
				}
				run.Scenario, run.Preset, run.K, run.Query, run.Repetition = scenario.Name, scenario.Preset, scenario.K, query.Query, rep
				csv.WriteString(run.CSVLine() + "\n")
			}
		}
	}

	outDir := filepath.Join(root, "artefatos")
	check(os.MkdirAll(outDir, 0o755))
	check(os.WriteFile(filepath.Join(outDir, "benchmark_results.txt"), []byte(log.String()), 0o644))
	check(os.WriteFile(filepath.Join(outDir, "benchmark_results.csv"), []byte(csv.String()), 0o644))
	fmt.Printf("Cenários: %d   Queries: %d   Execuções: %d\n", len(all), len(queries), countRuns(all, queries))
	fmt.Printf("Resultados em: %s\n", outDir)
}

func build(root, binary string) error {
	cmd := exec.Command("go", "build", "-o", binary, "./searchEngine")
	cmd.Dir = root
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func execute(root, binary string, args []string) (string, time.Duration, error) {
	var buf bytes.Buffer
	cmd := exec.Command(binary, args...)
	cmd.Dir = root
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	start := time.Now()
	err := cmd.Run()
	return buf.String(), time.Since(start), err
}

func writeHeader(log *strings.Builder, all []scenarios.Scenario, queries []types.Query) {
	hw := hardware.Detect()
	fmt.Fprintf(log, "=== Benchmark de busca ===\n")
	fmt.Fprintf(log, "timestamp: %s\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(log, "os: %s/%s\n", hw.OS, hw.Arch)
	fmt.Fprintf(log, "go_version: %s\n", hw.Go)
	fmt.Fprintf(log, "cpu: %s\n", hw.CPU)
	fmt.Fprintf(log, "cores: %d\n", hw.Cores)
	fmt.Fprintf(log, "ram: %s\n", hw.RAMGiB)
	fmt.Fprintf(log, "queries: %d (%s)\n", len(queries), queriesPath)
	fmt.Fprintf(log, "cenarios: %d\n", len(all))
	fmt.Fprintf(log, "total_de_execucoes: %d\n\n", countRuns(all, queries))
}

func writeExecution(log *strings.Builder, name, query string, rep int, elapsed time.Duration, args []string, output string, err error) {
	fmt.Fprintf(log, "=== %s | %q | repeticao %d | processo=%s ===\n", name, query, rep, elapsed.Round(time.Millisecond))
	fmt.Fprintf(log, "comando: bin/search %s\n", strings.Join(quote(args), " "))
	if err != nil {
		fmt.Fprintf(log, "erro: %v\n", err)
	}
	log.WriteString(trimHits(output, maxLoggedHits))
	log.WriteString("\n\n")
}

const maxLoggedHits = 5

func trimHits(output string, keep int) string {
	var out []string
	hits, hidden := 0, 0
	for _, line := range strings.Split(output, "\n") {
		isHit := reHitLine.MatchString(line)
		isDetail := strings.HasPrefix(line, "    ")
		if isHit {
			hits++
		}
		if (isHit || isDetail) && hits > keep {
			hidden++
			continue
		}
		out = append(out, line)
	}
	if hidden > 0 {
		out = append(out, fmt.Sprintf("... (%d resultados omitidos do log; contagem completa no CSV)", hidden/2))
	}
	return strings.Join(out, "\n")
}

func quote(args []string) []string {
	out := make([]string, len(args))
	for i, arg := range args {
		if strings.Contains(arg, " ") {
			arg = fmt.Sprintf("%q", arg)
		}
		out[i] = arg
	}
	return out
}

func countRuns(all []scenarios.Scenario, queries []types.Query) int {
	total := 0
	for _, s := range all {
		total += s.Repetitions * len(queries)
	}
	return total
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
