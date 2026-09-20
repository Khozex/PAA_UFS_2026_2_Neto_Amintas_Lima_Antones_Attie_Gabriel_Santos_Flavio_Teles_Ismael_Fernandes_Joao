package utils

import (
	"slices"
	"testing"
)

func TestNormalizeLowercasesAndSplitsOnPunctuation(t *testing.T) {
	got := Normalize("Create a Repository, in an Organization!")
	want := []string{"create", "repository", "organization"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestNormalizeKeepsUnderscoreInsideToken(t *testing.T) {
	got := Normalize("per_page owner")
	want := []string{"per_page", "owner"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestNormalizeEmptyAndStopwordsOnly(t *testing.T) {
	for _, text := range []string{"", "   ", "the of a", "!!! ---"} {
		if got := Normalize(text); len(got) != 0 {
			t.Fatalf("Normalize(%q) = %v, want empty", text, got)
		}
	}
}
