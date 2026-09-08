package utils

import (
	"bufio"
	"encoding/json"
	"os"

	"paa/searchEngine/types"
)

func ReadJSONL(path string) ([]types.RawDocument, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var docs []types.RawDocument
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	for scanner.Scan() {
		var doc types.RawDocument
		if err := json.Unmarshal(scanner.Bytes(), &doc); err != nil {
			return nil, err
		}
		docs = append(docs, doc)
	}
	return docs, scanner.Err()
}
