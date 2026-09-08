package clean

import (
	"regexp"
	"strings"

	"paa/ingestDataPipeline/types"
)

var (
	reBoilerplate = regexp.MustCompile(`(?s)\*\*Fine-grained access tokens for .*$`)
	reNote        = regexp.MustCompile(`>\s*\[!\w+\]`)
	reLink        = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	reHTML        = regexp.MustCompile(`<[^>]+>`)
	reMarkup      = regexp.MustCompile("[*`>#]+")
	reSpace       = regexp.MustCompile(`\s+`)
	rePath        = strings.NewReplacer("/", " ", "{", "", "}", "", "_", " ", "-", " ")
)

func CleanText(s string) string {
	s = reBoilerplate.ReplaceAllString(s, "")
	s = reNote.ReplaceAllString(s, "")
	s = reLink.ReplaceAllString(s, "$1")
	s = reHTML.ReplaceAllString(s, " ")
	s = reMarkup.ReplaceAllString(s, " ")
	s = reSpace.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

func Clean(d types.Document) types.Document {
	d.Summary = CleanText(d.Summary)
	d.Description = CleanText(d.Description)
	for i := range d.Params {
		d.Params[i].Description = CleanText(d.Params[i].Description)
	}
	for i := range d.Body {
		d.Body[i].Description = CleanText(d.Body[i].Description)
	}

	parts := []string{d.Summary, d.Method + " " + strings.TrimSpace(reSpace.ReplaceAllString(rePath.Replace(d.Path), " "))}
	if d.Description != "" {
		parts = append(parts, d.Description)
	}
	for _, f := range slicesConcat(d.Params, d.Body) {
		p := strings.ReplaceAll(f.Name, "_", " ")
		if f.Description != "" {
			p += ": " + f.Description
		}
		parts = append(parts, p)
	}
	d.Text = strings.Join(parts, ". ")
	return d
}

func slicesConcat(a, b []types.Field) []types.Field {
	return append(append([]types.Field{}, a...), b...)
}
