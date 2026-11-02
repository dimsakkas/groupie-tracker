package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"groupie-tracker/internal/fetch"
	"groupie-tracker/internal/models"
	"groupie-tracker/internal/search"
	"groupie-tracker/internal/store"
)

// pagesFixture builds a real Server and a mux with every page route, using
// the real templates from web/templates. withGeo decides whether the
// background geocoding is "finished".
func pagesFixture(t *testing.T, withGeo bool) (*Server, http.Handler) {
	t.Helper()
	useRealTemplates(t)

	raw := fetch.RawData{
		Artists: []models.RawArtist{
			{ID: 1, Name: "Queen", Image: "http://img/q.jpg", Members: []string{"Freddie Mercury", "Brian May"},
				CreationDate: 1970, FirstAlbum: "14-12-1973"},
			{ID: 2, Name: `<b>Evil</b>`, Image: "http://img/e.jpg", Members: []string{"Solo"},
				CreationDate: 2007, FirstAlbum: "27-06-2013"},
		},
		Locations: []models.RawLocationResp{
			{ID: 1, Locations: []string{"london-uk", "north_carolina-usa"}},
			{ID: 2, Locations: []string{"london-uk"}},
		},
		Relations: []models.RawRelationsResp{
			{ID: 1, DatesLocations: map[string][]string{
				"london-uk":          {"10-06-2019", "*01-01-2020"},
				"north_carolina-usa": {"01-02-2018"},
			}},
			{ID: 2, DatesLocations: map[string][]string{"london-uk": {"05-05-2021"}}},
		},
	}
	st := store.Build(raw)
	if withGeo {
		st.SetGeo(map[string]models.GeoResult{
			"london-uk":          {Location: "london-uk", Lat: 51.5074, Lng: -0.1278, Resolved: true},
			"north_carolina-usa": {Location: "north_carolina-usa", Lat: 35.67, Lng: -79.04, Resolved: true},
		})
	}
	srv := &Server{Store: st, Trie: search.BuildIndex(st.Artists)}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", srv.HomeHandler)
	mux.HandleFunc("GET /artist/{id}", srv.ArtistHandler)
	mux.HandleFunc("GET /search", srv.SearchHandler)
	mux.HandleFunc("GET /compare", srv.CompareHandler)
	mux.HandleFunc("GET /api/geo", srv.WorldGeoHandler)
	mux.HandleFunc("GET /api/suggest", srv.SuggestHandler)
	mux.HandleFunc("GET /api/artist/{id}/geo", srv.GeoHandler)
	mux.HandleFunc("/", NotFound)
	return srv, mux
}

func mustContain(t *testing.T, rec *httptest.ResponseRecorder, wants ...string) {
	t.Helper()
	body := rec.Body.String()
	for _, want := range wants {
		if !strings.Contains(body, want) {
			t.Errorf("page is missing %q\n%s", want, body)
		}
	}
}

