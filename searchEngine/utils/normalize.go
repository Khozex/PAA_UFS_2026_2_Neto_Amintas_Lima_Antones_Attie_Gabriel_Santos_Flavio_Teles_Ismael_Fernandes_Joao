package utils

import (
	"regexp"
	"strings"
)

var tokenRe = regexp.MustCompile(`[a-z0-9_]+`)

var stopwords = map[string]bool{
	"the": true, "a": true, "an": true, "of": true, "to": true, "in": true,
	"for": true, "is": true, "are": true, "and": true, "or": true, "not": true,
	"this": true, "that": true, "with": true, "by": true, "on": true, "be": true,
	"can": true, "you": true, "your": true, "will": true, "if": true, "it": true,
	"as": true, "at": true, "from": true, "all": true, "any": true, "only": true,
	"must": true, "use": true, "using": true, "see": true, "more": true,
	"need": true, "without": true, "when": true, "which": true, "has": true,
	"have": true, "was": true, "were": true, "been": true, "also": true,
}

func Normalize(text string) []string {
	var tokens []string
	for _, token := range tokenRe.FindAllString(strings.ToLower(text), -1) {
		if !stopwords[token] {
			tokens = append(tokens, token)
		}
	}
	return tokens
}
