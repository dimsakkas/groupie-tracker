package store

import (
	"math"
	"reflect"
	"testing"

	"groupie-tracker/internal/models"
)

func TestHaversineKm(t *testing.T) {
	tests := []struct {
		name                   string
		lat1, lng1, lat2, lng2 float64
		want, tolerance        float64
	}{
		{"same point", 51.5, -0.12, 51.5, -0.12, 0, 0.001},
		{"london to paris", 51.5074, -0.1278, 48.8566, 2.3522, 344, 3},
		{"london to new york", 51.5074, -0.1278, 40.7128, -74.0060, 5570, 30},
		{"antipodes stay finite", 0, 0, 0, 180, 20015, 5},
	}
	for _, tt := range tests {
		got := haversineKm(tt.lat1, tt.lng1, tt.lat2, tt.lng2)
		if math.Abs(got-tt.want) > tt.tolerance {
			t.Errorf("%s: got %.1f km, want %.1f ± %.1f", tt.name, got, tt.want, tt.tolerance)
		}
	}
}

// insightsStore builds a store with a known geo state, without any network.
func insightsStore(ready bool) *Store {
	st := newStore([]models.Artist{
		{ID: 1, Name: "A", CreationDate: 1970, FirstAlbum: "14-12-1973", Members: []string{"x", "y"},
			ConcertLocations: []models.ConcertLocation{
				{Location: "paris-france", Dates: []string{"2020-01-01x", "02-01-2020"}},
				{Location: "london-uk", Dates: []string{"01-01-2019"}},
				{Location: "nowhere-land", Dates: []string{"01-01-2018"}},
			}},
		{ID: 2, Name: "B", CreationDate: 2001, FirstAlbum: "bad-date", Members: []string{"z"},
			ConcertLocations: []models.ConcertLocation{
				{Location: "london-uk", Dates: []string{"05-05-2021"}},
			}},
		{ID: 3, Name: "C", CreationDate: 1999, FirstAlbum: "01-01-1990", Members: []string{"p", "q"},
			ConcertLocations: []models.ConcertLocation{
				{Location: "london-uk", Dates: []string{"06-06-2022"}},
			}},
	})
	if ready {
		st.SetGeo(map[string]models.GeoResult{
			"london-uk":    {Location: "london-uk", Lat: 51.5074, Lng: -0.1278, Resolved: true},
			"paris-france": {Location: "paris-france", Lat: 48.8566, Lng: 2.3522, Resolved: true},
		})
	}
	return st
}

func TestTourDistanceKm(t *testing.T) {
	st := insightsStore(true)

	km, ready, found := st.TourDistanceKm(1)
	if !ready || !found || math.Abs(km-344) > 3 {
		t.Errorf("artist 1: km=%.1f ready=%v found=%v, want ~344 true true", km, ready, found)
	}

	// One geocoded location only: no distance to travel.
	km, ready, found = st.TourDistanceKm(2)
	if !ready || !found || km != 0 {
		t.Errorf("artist 2: km=%.1f ready=%v found=%v, want 0 true true", km, ready, found)
	}

	if _, _, found = st.TourDistanceKm(99); found {
		t.Error("unknown artist reported as found")
	}
}

func TestTourDistanceKm_NotReady(t *testing.T) {
	km, ready, found := insightsStore(false).TourDistanceKm(1)
	if ready || !found || km != 0 {
		t.Errorf("km=%.1f ready=%v found=%v, want 0 false true", km, ready, found)
	}
}

func TestWorldPoints(t *testing.T) {
	points, ready := insightsStore(true).WorldPoints()
	if !ready {
		t.Fatal("not ready")
	}
	// nowhere-land was never geocoded and must be skipped.
	if len(points) != 2 {
		t.Fatalf("got %d points, want 2: %+v", len(points), points)
	}
	if points[0].Location != "london-uk" || !reflect.DeepEqual(points[0].ArtistIDs, []int{1, 2, 3}) {
		t.Errorf("london point wrong: %+v", points[0])
	}
	if points[1].Location != "paris-france" || !reflect.DeepEqual(points[1].ArtistIDs, []int{1}) {
		t.Errorf("paris point wrong: %+v", points[1])
	}
}

func TestWorldPoints_NotReadyIsEmptyNotNil(t *testing.T) {
	points, ready := insightsStore(false).WorldPoints()
	if ready || points == nil || len(points) != 0 {
		t.Errorf("points=%v (nil=%v) ready=%v, want empty non-nil and not ready", points, points == nil, ready)
	}
}

func TestYearBounds(t *testing.T) {
	cMin, cMax, aMin, aMax := insightsStore(false).YearBounds()
	if cMin != 1970 || cMax != 2001 {
		t.Errorf("creation bounds = %d-%d, want 1970-2001", cMin, cMax)
	}
	// artist 2 has an unparseable first album and must be ignored.
	if aMin != 1973 || aMax != 1990 {
		t.Errorf("album bounds = %d-%d, want 1973-1990", aMin, aMax)
	}
}

func TestYearBounds_Empty(t *testing.T) {
	a, b, c, d := newStore(nil).YearBounds()
	if a != 0 || b != 0 || c != 0 || d != 0 {
		t.Errorf("empty store bounds = %d %d %d %d, want zeros", a, b, c, d)
	}
}

func TestMemberCounts(t *testing.T) {
	got := insightsStore(false).MemberCounts()
	if !reflect.DeepEqual(got, []int{1, 2}) {
		t.Errorf("got %v, want [1 2]", got)
	}
}

func TestPathLengthKm(t *testing.T) {
	if got := PathLengthKm(nil); got != 0 {
		t.Errorf("empty path = %v, want 0", got)
	}
	one := []models.GeoResult{{Lat: 1, Lng: 1}}
	if got := PathLengthKm(one); got != 0 {
		t.Errorf("single point = %v, want 0", got)
	}
	path := []models.GeoResult{{Lat: 51.5074, Lng: -0.1278}, {Lat: 48.8566, Lng: 2.3522}, {Lat: 51.5074, Lng: -0.1278}}
	if got := PathLengthKm(path); math.Abs(got-688) > 6 {
		t.Errorf("there and back = %.1f, want ~688", got)
	}
}

func TestChronologicalConcerts(t *testing.T) {
	in := []models.ConcertLocation{
		{Location: "b-land", Dates: []string{"*05-05-2021", "01-01-2020"}},
		{Location: "a-land", Dates: []string{"01-06-2020"}},
		{Location: "c-land", Dates: []string{"not-a-date"}},
	}
	got := ChronologicalConcerts(in)

	wantOrder := []string{"b-land", "a-land", "c-land"}
	for i, want := range wantOrder {
		if got[i].Location != want {
			t.Fatalf("order = %v, want %v", got, wantOrder)
		}
	}
	if !reflect.DeepEqual(got[0].Dates, []string{"01-01-2020", "05-05-2021"}) {
		t.Errorf("dates not cleaned and sorted: %v", got[0].Dates)
	}
	if in[0].Dates[0] != "*05-05-2021" || in[0].Location != "b-land" {
		t.Error("input was modified")
	}
}
