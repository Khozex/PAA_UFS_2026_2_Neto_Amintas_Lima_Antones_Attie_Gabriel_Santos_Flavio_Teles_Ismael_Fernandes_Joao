// Package engine junta as peças do searchEngine em um objeto que carrega o
// corpus uma vez e responde quantas consultas forem necessárias. É usado pelo
// CLI (searchEngine/main.go) e pelo servidor MCP (ui/mcp).
package engine

import (
	"fmt"
	"sync"
	"time"

	"paa/searchEngine/corpus"
	indexedsearch "paa/searchEngine/indexedSearch"
	linearsearch "paa/searchEngine/linearSearch"
	"paa/searchEngine/selection"
	"paa/searchEngine/sorting"
	"paa/searchEngine/types"
	"paa/searchEngine/utils"
)

const (
	DefaultCorpusPath = "data/processed/docs.jsonl"
	DefaultK          = 5
)

type Options struct {
	CorpusPath    string // caminho do docs.jsonl, relativo ao go.mod
	Limit         int    // usa só os N primeiros documentos; 0 = todos
	Config        string // linear | indexed
	Select        string // topk | heap | sort
	OrderStrategy string // quick | heap | merge | std, só com Select=sort
	K             int    // quantos resultados por consulta
}

func DefaultOptions() Options {
	return Options{
		CorpusPath:    DefaultCorpusPath,
		Config:        "linear",
		Select:        "topk",
		OrderStrategy: "merge",
		K:             DefaultK,
	}
}

type Preset struct {
	Config, Select, OrderStrategy string
}

var Presets = map[int]Preset{
	1: {"linear", "topk", "merge"},
	2: {"indexed", "topk", "merge"},
	3: {"linear", "sort", "merge"},
}

func (o *Options) ApplyPreset(id int) error {
	if id == 0 {
		return nil
	}
	p, ok := Presets[id]
	if !ok {
		return fmt.Errorf("preset desconhecido: %d", id)
	}
	o.Config, o.Select, o.OrderStrategy = p.Config, p.Select, p.OrderStrategy
	return nil
}

type searchFunc func(*types.Corpus, string, selection.Selector) ([]types.Hit, types.Stats)

var searchers = map[string]searchFunc{
	"linear":  linearsearch.Search,
	"indexed": indexedsearch.Search,
}

func (o Options) Validate() error {
	if o.K < 0 {
		return fmt.Errorf("k deve ser >= 0, recebido: %d", o.K)
	}
	algo, ok := sorting.Registry[o.OrderStrategy]
	if !ok {
		return fmt.Errorf("order_strategy desconhecida: %s", o.OrderStrategy)
	}
	if _, err := selection.New(o.Select, o.K, algo); err != nil {
		return err
	}
	if _, ok := searchers[o.Config]; !ok {
		return fmt.Errorf("config desconhecida: %s", o.Config)
	}
	return nil
}

type Engine struct {
	Corpus    *types.Corpus
	Label     string        // rótulo da seleção: topk, heap ou sort/<algoritmo>
	LoadTime  time.Duration // leitura do jsonl + TF/DF/IDF
	IndexTime time.Duration // construção do índice invertido; 0 no linear
	opts      Options
	search    searchFunc
	algo      sorting.Algorithm
	indexOnce sync.Mutex
}

func Load(opts Options) (*Engine, error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}
	start := time.Now()
	raws, err := utils.ReadJSONL(utils.Resolve(opts.CorpusPath))
	if err != nil {
		return nil, err
	}
	if opts.Limit > 0 && opts.Limit < len(raws) {
		raws = raws[:opts.Limit]
	}
	e := &Engine{
		Corpus: corpus.Load(raws),
		Label:  selectionLabel(opts.Select, opts.OrderStrategy),
		opts:   opts,
		search: searchers[opts.Config],
		algo:   sorting.Registry[opts.OrderStrategy],
	}
	e.LoadTime = time.Since(start)

	if opts.Config == "indexed" {
		start = time.Now()
		indexedsearch.BuildIndex(e.Corpus)
		e.IndexTime = time.Since(start)
	}
	return e, nil
}

func (e *Engine) Options() Options { return e.opts }

// Query devolve os K melhores documentos para a pergunta, com K das opções.
func (e *Engine) Query(text string) ([]types.Hit, types.Stats) {
	return e.QueryK(text, e.opts.K)
}

// QueryK é Query com um k diferente do configurado.
func (e *Engine) QueryK(text string, k int) ([]types.Hit, types.Stats) {
	sel, _ := selection.New(e.opts.Select, k, e.algo)
	hits, stats := e.search(e.Corpus, text, sel)
	stats.Selection = e.Label
	return hits, stats
}

// QueryOptions muda config, seleção e k de uma única consulta, sem recarregar
// o corpus. Campos vazios usam o valor das Options de Load.
type QueryOptions struct {
	Config        string
	Select        string
	OrderStrategy string
	K             int
}

// Search roda uma consulta com QueryOptions. Se pedir "indexed" e o índice
// ainda não existe, constrói na hora (uma vez) e soma o tempo em IndexTime.
func (e *Engine) Search(text string, q QueryOptions) ([]types.Hit, types.Stats, error) {
	o := e.opts
	if q.Config != "" {
		o.Config = q.Config
	}
	if q.Select != "" {
		o.Select = q.Select
	}
	if q.OrderStrategy != "" {
		o.OrderStrategy = q.OrderStrategy
	}
	if q.K > 0 {
		o.K = q.K
	}
	if err := o.Validate(); err != nil {
		return nil, types.Stats{}, err
	}
	if o.Config == "indexed" {
		e.ensureIndex()
	}
	sel, _ := selection.New(o.Select, o.K, sorting.Registry[o.OrderStrategy])
	hits, stats := searchers[o.Config](e.Corpus, text, sel)
	stats.Selection = selectionLabel(o.Select, o.OrderStrategy)
	return hits, stats, nil
}

func (e *Engine) ensureIndex() {
	e.indexOnce.Lock()
	defer e.indexOnce.Unlock()
	if e.Corpus.InverseIndex != nil {
		return
	}
	start := time.Now()
	indexedsearch.BuildIndex(e.Corpus)
	e.IndexTime = time.Since(start)
}

func selectionLabel(mode, strategy string) string {
	if mode == "sort" {
		return mode + "/" + strategy
	}
	return mode
}
