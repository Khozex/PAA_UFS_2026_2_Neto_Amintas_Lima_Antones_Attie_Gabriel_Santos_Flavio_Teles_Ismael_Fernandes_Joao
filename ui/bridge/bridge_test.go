package bridge

import (
	"strings"
	"testing"
)

func TestLooping(t *testing.T) {
	normal := strings.Repeat("The endpoint is GET /repos/{owner}/{repo}/issues. ", 2) + "Parameters: owner, repo, state, labels, sort, direction, per_page, page."
	if looping(normal) {
		t.Error("texto normal não deveria ser loop")
	}
	loop := "Docs: https://docs.github.com/rest/x " + strings.Repeat("- owner: The account owner of the repository.\n", 6)
	if !looping(loop) {
		t.Error("linha repetida 6 vezes deveria ser loop")
	}
	if looping("curto") {
		t.Error("texto curto nunca é loop")
	}
}

func TestCleanQuery(t *testing.T) {
	cases := map[string]string{
		"\"delete release\"\n":             "delete release",
		"Query: cancel workflow run.":      "cancel workflow run",
		"  list issues  ":                  "list issues",
		"create repo\ncreate repo\ncreate": "create repo",
	}
	for in, want := range cases {
		if got := cleanQuery(in); got != want {
			t.Errorf("cleanQuery(%q) = %q, quer %q", in, got, want)
		}
	}
}

func TestLanguageInstruction(t *testing.T) {
	if !strings.Contains(languageInstruction("como eu apago uma release?"), "Portuguese") {
		t.Error("pergunta em português")
	}
	if strings.Contains(languageInstruction("how do I cancel a workflow run?"), "Portuguese") {
		t.Error("'do' em inglês não é português")
	}
}
