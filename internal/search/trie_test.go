package search

import (
	"groupie-tracker/internal/models"
	"testing"
)

func TestSearch_ExactMatchBeforeDescendants(t *testing.T) {
	trie := NewTrie()
	trie.Insert("Queen", models.SearchEntry{Value: "Queen", Type: "artist", ArtistID: 1})
	trie.Insert("Queensland", models.SearchEntry{Value: "Queensland", Type: "location", ArtistID: 2})

	results := trie.Search("queen", 10)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d: %+v", len(results), results)
	}
	if results[0].Value != "Queen" {
		t.Errorf("expected the exact-token match (%q) to come first, got %q", "Queen", results[0].Value)
	}
	if results[1].Value != "Queensland" {
		t.Errorf("expected the longer completion (%q) to come second, got %q", "Queensland", results[1].Value)
	}
}

func TestSearch_CaseInsensitive(t *testing.T) {
	trie := NewTrie()
	trie.Insert("Queen", models.SearchEntry{Value: "Queen", Type: "artist", ArtistID: 1})

	results := trie.Search("QUEEN", 10)
	if len(results) != 1 || results[0].Value != "Queen" {
		t.Fatalf("expected case-insensitive match, got %+v", results)
	}
}

func TestSearch_LimitIsRespected(t *testing.T) {
	trie := NewTrie()
	// 10 different artists that all happen to have "usa" as a location token.
	for i := 1; i <= 10; i++ {
		trie.Insert("usa", models.SearchEntry{Value: "usa", Type: "location", ArtistID: i})
	}

	results := trie.Search("usa", 3)
	if len(results) != 3 {
		t.Fatalf("expected exactly 3 results with limit=3, got %d: %+v", len(results), results)
	}
}

func TestSearch_NoMatchReturnsNil(t *testing.T) {
	trie := NewTrie()
	trie.Insert("Queen", models.SearchEntry{Value: "Queen", Type: "artist", ArtistID: 1})

	results := trie.Search("zzz", 10)
	if results != nil {
		t.Fatalf("expected nil for a prefix with no matches, got %+v", results)
	}
}

func TestSearch_DeterministicOrderAcrossRepeatedCalls(t *testing.T) {
	trie := NewTrie()
	trie.Insert("usa", models.SearchEntry{Value: "usa", Type: "location", ArtistID: 1})
	trie.Insert("uk", models.SearchEntry{Value: "uk", Type: "location", ArtistID: 2})
	trie.Insert("ukraine", models.SearchEntry{Value: "ukraine", Type: "location", ArtistID: 3})

	first := trie.Search("u", 10)
	for i := 0; i < 20; i++ {
		again := trie.Search("u", 10)
		if len(again) != len(first) {
			t.Fatalf("result length changed between calls: %d vs %d", len(first), len(again))
		}
		for j := range first {
			if first[j].Value != again[j].Value {
				t.Fatalf("result order is not deterministic: call 1 = %+v, call %d = %+v", first, i+2, again)
			}
		}
	}
}

func TestSearch_ContextFieldCarriesThrough(t *testing.T) {
	trie := NewTrie()
	trie.Insert("Brian May", models.SearchEntry{
		Value:    "Brian May",
		Type:     "member",
		Context:  "Member of Queen",
		ArtistID: 1,
	})

	results := trie.Search("brian", 10)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Context != "Member of Queen" {
		t.Errorf("expected Context %q, got %q", "Member of Queen", results[0].Context)
	}
}
