package main

import (
	"slices"
	"strings"
)

var methods = []string{"get", "post", "put", "patch", "delete"}

func Extract(s *Spec) []Document {
	var docs []Document
	for _, path := range sortedKeys(s.Paths) {
		for _, m := range methods {
			op := s.Paths[path][m]
			if op == nil {
				continue
			}
			docs = append(docs, toDocument(s, op, strings.ToUpper(m), path))
		}
	}
	return docs
}

func toDocument(s *Spec, op *Operation, method, path string) Document {
	d := Document{
		ID:          op.OperationID,
		Method:      method,
		Path:        path,
		Summary:     op.Summary,
		Description: op.Description,
		Deprecated:  op.Deprecated,
		DocsURL:     op.ExternalDocs.URL,
	}
	if len(op.Tags) > 0 {
		d.Category = op.Tags[0]
	}
	for _, p := range op.Parameters {
		if name, ok := strings.CutPrefix(p.Ref, "#/components/parameters/"); ok {
			p = s.Components.Parameters[name]
		}
		d.Params = append(d.Params, Field{p.Name, p.In, p.Required, p.Description})
	}
	if op.RequestBody != nil {
		d.Body = schemaFields(s, op.RequestBody.Content["application/json"].Schema, 0)
	}
	return d
}

func schemaFields(s *Spec, sc *Schema, depth int) []Field {
	sc = resolve(s, sc)
	if sc == nil || depth > 1 {
		return nil
	}
	var out []Field
	for _, name := range sortedKeys(sc.Properties) {
		desc := ""
		if p := resolve(s, sc.Properties[name]); p != nil {
			desc = p.Description
		}
		out = append(out, Field{name, "body", slices.Contains(sc.Required, name), desc})
	}
	for _, alt := range slices.Concat(sc.OneOf, sc.AnyOf, sc.AllOf) {
		out = append(out, schemaFields(s, alt, depth+1)...)
	}
	return out
}

func resolve(s *Spec, sc *Schema) *Schema {
	if sc == nil {
		return nil
	}
	if name, ok := strings.CutPrefix(sc.Ref, "#/components/schemas/"); ok {
		return s.Components.Schemas[name]
	}
	return sc
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}
