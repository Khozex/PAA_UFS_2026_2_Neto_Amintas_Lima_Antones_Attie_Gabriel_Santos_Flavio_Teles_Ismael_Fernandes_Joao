package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"paa/ingestDataPipeline/clean"
	"paa/ingestDataPipeline/extract"
	"paa/ingestDataPipeline/types"
	"paa/ingestDataPipeline/utils"
)

func main() {
	in := flag.String("in", "data/githubApiDoc.json", "")
	out := flag.String("out", "data/processed", "")
	flag.Parse()

	m := types.Manifest{StageMs: map[string]float64{}, Generated: time.Now().Format(time.RFC3339)}
	start := time.Now()

	t := time.Now()
	spec, err := utils.LoadSpec(*in)
	check(err)
	m.StageMs["load"] = ms(t)
	m.Spec, m.Version, m.License = spec.Info.Title, spec.Info.Version, spec.Info.License.Name

	t = time.Now()
	docs := extract.Extract(spec)
	m.StageMs["extract"] = ms(t)

	t = time.Now()
	for i := range docs {
		docs[i] = clean.Clean(docs[i])
		m.Words += len(strings.Fields(docs[i].Text))
	}
	m.StageMs["clean"] = ms(t)

	t = time.Now()
	check(os.MkdirAll(*out, 0o755))
	check(utils.WriteJSONL(filepath.Join(*out, "docs.jsonl"), docs))
	m.StageMs["write"] = ms(t)
	m.Docs = len(docs)
	m.StageMs["total"] = ms(start)

	b, err := utils.WriteManifest(filepath.Join(*out, "manifest.json"), m)
	check(err)
	fmt.Println(string(b))
}

func ms(t time.Time) float64 { return float64(time.Since(t).Microseconds()) / 1000 }

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
