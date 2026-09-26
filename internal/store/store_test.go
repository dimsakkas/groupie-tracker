package store

import (
	"groupie-tracker/internal/fetch"
	"groupie-tracker/internal/models"
	"testing"
)

// findArtist is a small test helper: linear search by ID over the slice
// BuildArtists returns, since BuildArtists itself only returns a []Artist,
// not a lookup map (that's what Store.ArtistsByID is for, tested separately
// in TestStoreBuild_ArtistsByID_Lookup below).
func findArtist(artists []models.Artist, id int) (models.Artist, bool) {
	for _, a := range artists {
		if a.ID == id {
			return a, true
		}
	}
	return models.Artist{}, false
}

// testRawData is one shared fixture reused across every test in this file.
// It is built directly as Go structs (not JSON) because by the time raw
// data reaches store.BuildArtists, fetch.FetchAll has already decoded it —
// so we test at the boundary this package actually owns: RawData in,
// []models.Artist out.
//
// Artist 1 (Queen)          -> exercises the "members" join
// Artist 2 (Gorillaz)       -> exercises firstAlbum passthrough
// Artist 3 (Foo Fighters)   -> exercises a second, different-sized members join
// Artist 4 (Travis Scott)   -> exercises the relations -> ConcertLocations join
//
//	with exactly 10 concert locations
//
// Artist 5 (Ghost Artist)   -> deliberately has NO entry in Locations, Dates,
//
//	or Relations, to prove BuildArtists degrades
//	gracefully (nil fields) instead of panicking
var testRawData = fetch.RawData{
	Artists: []models.RawArtist{
		{
			ID:   1,
			Name: "Queen",
			Members: []string{
				"Freddie Mercury", "Brian May", "Roger Taylor", "John Deacon",
				"Spike Edney", "Neil Fairclough", "Rufus Tiger Taylor",
			},
			CreationDate: 1970,
			FirstAlbum:   "13-07-1973",
			Image:        "queen.jpg",
		},
		{
			ID:           2,
			Name:         "Gorillaz",
			Members:      []string{"2D", "Murdoc Niccals", "Noodle", "Russel Hobbs"},
			CreationDate: 1998,
			FirstAlbum:   "26-03-2001",
			Image:        "gorillaz.jpg",
		},
		{
			ID:   3,
			Name: "Foo Fighters",
			Members: []string{
				"Dave Grohl", "Nate Mendel", "Pat Smear",
				"Chris Shiflett", "Rami Jaffee", "Josh Freese",
			},
			CreationDate: 1994,
			FirstAlbum:   "04-07-1995",
			Image:        "foofighters.jpg",
		},
		{
			ID:           4,
			Name:         "Travis Scott",
			Members:      []string{"Travis Scott"},
			CreationDate: 2007,
			FirstAlbum:   "27-06-2013",
			Image:        "travisscott.jpg",
		},
		{
			ID:           5,
			Name:         "Ghost Artist",
			Members:      []string{"Nobody"},
			CreationDate: 2020,
			FirstAlbum:   "01-01-2020",
			Image:        "ghost.jpg",
		},
	},
	Locations: []models.RawLocationResp{
		{ID: 1, Locations: []string{"london-uk", "wembley-uk"}},
		{ID: 2, Locations: []string{"gorillaz_hq-uk"}},
		{ID: 3, Locations: []string{"seattle-usa"}},
		{ID: 4, Locations: []string{
			"houston-usa", "new_york-usa", "los_angeles-usa", "chicago-usa",
			"miami-usa", "atlanta-usa", "dallas-usa", "boston-usa",
			"denver-usa", "las_vegas-usa",
		}},
		// deliberately no entry for ID 5
	},
	Dates: []models.RawDatesResp{
		{ID: 1, Dates: []string{"*01-01-2020"}},
		{ID: 2, Dates: []string{"*02-02-2020"}},
		{ID: 3, Dates: []string{"*03-03-2020"}},
		{ID: 4, Dates: []string{"*04-04-2020"}},
		// deliberately no entry for ID 5
	},
	Relations: []models.RawRelationsResp{
		{ID: 1, DatesLocations: map[string][]string{
			"london-uk": {"01-01-2020"},
		}},
		{ID: 2, DatesLocations: map[string][]string{
			"gorillaz_hq-uk": {"02-02-2020"},
		}},
		{ID: 3, DatesLocations: map[string][]string{
			"seattle-usa": {"03-03-2020"},
		}},
		{ID: 4, DatesLocations: map[string][]string{
			"houston-usa":     {"10-04-2023"},
			"new_york-usa":    {"11-04-2023"},
			"los_angeles-usa": {"12-04-2023"},
			"chicago-usa":     {"13-04-2023"},
			"miami-usa":       {"14-04-2023"},
			"atlanta-usa":     {"15-04-2023"},
			"dallas-usa":      {"16-04-2023"},
			"boston-usa":      {"17-04-2023"},
			"denver-usa":      {"18-04-2023"},
			"las_vegas-usa":   {"19-04-2023"},
		}},
		// deliberately no entry for ID 5
	},
}

func TestBuildArtists_QueenMembers(t *testing.T) {
	artists := BuildArtists(testRawData)

	queen, ok := findArtist(artists, 1)
	if !ok {
		t.Fatal("Queen (ID 1) not found in BuildArtists result")
	}

	if len(queen.Members) != 7 {
		t.Fatalf("expected Queen to have 7 members, got %d: %+v", len(queen.Members), queen.Members)
	}

	want := map[string]bool{
		"Freddie Mercury": true, "Brian May": true, "Roger Taylor": true,
		"John Deacon": true, "Spike Edney": true, "Neil Fairclough": true,
		"Rufus Tiger Taylor": true,
	}
	for _, m := range queen.Members {
		if !want[m] {
			t.Errorf("unexpected member in Queen's list: %q", m)
		}
	}
}

