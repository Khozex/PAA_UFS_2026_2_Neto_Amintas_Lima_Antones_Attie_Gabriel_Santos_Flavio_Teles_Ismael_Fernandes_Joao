package utils

import (
	"bufio"
	"encoding/json"
	"os"

	"paa/ingestDataPipeline/types"
)

func LoadSpec(path string) (*types.Spec, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s types.Spec
	return &s, json.Unmarshal(b, &s)
}

func WriteJSONL(path string, docs []types.Document) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	enc := json.NewEncoder(w)
	for _, d := range docs {
		if err := enc.Encode(d); err != nil {
			return err
		}
	}
	return w.Flush()
}

func WriteManifest(path string, m types.Manifest) ([]byte, error) {
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, err
	}
	return b, os.WriteFile(path, b, 0o644)
}
