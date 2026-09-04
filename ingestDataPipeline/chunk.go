package main

import (
	"fmt"
	"strings"
)

type Chunk struct {
	ID    string `json:"id"`
	DocID string `json:"doc_id"`
	Seq   int    `json:"seq"`
	Text  string `json:"text"`
}

func ChunkAll(docs []Document, size, overlap int) []Chunk {
	var out []Chunk
	for _, d := range docs {
		words := strings.Fields(d.Text)
		if size <= 0 || len(words) <= size {
			out = append(out, Chunk{d.ID + "#0", d.ID, 0, d.Text})
			continue
		}
		step := max(size-overlap, 1)
		for seq, start := 0, 0; start < len(words); seq, start = seq+1, start+step {
			end := min(start+size, len(words))
			out = append(out, Chunk{fmt.Sprintf("%s#%d", d.ID, seq), d.ID, seq, strings.Join(words[start:end], " ")})
			if end == len(words) {
				break
			}
		}
	}
	return out
}
