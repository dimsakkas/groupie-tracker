package search

import (
	"groupie-tracker/internal/models"
	"testing"
)

func testArtists() []models.Artist {
	return []models.Artist{
		{
			ID:           1,
			Name:         "Queen",
			Image:        "https://example.com/queen.jpg",
			Members:      []string{"Freddie Mercury", "Brian May"},
			CreationDate: 1970,
			FirstAlbum:   "13-07-1973",
			Locations:    []string{"london-uk", "wembley-uk"},
		},
		{
			ID:           2,
			Name:         "Travis Scott",
			Image:        "https://example.com/travisscott.jpg",
			Members:      []string{"Travis Scott"},
			CreationDate: 2007,
			FirstAlbum:   "27-06-2013",
			Locations:    []string{"north_carolina-usa", "georgia-usa"},
		},
	}
}

func TestBuildIndex_ArtistNameSearchable(t *testing.T) {
	trie := BuildIndex(testArtists())

	results := trie.Search("queen", 10)
	if len(results) != 1 || results[0].Value != "Queen" || results[0].Type != "artist" {
		t.Fatalf("expected to find Queen as an artist entry, got %+v", results)
	}
}

func TestBuildIndex_MemberHasContext(t *testing.T) {
	trie := BuildIndex(testArtists())

	results := trie.Search("brian", 10)
	if len(results) != 1 {
		t.Fatalf("expected 1 result for 'brian', got %d: %+v", len(results), results)
	}
	if results[0].Type != "member" {
		t.Errorf("expected Type %q, got %q", "member", results[0].Type)
	}
	if results[0].Context != "Member of Queen" {
		t.Errorf("expected Context %q, got %q", "Member of Queen", results[0].Context)
	}
}

func TestBuildIndex_LocationReachableFromEitherToken(t *testing.T) {
	trie := BuildIndex(testArtists())

	// "north_carolina-usa" must be reachable from BOTH of its tokens.
	byNorth := trie.Search("north", 10)
	byUsa := trie.Search("usa", 10)

	if len(byNorth) != 1 || byNorth[0].Value != "north_carolina-usa" {
		t.Fatalf("expected to find the location via 'north', got %+v", byNorth)
	}

	foundViaUsa := false
	for _, r := range byUsa {
		if r.Value == "north_carolina-usa" {
			foundViaUsa = true
		}
	}
	if !foundViaUsa {
		t.Errorf("expected to find 'north_carolina-usa' via the 'usa' token too, got %+v", byUsa)
	}
}

func TestBuildIndex_CreationDateAndFirstAlbumSearchable(t *testing.T) {
	trie := BuildIndex(testArtists())

	byYear := trie.Search("1970", 10)
	if len(byYear) != 1 || byYear[0].Type != "creation_date" || byYear[0].ArtistID != 1 {
		t.Fatalf("expected creation_date match for Queen, got %+v", byYear)
	}

	// FirstAlbum goes through InsertMultiToken just like everything else,
	// so it's split on '-' into ["13", "07", "1973"] -- searching the whole
	// dashed string wouldn't match (there's no '-' edge in the trie at all).
	// Searching the year token is both what actually works AND the more
	// realistic query a person would type.
	byAlbum := trie.Search("1973", 10)
	if len(byAlbum) != 1 || byAlbum[0].Type != "first_album_date" {
		t.Fatalf("expected first_album_date match, got %+v", byAlbum)
	}
}

func TestBuildIndex_ImageIsNeverIndexed(t *testing.T) {
	trie := BuildIndex(testArtists())

	// The image URLs contain "example" -- if Image were ever indexed by
	// mistake, this search would return something.
	results := trie.Search("example", 10)
	if results != nil {
		t.Fatalf("expected Image URLs to never be searchable, got %+v", results)
	}
}

func TestBuildIndex_BroadSearchHasNoDuplicates(t *testing.T) {
	trie := BuildIndex(testArtists())

	// "usa" is one of two tokens for the same location entry (georgia-usa
	// and north_carolina-usa both end in "-usa"), so this also exercises
	// the dedup path across two DIFFERENT underlying entries, not just one.
	results := trie.Search("usa", 100)
	seenValues := make(map[string]int)
	for _, r := range results {
		seenValues[r.Value]++
	}
	for value, count := range seenValues {
		if count > 1 {
			t.Errorf("value %q appeared %d times, expected at most once", value, count)
		}
	}
}