func TestBuildArtists_GorillazFirstAlbum(t *testing.T) {
	artists := BuildArtists(testRawData)

	gorillaz, ok := findArtist(artists, 2)
	if !ok {
		t.Fatal("Gorillaz (ID 2) not found in BuildArtists result")
	}

	if gorillaz.FirstAlbum != "26-03-2001" {
		t.Fatalf("expected Gorillaz firstAlbum to be %q, got %q", "26-03-2001", gorillaz.FirstAlbum)
	}
}

func TestBuildArtists_FooFightersMembers(t *testing.T) {
	artists := BuildArtists(testRawData)

	ff, ok := findArtist(artists, 3)
	if !ok {
		t.Fatal("Foo Fighters (ID 3) not found in BuildArtists result")
	}

	if len(ff.Members) != 6 {
		t.Fatalf("expected Foo Fighters to have 6 members, got %d: %+v", len(ff.Members), ff.Members)
	}
}

func TestBuildArtists_TravisScottConcertLocations_ExactSet(t *testing.T) {
	artists := BuildArtists(testRawData)

	ts, ok := findArtist(artists, 4)
	if !ok {
		t.Fatal("Travis Scott (ID 4) not found in BuildArtists result")
	}

	want := map[string]bool{
		"houston-usa": true, "new_york-usa": true, "los_angeles-usa": true,
		"chicago-usa": true, "miami-usa": true, "atlanta-usa": true,
		"dallas-usa": true, "boston-usa": true, "denver-usa": true,
		"las_vegas-usa": true,
	}

	if len(ts.ConcertLocations) != len(want) {
		t.Fatalf("expected exactly %d concert locations for Travis Scott, got %d: %+v",
			len(want), len(ts.ConcertLocations), ts.ConcertLocations)
	}

	// Map iteration order is non-deterministic in Go, so we can't compare
	// slices directly -- we compare the *set* of locations instead.
	got := make(map[string]bool, len(ts.ConcertLocations))
	for _, c := range ts.ConcertLocations {
		got[c.Location] = true
	}

	for loc := range want {
		if !got[loc] {
			t.Errorf("expected concert location %q missing from Travis Scott's result", loc)
		}
	}
	for loc := range got {
		if !want[loc] {
			t.Errorf("unexpected concert location %q found in Travis Scott's result", loc)
		}
	}
}

func TestBuildArtists_RawLocationsAndDatesFieldsPopulated(t *testing.T) {
	artists := BuildArtists(testRawData)

	queen, ok := findArtist(artists, 1)
	if !ok {
		t.Fatal("Queen (ID 1) not found in BuildArtists result")
	}

	wantLocations := []string{"london-uk", "wembley-uk"}
	if len(queen.Locations) != len(wantLocations) {
		t.Fatalf("expected %d raw locations, got %d: %+v", len(wantLocations), len(queen.Locations), queen.Locations)
	}
	for i, loc := range wantLocations {
		if queen.Locations[i] != loc {
			t.Errorf("Locations[%d]: expected %q, got %q", i, loc, queen.Locations[i])
		}
	}

	wantDates := []string{"*01-01-2020"}
	if len(queen.Dates) != len(wantDates) {
		t.Fatalf("expected %d raw dates, got %d: %+v", len(wantDates), len(queen.Dates), queen.Dates)
	}
	for i, d := range wantDates {
		if queen.Dates[i] != d {
			t.Errorf("Dates[%d]: expected %q, got %q", i, d, queen.Dates[i])
		}
	}
}

func TestBuildArtists_MissingLocationsAndDates_DoesNotPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("BuildArtists panicked on an artist with no locations/dates/relations entry: %v", r)
		}
	}()

	artists := BuildArtists(testRawData)

	ghost, ok := findArtist(artists, 5)
	if !ok {
		t.Fatal("Ghost Artist (ID 5) not found in BuildArtists result")
	}

	if ghost.Locations != nil {
		t.Errorf("expected nil Locations for an artist with no locations entry, got %+v", ghost.Locations)
	}
	if ghost.Dates != nil {
		t.Errorf("expected nil Dates for an artist with no dates entry, got %+v", ghost.Dates)
	}
	if len(ghost.ConcertLocations) != 0 {
		t.Errorf("expected 0 ConcertLocations for an artist with no relations entry, got %+v", ghost.ConcertLocations)
	}
}

func TestStoreBuild_ArtistsByID_Lookup(t *testing.T) {
	s := Build(testRawData)

	if len(s.ArtistsByID) != len(testRawData.Artists) {
		t.Fatalf("expected %d artists in ArtistsByID, got %d", len(testRawData.Artists), len(s.ArtistsByID))
	}

	queen, ok := s.ArtistsByID[1]
	if !ok {
		t.Fatal("expected ArtistsByID[1] to exist")
	}
	if queen.Name != "Queen" {
		t.Errorf("expected ArtistsByID[1].Name to be %q, got %q", "Queen", queen.Name)
	}

	travisScott, ok := s.ArtistsByID[4]
	if !ok {
		t.Fatal("expected ArtistsByID[4] to exist")
	}
	if travisScott.Name != "Travis Scott" {
		t.Errorf("expected ArtistsByID[4].Name to be %q, got %q", "Travis Scott", travisScott.Name)
	}

	if _, ok := s.ArtistsByID[999]; ok {
		t.Error("expected ArtistsByID[999] to not exist")
	}
}
