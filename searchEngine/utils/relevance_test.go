package utils

import (
	"testing"

	"paa/searchEngine/types"
)

func hit(id string, length int, score float64) types.Hit {
	return types.Hit{Doc: &types.Document{ID: id, Len: length}, Score: score}
}

func TestMoreRelevantPrefersHigherScore(t *testing.T) {
	if !MoreRelevant(hit("b", 100, 2), hit("a", 1, 1)) {
		t.Fatal("higher score should win regardless of Len and ID")
	}
}

func TestMoreRelevantBreaksScoreTieByShorterDocument(t *testing.T) {
	if !MoreRelevant(hit("b", 5, 1), hit("a", 10, 1)) {
		t.Fatal("shorter document should win on score tie")
	}
}

func TestMoreRelevantBreaksFullTieByID(t *testing.T) {
	if !MoreRelevant(hit("a", 5, 1), hit("b", 5, 1)) {
		t.Fatal("smaller ID should win on score and Len tie")
	}
}

func TestMoreRelevantIsStrictTotalOrder(t *testing.T) {
	hits := []types.Hit{hit("a", 5, 1), hit("b", 5, 1), hit("a", 9, 1), hit("c", 5, 2)}
	for i := range hits {
		if MoreRelevant(hits[i], hits[i]) {
			t.Fatalf("hit %d compares more relevant than itself", i)
		}
		for j := range hits {
			if i != j && MoreRelevant(hits[i], hits[j]) == MoreRelevant(hits[j], hits[i]) {
				t.Fatalf("hits %d and %d are not ordered", i, j)
			}
		}
	}
}
