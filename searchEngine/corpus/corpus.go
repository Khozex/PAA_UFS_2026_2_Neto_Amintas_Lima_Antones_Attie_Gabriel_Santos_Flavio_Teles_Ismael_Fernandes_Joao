package corpus

import (
	"math"
	"strings"

	"paa/searchEngine/types"
	"paa/searchEngine/utils"
)

func Load(raws []types.RawDocument) *types.Corpus {
	corpus := types.NewCorpus()

	for _, raw := range raws {
		doc := PrepareDocuments(raw)
		corpus.Docs = append(corpus.Docs, doc)
		for term := range distinctTerms(doc) {
			corpus.DF[term]++
		}
	}

	total := float64(corpus.N())
	for term, df := range corpus.DF {
		corpus.IDF[term] = math.Log(total / float64(df))
	}
	return corpus
}

func distinctTerms(doc types.Document) map[string]bool {
	seen := map[string]bool{}
	for _, termCounts := range doc.TF {
		for term := range termCounts {
			seen[term] = true
		}
	}
	return seen
}

func PrepareDocuments(raw types.RawDocument) types.Document {
	doc := types.Document{
		ID:      raw.ID,
		Method:  raw.Method,
		Path:    raw.Path,
		Summary: raw.Summary,
		TF:      map[string]map[string]int{},
	}

	fields := map[string]string{
		"summary":     raw.Summary,
		"path":        raw.Path,
		"description": raw.Description,
		"params":      joinFields(raw.Params, raw.Body),
	}

	for field, text := range fields {
		for _, term := range utils.Normalize(text) {
			if doc.TF[field] == nil {
				doc.TF[field] = map[string]int{}
			}
			doc.TF[field][term]++
			doc.Len++
		}
	}
	return doc
}

func joinFields(lists ...[]types.Field) string {
	var builder strings.Builder
	for _, list := range lists {
		for _, field := range list {
			builder.WriteString(strings.ReplaceAll(field.Name, "_", " "))
			builder.WriteByte(' ')
			builder.WriteString(field.Description)
			builder.WriteByte(' ')
		}
	}
	return builder.String()
}
