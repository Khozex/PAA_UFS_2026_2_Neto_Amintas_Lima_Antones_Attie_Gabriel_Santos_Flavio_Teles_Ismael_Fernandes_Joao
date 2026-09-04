package main

type Field struct {
	Name        string `json:"name"`
	In          string `json:"in"`
	Required    bool   `json:"required"`
	Description string `json:"description"`
}

type Document struct {
	ID          string  `json:"id"`
	Method      string  `json:"method"`
	Path        string  `json:"path"`
	Summary     string  `json:"summary"`
	Description string  `json:"description"`
	Category    string  `json:"category"`
	Params      []Field `json:"params"`
	Body        []Field `json:"body"`
	Deprecated  bool    `json:"deprecated"`
	DocsURL     string  `json:"docs_url"`
	Text        string  `json:"text"`
}
