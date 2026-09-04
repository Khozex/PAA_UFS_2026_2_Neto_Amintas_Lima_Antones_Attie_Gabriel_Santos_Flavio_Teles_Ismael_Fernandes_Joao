package main

import (
	"encoding/json"
	"os"
)

type Spec struct {
	Info struct {
		Title   string `json:"title"`
		Version string `json:"version"`
		License struct {
			Name string `json:"name"`
		} `json:"license"`
	} `json:"info"`
	Paths      map[string]map[string]*Operation `json:"paths"`
	Components struct {
		Schemas    map[string]*Schema   `json:"schemas"`
		Parameters map[string]Parameter `json:"parameters"`
	} `json:"components"`
}

type Operation struct {
	Summary     string      `json:"summary"`
	Description string      `json:"description"`
	OperationID string      `json:"operationId"`
	Tags        []string    `json:"tags"`
	Parameters  []Parameter `json:"parameters"`
	RequestBody *struct {
		Content map[string]struct {
			Schema *Schema `json:"schema"`
		} `json:"content"`
	} `json:"requestBody"`
	Deprecated   bool `json:"deprecated"`
	ExternalDocs struct {
		URL string `json:"url"`
	} `json:"externalDocs"`
}

type Parameter struct {
	Ref         string `json:"$ref"`
	Name        string `json:"name"`
	In          string `json:"in"`
	Required    bool   `json:"required"`
	Description string `json:"description"`
}

type Schema struct {
	Ref         string             `json:"$ref"`
	Description string             `json:"description"`
	Required    []string           `json:"required"`
	Properties  map[string]*Schema `json:"properties"`
	OneOf       []*Schema          `json:"oneOf"`
	AnyOf       []*Schema          `json:"anyOf"`
	AllOf       []*Schema          `json:"allOf"`
}

func Load(path string) (*Spec, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s Spec
	return &s, json.Unmarshal(b, &s)
}
