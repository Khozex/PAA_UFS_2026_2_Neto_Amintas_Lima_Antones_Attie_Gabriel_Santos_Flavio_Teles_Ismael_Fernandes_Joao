package extract

import (
	"maps"
	"slices"
	"strings"

	"paa/ingestDataPipeline/types"
)

var methods = []string{"get", "post", "put", "patch", "delete"}

func Extract(s *types.Spec) []types.Document {
	var docs []types.Document
	for _, path := range slices.Sorted(maps.Keys(s.Paths)) {
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

func toDocument(s *types.Spec, op *types.Operation, method, path string) types.Document {
	d := types.Document{
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
		d.Params = append(d.Params, types.Field{Name: p.Name, In: p.In, Required: p.Required, Description: p.Description})
	}
	if op.RequestBody != nil {
		d.Body = schemaFields(s, op.RequestBody.Content["application/json"].Schema, 0)
	}
	return d
}

func schemaFields(s *types.Spec, sc *types.Schema, depth int) []types.Field {
	sc = resolve(s, sc)
	if sc == nil || depth > 1 {
		return nil
	}
	var out []types.Field
	for _, name := range slices.Sorted(maps.Keys(sc.Properties)) {
		desc := ""
		if p := resolve(s, sc.Properties[name]); p != nil {
			desc = p.Description
		}
		out = append(out, types.Field{Name: name, In: "body", Required: slices.Contains(sc.Required, name), Description: desc})
	}
	for _, alt := range slices.Concat(sc.OneOf, sc.AnyOf, sc.AllOf) {
		out = append(out, schemaFields(s, alt, depth+1)...)
	}
	return out
}

func resolve(s *types.Spec, sc *types.Schema) *types.Schema {
	if sc == nil {
		return nil
	}
	if name, ok := strings.CutPrefix(sc.Ref, "#/components/schemas/"); ok {
		return s.Components.Schemas[name]
	}
	return sc
}
