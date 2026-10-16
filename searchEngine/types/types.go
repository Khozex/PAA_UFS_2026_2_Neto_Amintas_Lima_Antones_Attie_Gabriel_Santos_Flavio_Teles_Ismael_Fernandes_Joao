package types

import "time"

type Field struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type RawDocument struct {
	ID          string  `json:"id"`
	Method      string  `json:"method"`
	Path        string  `json:"path"`
	Summary     string  `json:"summary"`
	Description string  `json:"description"`
	Params      []Field `json:"params"`
	Body        []Field `json:"body"`
	DocsURL     string  `json:"docs_url"`
	Text        string  `json:"text"`
}

type Document struct {
	ID      string
	Method  string
	Path    string
	Summary string
	DocsURL string
	Text    string
	TF      map[string]map[string]int
	Len     int
}

type DocumentNode struct {
	Path       string
	Parent     *DocumentNode
	Children   map[string]*DocumentNode
	Operations map[string]*Document
}

type Corpus struct {
	Docs         []Document
	DF           map[string]int
	IDF          map[string]float64
	InverseIndex map[string][]int
	GraphRoot    *DocumentNode
}

func NewCorpus() *Corpus {
	return &Corpus{
		DF:  make(map[string]int),
		IDF: make(map[string]float64),
		GraphRoot: &DocumentNode{
			Path:       "",
			Children:   make(map[string]*DocumentNode),
			Operations: make(map[string]*Document),
		},
	}
}

func (c *Corpus) N() int { return len(c.Docs) }

type Hit struct {
	Doc   *Document
	Score float64
}

type Stats struct {
	N           int
	Candidates  int
	Comparisons int
	EmptyQuery  bool
	QueryTime   time.Duration
	Selection   string
}

type Query struct {
	Query    string   `json:"query"`
	Relevant []string `json:"relevant"`
}
