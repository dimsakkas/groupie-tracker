package store

import (
	"groupie-tracker/internal/models"
	"slices"
	"sync"
	"testing"
)

func fakeMembers(n int) []string {
	names := make([]string, n)
	for i := range names {
		names[i] = "member"
	}
	return names
}

// filterFixture: 4 artists covering every filter case.
// Deliberately shuffled by ID, so the ordering is tested too.
func filterFixture() *Store {
	return newStore([]models.Artist{
		{ID: 3, Name: "C", CreationDate: 2007, FirstAlbum: "27-06-2013", Members: fakeMembers(1),
			Locations: []string{"north_carolina-usa", "texas-usa"}},
		{ID: 1, Name: "A", CreationDate: 1970, FirstAlbum: "14-12-1973", Members: fakeMembers(4),
			Locations: []string{"london-uk", "texas-usa"}},
		{ID: 4, Name: "D", CreationDate: 1980, FirstAlbum: "bad-date", Members: fakeMembers(6),
			Locations: []string{"paris-france"}},
		{ID: 2, Name: "B", CreationDate: 1994, FirstAlbum: "05-03-1996", Members: fakeMembers(5),
			Locations: []string{"seattle-usa", "washington-usa"}},
	})
}

func TestNewStore_ArtistsOrderedByID(t *testing.T) {
	s := filterFixture()
	var got []int
	for _, a := range s.Artists {
		got = append(got, a.ID)
	}
	if !slices.Equal(got, []int{1, 2, 3, 4}) {
		t.Fatalf("Artists order = %v, want [1 2 3 4]", got)
	}
	if s.ArtistsByID[2].Name != "B" {
		t.Errorf("ArtistsByID[2] = %+v", s.ArtistsByID[2])
	}
}

func TestFilter(t *testing.T) {
	s := filterFixture()
	tests := []struct {
		name string
		p    FilterParams
		want []int
	}{
		{"no filters returns everyone in ID order", FilterParams{}, []int{1, 2, 3, 4}},
		{"creation range", FilterParams{CreationMin: 1990, CreationMax: 2010}, []int{2, 3}},
		{"creation min only", FilterParams{CreationMin: 1990}, []int{2, 3}},
		{"creation max only", FilterParams{CreationMax: 1980}, []int{1, 4}},
		{"first album range", FilterParams{FirstAlbumMin: 1990, FirstAlbumMax: 2000}, []int{2}},
		{"first album min only", FilterParams{FirstAlbumMin: 2000}, []int{3}},
		{"members", FilterParams{Members: []int{4, 6}}, []int{1, 4}},
		{"location exact string", FilterParams{Locations: []string{"texas-usa"}}, []int{1, 3}},
		{"location by state name only", FilterParams{Locations: []string{"texas"}}, []int{1, 3}},
		{"location with underscore as space", FilterParams{Locations: []string{"north carolina"}}, []int{3}},
		{"location is case-insensitive", FilterParams{Locations: []string{"TEXAS"}}, []int{1, 3}},
		{"several locations are OR", FilterParams{Locations: []string{"london", "paris"}}, []int{1, 4}},
		{"empty location term is ignored", FilterParams{Locations: []string{"", "  "}}, []int{1, 2, 3, 4}},
		{"different filters are AND", FilterParams{CreationMin: 2000, Locations: []string{"texas"}}, []int{3}},
		{"nothing matches", FilterParams{Locations: []string{"atlantis"}}, []int{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := s.Filter(tc.p)
			if got == nil {
				t.Fatal("Filter returned nil, want a non-nil slice (JSON [] not null)")
			}
			if !slices.Equal(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestFilter_MalformedFirstAlbumNeverPanics(t *testing.T) {
	s := newStore([]models.Artist{
		{ID: 1, FirstAlbum: ""},
		{ID: 2, FirstAlbum: "2001"},
		{ID: 3, FirstAlbum: "not-a-date"},
		{ID: 4, FirstAlbum: "01-01-1999"},
	})
	// With a year bound, malformed values are excluded and nothing panics.
	got := s.Filter(FilterParams{FirstAlbumMin: 1990})
	if !slices.Equal(got, []int{4}) {
		t.Errorf("got %v, want [4]", got)
	}
	// Without a year bound, FirstAlbum is not even read.
	if got := s.Filter(FilterParams{}); len(got) != 4 {
		t.Errorf("got %v, want all 4 artists", got)
	}
}

func TestUniqueLocations_DedupedAndSorted(t *testing.T) {
	got := filterFixture().UniqueLocations()
	want := []string{"london-uk", "north_carolina-usa", "paris-france", "seattle-usa", "texas-usa", "washington-usa"}
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func geoFixture() *Store {
	return newStore([]models.Artist{{
		ID: 1,
		ConcertLocations: []models.ConcertLocation{
			{Location: "london-uk", Dates: []string{"10-06-2019"}},
			{Location: "paris-france", Dates: []string{"*03-04-2020", "01-02-2018"}}, // the earliest date is 2018
			{Location: "nowhere-xx", Dates: []string{"01-01-2017"}},                  // not geocoded
			{Location: "rome-italy", Dates: []string{"garbage"}},                     // unparseable date: sorts last
		},
	}, {ID: 2}})
}

func geoResults() map[string]models.GeoResult {
	return map[string]models.GeoResult{
		"london-uk":    {Location: "london-uk", Lat: 51.5, Lng: -0.12, Resolved: true},
		"paris-france": {Location: "paris-france", Lat: 48.8, Lng: 2.35, Resolved: true},
		"rome-italy":   {Location: "rome-italy", Lat: 41.9, Lng: 12.5, Resolved: true},
	}
}

func TestGeoFor_NotReadyYet(t *testing.T) {
	coords, ready, found := geoFixture().GeoFor(1)
	if ready || !found {
		t.Fatalf("ready=%v found=%v, want ready=false found=true", ready, found)
	}
	if coords == nil || len(coords) != 0 {
		t.Errorf("coords = %#v, want empty non-nil slice", coords)
	}
}

func TestGeoFor_UnknownArtist(t *testing.T) {
	_, _, found := geoFixture().GeoFor(999)
	if found {
		t.Fatal("unknown artist must report found=false")
	}
}

func TestGeoFor_ChronologicalAndSkipsUnresolved(t *testing.T) {
	s := geoFixture()
	s.SetGeo(geoResults())

	coords, ready, found := s.GeoFor(1)
	if !ready || !found {
		t.Fatalf("ready=%v found=%v, want both true", ready, found)
	}
	var got []string
	for _, c := range coords {
		got = append(got, c.Location)
	}
	want := []string{"paris-france", "london-uk", "rome-italy"}
	if !slices.Equal(got, want) {
		t.Errorf("order = %v, want %v", got, want)
	}
}

func TestGeoFor_ReadyButArtistWithoutConcerts(t *testing.T) {
	s := geoFixture()
	s.SetGeo(geoResults())
	coords, ready, found := s.GeoFor(2)
	if !ready || !found || coords == nil || len(coords) != 0 {
		t.Errorf("coords=%#v ready=%v found=%v, want empty non-nil, true, true", coords, ready, found)
	}
}

// Run this with -race: if the lock is missing, the race detector will complain.
func TestGeoFor_ConcurrentReadsWhileSetGeo(t *testing.T) {
	s := geoFixture()
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				coords, ready, _ := s.GeoFor(1)
				if ready && len(coords) != 3 {
					t.Errorf("ready but %d coords", len(coords))
					return
				}
			}
		}()
	}
	s.SetGeo(geoResults())
	wg.Wait()
}