func TestPrettyLocation(t *testing.T) {
	tests := map[string]string{
		"los_angeles-usa":                 "Los Angeles, USA",
		"north_carolina-usa":              "North Carolina, USA",
		"london-uk":                       "London, UK",
		"dunedin-new_zealand":             "Dunedin, New Zealand",
		"willemstad-netherlands_antilles": "Willemstad, Netherlands Antilles",
		"singapore":                       "Singapore",
		"":                                "",
		"zürich-switzerland":              "Zürich, Switzerland",
		"playa_del_carmen-mexico":         "Playa Del Carmen, Mexico",
	}
	for raw, want := range tests {
		if got := prettyLocation(raw); got != want {
			t.Errorf("prettyLocation(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestLocationGroups(t *testing.T) {
	states := map[string]string{
		"texas-usa":   "texas",
		"georgia-usa": "georgia",
	}
	groups := locationGroups([]string{"london-uk", "texas-usa", "georgia-usa", "singapore"}, states)

	if len(groups) != 3 {
		t.Fatalf("got %d groups, want 3: %+v", len(groups), groups)
	}
	if groups[0].Name != "Singapore" || groups[1].Name != "UK" || groups[2].Name != "USA" {
		t.Errorf("groups not sorted by name: %+v", groups)
	}

	usa := groups[2]
	if len(usa.States) != 2 {
		t.Fatalf("USA should have 2 states (Georgia, Texas), got %d: %+v", len(usa.States), usa.States)
	}
	if usa.States[0].Name != "Georgia" || usa.States[1].Name != "Texas" {
		t.Errorf("USA states not sorted by name: %+v", usa.States)
	}
	if len(usa.States[0].Cities) != 1 || usa.States[0].Cities[0].Label != "Georgia" || usa.States[0].Cities[0].Value != "georgia-usa" {
		t.Errorf("Georgia state's city wrong: %+v", usa.States[0])
	}

	singapore := groups[0]
	if len(singapore.States) != 1 || singapore.States[0].Name != "" {
		t.Errorf("Singapore (no geocoded state) should have one unnamed state group: %+v", singapore.States)
	}
	if len(singapore.States[0].Cities) != 1 || singapore.States[0].Cities[0].Label != "Singapore" {
		t.Errorf("Singapore city wrong: %+v", singapore.States[0].Cities)
	}
}

func TestArtistPage(t *testing.T) {
	_, mux := pagesFixture(t, true)
	rec := get(mux, "/artist/1")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	mustContain(t, rec,
		"<h1>Queen</h1>", "Freddie Mercury", "Brian May", "Members (2)", "14-12-1973", "1970",
		"3 shows in 2 locations", `data-artist-id="1"`, "North Carolina, USA", "London, UK",
		`<option value="2">`, // compare form lists the other artist
	)
	if strings.Contains(rec.Body.String(), `<option value="1">`) {
		t.Error("compare form offers the artist itself")
	}
	if strings.Contains(rec.Body.String(), "*01-01-2020") {
		t.Error("the * prefix of the date was not stripped")
	}
}

func TestArtistPage_ConcertsAreChronological(t *testing.T) {
	_, mux := pagesFixture(t, true)
	body := get(mux, "/artist/1").Body.String()

	// North Carolina (2018) must come before London (2019).
	if nc, ld := strings.Index(body, "North Carolina, USA</h3>"), strings.Index(body, "London, UK</h3>"); nc < 0 || ld < 0 || nc > ld {
		t.Errorf("concerts not in chronological order (nc=%d, london=%d)", nc, ld)
	}
}

func TestArtistPage_EscapesName(t *testing.T) {
	_, mux := pagesFixture(t, true)
	body := get(mux, "/artist/2").Body.String()

	if strings.Contains(body, "<b>Evil</b>") {
		t.Error("artist name was not HTML-escaped")
	}
}

func TestArtistPage_BadAndUnknownIDs(t *testing.T) {
	_, mux := pagesFixture(t, true)

	for target, want := range map[string]int{
		"/artist/abc":  http.StatusBadRequest,
		"/artist/9999": http.StatusNotFound,
		"/artist/-1":   http.StatusNotFound,
		"/artist/1.5":  http.StatusBadRequest,
	} {
		if rec := get(mux, target); rec.Code != want {
			t.Errorf("GET %s = %d, want %d", target, rec.Code, want)
		}
	}
}

func TestSearchPage(t *testing.T) {
	_, mux := pagesFixture(t, true)

	t.Run("no query shows the hint", func(t *testing.T) {
		rec := get(mux, "/search")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d", rec.Code)
		}
		mustContain(t, rec, "Type an artist")
	})

	t.Run("multi-word location query finds both artists", func(t *testing.T) {
		rec := get(mux, "/search?q=london-uk")
		mustContain(t, rec, "2 results", "London, UK", "Artist: Queen", `href="/artist/1"`, `href="/artist/2"`)
	})

	t.Run("member by first and partial last name", func(t *testing.T) {
		rec := get(mux, "/search?q=freddie+mer")
		mustContain(t, rec, "Freddie Mercury", "Member of Queen", `href="/artist/1"`)
	})

	t.Run("date query", func(t *testing.T) {
		rec := get(mux, "/search?q=14-12-1973")
		mustContain(t, rec, "14-12-1973", "First album", "Artist: Queen")
	})

	t.Run("no match", func(t *testing.T) {
		mustContain(t, get(mux, "/search?q=zzzz"), "Nothing matches")
	})

	t.Run("query is escaped", func(t *testing.T) {
		body := get(mux, "/search?q=%3Cscript%3E").Body.String()
		if strings.Contains(body, "<script>") && !strings.Contains(body, "&lt;script&gt;") {
			t.Error("query was echoed without escaping")
		}
		if strings.Contains(body, "“<script>”") {
			t.Error("raw <script> in the page")
		}
	})
}

func TestComparePage(t *testing.T) {
	t.Run("chooser without parameters", func(t *testing.T) {
		_, mux := pagesFixture(t, true)
		rec := get(mux, "/compare")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d", rec.Code)
		}
		mustContain(t, rec, "Compare artists", `name="a"`, `name="b"`)
		if strings.Contains(rec.Body.String(), "<table") {
			t.Error("table shown before two artists were chosen")
		}
	})

	t.Run("one artist preselected", func(t *testing.T) {
		_, mux := pagesFixture(t, true)
		rec := get(mux, "/compare?a=1")
		mustContain(t, rec, `<option value="1" selected>`)
		if strings.Contains(rec.Body.String(), "<table") {
			t.Error("table shown with only one artist")
		}
	})

	t.Run("both artists with distance", func(t *testing.T) {
		_, mux := pagesFixture(t, true)
		rec := get(mux, "/compare?a=1&b=2")
		mustContain(t, rec, "<table", "Queen", "Freddie Mercury, Brian May", "3", "Tour distance", " km</td>")
		if strings.Contains(rec.Body.String(), "Still calculating") {
			t.Error("distance shown as pending although geocoding is done")
		}
	})

	t.Run("distance pending while geocoding runs", func(t *testing.T) {
		_, mux := pagesFixture(t, false)
		mustContain(t, get(mux, "/compare?a=1&b=2"), "Still calculating")
	})

	t.Run("bad and unknown ids", func(t *testing.T) {
		_, mux := pagesFixture(t, true)
		for target, want := range map[string]int{
			"/compare?a=abc&b=1":  http.StatusBadRequest,
			"/compare?a=1&b=xyz":  http.StatusBadRequest,
			"/compare?a=1&b=9999": http.StatusNotFound,
			"/compare?a=9999&b=1": http.StatusNotFound,
		} {
			if rec := get(mux, target); rec.Code != want {
				t.Errorf("GET %s = %d, want %d", target, rec.Code, want)
			}
		}
	})
}

func TestHome_FilterPanelUsesTheData(t *testing.T) {
	_, mux := pagesFixture(t, true)
	rec := get(mux, "/")

	mustContain(t, rec,
		`min="1970" max="2007"`, // creation slider bounds
		`min="1973" max="2013"`, // first album slider bounds
		`name="members" value="1"`, `name="members" value="2"`,
		`value="london-uk"`, `value="north_carolina-usa"`, // raw values go to the API
		"North Carolina", ">USA<",
		`data-name="Queen"`, `class="fav"`,
	)
}

func TestWorldGeoAndDistanceAPIs(t *testing.T) {
	t.Run("ready", func(t *testing.T) {
		_, mux := pagesFixture(t, true)
		rec := get(mux, "/api/geo")
		mustBeJSON(t, rec)
		mustContain(t, rec, `"ready":true`, `"location":"london-uk"`, `"artist_ids":[1,2]`, `"lat":51.5074`)

		geo := get(mux, "/api/artist/1/geo")
		mustContain(t, geo, `"distance_km":`)
		if strings.Contains(geo.Body.String(), `"distance_km":0`) {
			t.Errorf("two geocoded locations but distance 0: %s", geo.Body.String())
		}
	})

	t.Run("not ready is an empty list, not null", func(t *testing.T) {
		_, mux := pagesFixture(t, false)
		mustContain(t, get(mux, "/api/geo"), `"ready":false`, `"locations":[]`)
		mustContain(t, get(mux, "/api/artist/1/geo"), `"distance_km":0`)
	})
}

func TestSuggest_MultiWordAndDeterministic(t *testing.T) {
	_, mux := pagesFixture(t, true)

	t.Run("two words", func(t *testing.T) {
		mustContain(t, get(mux, "/api/suggest?q=freddie+mer"), `"value":"Freddie Mercury"`)
	})
	t.Run("every word must match, not only the first", func(t *testing.T) {
		for _, q := range []string{"freddie+zzz", "london-xx", "brian+mercury"} {
			if body := get(mux, "/api/suggest?q="+q).Body.String(); strings.TrimSpace(body) != "[]" {
				t.Errorf("q=%s: got %s, want []", q, body)
			}
		}
	})
	t.Run("location with a dash", func(t *testing.T) {
		mustContain(t, get(mux, "/api/suggest?q=london-uk"), `"value":"london-uk"`)
	})
	t.Run("a location shared by two artists is suggested once", func(t *testing.T) {
		body := get(mux, "/api/suggest?q=london").Body.String()
		if n := strings.Count(body, `"value":"london-uk"`); n != 1 {
			t.Errorf("london-uk appears %d times, want 1: %s", n, body)
		}
	})
	t.Run("same answer every time", func(t *testing.T) {
		first := get(mux, "/api/suggest?q=l").Body.String()
		for i := 0; i < 30; i++ {
			if got := get(mux, "/api/suggest?q=l").Body.String(); got != first {
				t.Fatalf("answer changed between requests:\n%s\n%s", first, got)
			}
		}
	})
	t.Run("only separators", func(t *testing.T) {
		if body := get(mux, "/api/suggest?q=---").Body.String(); strings.TrimSpace(body) != "[]" {
			t.Errorf("got %q, want []", body)
		}
	})
}

func TestTitleCase_EmptyAndUnicode(t *testing.T) {
	if got := titleCase(""); got != "" {
		t.Errorf("titleCase(\"\") = %q", got)
	}
	if got := titleCase("élan vital"); got != "Élan Vital" {
		t.Errorf("titleCase unicode = %q", got)
	}
}

func TestFormatKm(t *testing.T) {
	tests := map[int]string{0: "0 km", 7: "7 km", 999: "999 km", 1000: "1,000 km", 12313: "12,313 km", 1234567: "1,234,567 km"}
	for in, want := range tests {
		if got := formatKm(in); got != want {
			t.Errorf("formatKm(%d) = %q, want %q", in, got, want)
		}
	}
}
