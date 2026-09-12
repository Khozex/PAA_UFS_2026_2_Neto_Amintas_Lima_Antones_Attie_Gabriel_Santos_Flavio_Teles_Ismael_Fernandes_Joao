package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	queryText = "create a repository in an organization"
	defaultK  = 5
)

type Scenario struct {
	Name          string
	Config        string
	Order         string
	OrderStrategy string
	Limit         int
	Repetitions   int
}

func main() {
	repoRoot, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	//tres configurações: linear, indexed, linear com heap-sort
	//pra cada configuração: quick-sort e heap-sort
	//pra cada configuração: corpus completo e limitado a 100
	scenarios := []Scenario{
		{Name: "linear_full_quick", Config: "linear", Order: "desc", OrderStrategy: "quick", Limit: 0, Repetitions: 2},
		{Name: "indexed_full_quick", Config: "indexed", Order: "desc", OrderStrategy: "quick", Limit: 0, Repetitions: 2},
		{Name: "linear_full_heap", Config: "linear", Order: "desc", OrderStrategy: "heap", Limit: 0, Repetitions: 2},
		{Name: "linear_subset_quick", Config: "linear", Order: "desc", OrderStrategy: "quick", Limit: 100, Repetitions: 2},
		{Name: "indexed_subset_quick", Config: "indexed", Order: "desc", OrderStrategy: "quick", Limit: 100, Repetitions: 2},
		{Name: "linear_subset_heap", Config: "linear", Order: "desc", OrderStrategy: "heap", Limit: 100, Repetitions: 2},
	}

	resultsPath := filepath.Join(repoRoot, "artefatos", "benchmark_results.txt")
	if err := os.MkdirAll(filepath.Dir(resultsPath), 0o755); err != nil {
		panic(err)
	}

	var out strings.Builder
	out.WriteString("=== Benchmark de busca ===\n")
	out.WriteString(fmt.Sprintf("timestamp: %s\n", time.Now().Format(time.RFC3339)))
	out.WriteString(fmt.Sprintf("os: %s/%s\n", runtime.GOOS, runtime.GOARCH))
	out.WriteString(fmt.Sprintf("go_version: %s\n", runtime.Version()))
	out.WriteString(fmt.Sprintf("query: %s\n", queryText))
	out.WriteString(fmt.Sprintf("k: %d\n\n", defaultK))

	totalRuns := 0
	for _, scenario := range scenarios {
		totalRuns += scenario.Repetitions
	}
	out.WriteString(fmt.Sprintf("total_de_execucoes: %d\n\n", totalRuns))

	for _, scenario := range scenarios {
		for rep := 1; rep <= scenario.Repetitions; rep++ {
			args := []string{
				"run",
				"./searchEngine",
				"-query",
				queryText,
				"-k",
				fmt.Sprintf("%d", defaultK),
				"-config",
				scenario.Config,
				"-order",
				scenario.Order,
				"-order_strategy",
				scenario.OrderStrategy,
			}
			if scenario.Limit > 0 {
				args = append(args, "-limit", fmt.Sprintf("%d", scenario.Limit))
			}

			start := time.Now()
			cmd := exec.Command("go", args...)
			cmd.Dir = repoRoot
			var buf bytes.Buffer
			cmd.Stdout = &buf
			cmd.Stderr = &buf

			err := cmd.Run()
			elapsed := time.Since(start)

			out.WriteString(fmt.Sprintf("=== %s | repeticao %d | duracao=%s ===\n", scenario.Name, rep, elapsed.Round(time.Millisecond)))
			out.WriteString(fmt.Sprintf("comando: go %s\n", strings.Join(args, " ")))
			if err != nil {
				out.WriteString(fmt.Sprintf("erro: %v\n", err))
			}
			out.WriteString(buf.String())
			out.WriteString("\n\n")
		}
	}

	if err := os.WriteFile(resultsPath, []byte(out.String()), 0o644); err != nil {
		panic(err)
	}

	fmt.Printf("Resultados salvos em: %s\n", resultsPath)
	fmt.Printf("Total de execucoes: %d\n", totalRuns)
}
