package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Manifest struct {
	Spec      string             `json:"spec"`
	Version   string             `json:"version"`
	License   string             `json:"license"`
	Docs      int                `json:"docs"`
	Chunks    int                `json:"chunks"`
	ChunkSize int                `json:"chunk_size"`
	Overlap   int                `json:"overlap"`
	Words     int                `json:"words"`
	StageMs   map[string]float64 `json:"stage_ms"`
	Generated string             `json:"generated_at"`
}

func main() {
	in := flag.String("in", "data/githubApiDoc.json", "")
	out := flag.String("out", "data/processed", "")
	size := flag.Int("chunk", 0, "")
	overlap := flag.Int("overlap", 0, "")
	flag.Parse()

	m := Manifest{ChunkSize: *size, Overlap: *overlap, StageMs: map[string]float64{}, Generated: time.Now().Format(time.RFC3339)}
	start := time.Now()

	t := time.Now()
	spec, err := Load(*in)
	check(err)
	m.StageMs["load"] = ms(t)
	m.Spec, m.Version, m.License = spec.Info.Title, spec.Info.Version, spec.Info.License.Name

	t = time.Now()
	docs := Extract(spec)
	m.StageMs["extract"] = ms(t)

	t = time.Now()
	for i := range docs {
		docs[i] = Clean(docs[i])
		m.Words += len(strings.Fields(docs[i].Text))
	}
	m.StageMs["clean"] = ms(t)

	t = time.Now()
	chunks := ChunkAll(docs, *size, *overlap)
	m.StageMs["chunk"] = ms(t)

	t = time.Now()
	check(os.MkdirAll(*out, 0o755))
	check(writeJSONL(filepath.Join(*out, "docs.jsonl"), docs))
	check(writeJSONL(filepath.Join(*out, "chunks.jsonl"), chunks))
	m.StageMs["write"] = ms(t)
	m.Docs, m.Chunks = len(docs), len(chunks)
	m.StageMs["total"] = ms(start)

	b, _ := json.MarshalIndent(m, "", "  ")
	check(os.WriteFile(filepath.Join(*out, "manifest.json"), b, 0o644))
	fmt.Println(string(b))
}

func writeJSONL[T any](path string, items []T) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	enc := json.NewEncoder(w)
	for _, it := range items {
		if err := enc.Encode(it); err != nil {
			return err
		}
	}
	return w.Flush()
}

func ms(t time.Time) float64 { return float64(time.Since(t).Microseconds()) / 1000 }

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
