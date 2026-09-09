package types

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

type Manifest struct {
	Spec        string             `json:"spec"`
	Version     string             `json:"version"`
	License     string             `json:"license"`
	Docs        int                `json:"docs"`
	Words       int                `json:"words"`
	WordsPerDoc WordStats          `json:"words_per_doc"`
	StageMs     map[string]float64 `json:"stage_ms"`
	Generated   string             `json:"generated_at"`
}

type WordStats struct {
	Min    int     `json:"min"`
	Median float64 `json:"median"`
	Mean   float64 `json:"mean"`
	Max    int     `json:"max"`
}
