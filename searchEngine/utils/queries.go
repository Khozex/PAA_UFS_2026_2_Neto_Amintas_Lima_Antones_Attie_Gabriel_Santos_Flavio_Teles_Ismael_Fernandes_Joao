package utils

import (
	"encoding/json"
	"os"

	"paa/searchEngine/types"
)

func ReadQueries(path string) ([]types.Query, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var queries []types.Query
	if err := json.Unmarshal(data, &queries); err != nil {
		return nil, err
	}
	return queries, nil
}
