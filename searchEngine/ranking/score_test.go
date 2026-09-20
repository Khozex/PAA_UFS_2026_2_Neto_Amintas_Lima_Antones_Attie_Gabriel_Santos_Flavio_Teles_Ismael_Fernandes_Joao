package ranking

import (
	"testing"

	"paa/searchEngine/corpus"
	"paa/searchEngine/types"
	"paa/searchEngine/utils"
)

func smallCorpus() *types.Corpus {
	return corpus.Load([]types.RawDocument{
		{ID: "a", Summary: "create repository", Path: "/repos", Description: "creates a repository"},
		{ID: "b", Summary: "list issues", Path: "/issues", Description: "lists issues"},
		{ID: "c", Summary: "delete repository webhook", Path: "/repos/hooks", Description: "deletes a webhook"},
	})
}

func TestScoreIsZeroWhenNoQueryTermMatches(t *testing.T) {
	c := smallCorpus()
	if got := Score(c, &c.Docs[1], utils.Normalize("create repository")); got != 0 {
		t.Fatalf("got %v, want 0", got)
	}
}

func TestScoreIsPositiveWhenATermMatches(t *testing.T) {
	c := smallCorpus()
	if got := Score(c, &c.Docs[0], utils.Normalize("create repository")); got <= 0 {
		t.Fatalf("got %v, want > 0", got)
	}
}

func TestScoreIsBitwiseReproducible(t *testing.T) {
	c := smallCorpus()
	terms := utils.Normalize("create repository webhook")
	first := Score(c, &c.Docs[2], terms)
	for i := 0; i < 100; i++ {
		if got := Score(c, &c.Docs[2], terms); got != first {
			t.Fatalf("run %d: got %v, want %v", i, got, first)
		}
	}
}

func TestTermPresentInEveryDocumentContributesNothing(t *testing.T) {
	c := corpus.Load([]types.RawDocument{
		{ID: "a", Summary: "repository alpha"},
		{ID: "b", Summary: "repository beta"},
	})
	if got := Score(c, &c.Docs[0], utils.Normalize("repository")); got != 0 {
		t.Fatalf("got %v, want 0 for idf=0 term", got)
	}
}
