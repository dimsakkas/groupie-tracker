package handlers

import (
	"encoding/json"
	"groupie-tracker/internal/fetch"
	"groupie-tracker/internal/models"
	"groupie-tracker/internal/search"
	"groupie-tracker/internal/store"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
)

// apiFixture builds a small real Server (store + trie) and a mux with the
// three API routes, so the route patterns are exercised as well.
func apiFixture() (*Server, http.Handler) {
	raw := fetch.RawData{
		Artists: []models.RawArtist{
			{ID: 1, Name: "Queen", Members: []string{"Freddie Mercury", "Brian May", "John Deacon", "Roger Taylor"},
				CreationDate: 1970, FirstAlbum: "14-12-1973"},
			{ID: 2, Name: "Travis Scott", Members: []string{"Travis Scott"},
				CreationDate: 2007, FirstAlbum: "27-06-2013"},
			{ID: 3, Name: "Logic", Members: []string{"Logic"},
				CreationDate: 2009, FirstAlbum: "01-01-2014"},
		},
		Locations: []models.RawLocationResp{
			{ID: 1, Locations: []string{"london-uk", "texas-usa"}},
			{ID: 2, Locations: []string{"texas-usa", "queensland-australia"}},
			{ID: 3, Locations: []string{"washington-usa"}},
		},
		Relations: []models.RawRelationsResp{
			{ID: 1, DatesLocations: map[string][]string{"london-uk": {"10-06-2019"}, "texas-usa": {"01-02-2018"}}},
			{ID: 2, DatesLocations: map[string][]string{"texas-usa": {"05-05-2021"}, "queensland-australia": {"01-01-2020"}}},
			{ID: 3, DatesLocations: map[string][]string{"washington-usa": {"03-03-2022"}}},
		},
	}
	st := store.Build(raw)
	srv := &Server{Store: st, Trie: search.BuildIndex(st.Artists)}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/suggest", srv.SuggestHandler)
	mux.HandleFunc("GET /api/filter", srv.FilterHandler)
	mux.HandleFunc("GET /api/artist/{id}/geo", srv.GeoHandler)
	return srv, mux
}

func get(h http.Handler, target string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
	return rec
}

func mustBeJSON(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	if !json.Valid(rec.Body.Bytes()) {
		t.Errorf("body is not valid JSON: %q", rec.Body.String())
	}
}

func TestSuggest_CrossTypeMatchAndLowercaseKeys(t *testing.T) {
	_, h := apiFixture()
	rec := get(h, "/api/suggest?q=quee")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	mustBeJSON(t, rec)

	var got []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	var sawArtist, sawLocation bool
	for _, e := range got {
		for _, key := range []string{"value", "type", "context", "artist_id"} {
			if _, ok := e[key]; !ok {
				t.Errorf("entry %v is missing key %q", e, key)
			}
		}
		if e["value"] == "Queen" && e["type"] == "artist" {
			sawArtist = true
		}
		if e["value"] == "queensland-australia" && e["type"] == "location" {
			sawLocation = true
		}
	}
	if !sawArtist || !sawLocation {
		t.Errorf("want both the artist Queen and the location queensland-australia, got %v", got)
	}
}

func TestSuggest_EmptyOrUnknownQueryGivesEmptyArrayNotNull(t *testing.T) {
	_, h := apiFixture()
	for _, target := range []string{"/api/suggest", "/api/suggest?q=", "/api/suggest?q=%20%20", "/api/suggest?q=zzzz"} {
		rec := get(h, target)
		if rec.Code != http.StatusOK {
			t.Errorf("%s: status = %d, want 200", target, rec.Code)
		}
		if body := strings.TrimSpace(rec.Body.String()); body != "[]" {
			t.Errorf("%s: body = %q, want []", target, body)
		}
	}
}

func TestSuggest_ResultsAreCapped(t *testing.T) {
	srv, h := apiFixture()
	// 30 artists whose names all start with "zz".
	var artists []models.Artist
	for i := 1; i <= 30; i++ {
		artists = append(artists, models.Artist{ID: 100 + i, Name: "zz" + strings.Repeat("a", i)})
	}
	srv.Trie = search.BuildIndex(artists)

	var got []models.SearchEntry
	rec := get(h, "/api/suggest?q=zz")
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != suggestLimit {
		t.Errorf("got %d suggestions, want exactly %d", len(got), suggestLimit)
	}
}

func decodeFilter(t *testing.T, rec *httptest.ResponseRecorder) (ids []int, total int) {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %q)", rec.Code, rec.Body.String())
	}
	mustBeJSON(t, rec)
	var resp struct {
		ArtistIDs []int `json:"artist_ids"`
		Total     int   `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	return resp.ArtistIDs, resp.Total
}

func TestFilter(t *testing.T) {
	_, h := apiFixture()
	tests := []struct {
		name  string
		query string
		want  []int
	}{
		{"no params returns everyone", "", []int{1, 2, 3}},
		{"creation range", "?creation_min=2000&creation_max=2008", []int{2}},
		{"creation min only", "?creation_min=2000", []int{2, 3}},
		{"first album range", "?first_album_min=2010&first_album_max=2013", []int{2}},
		{"members", "?members=1", []int{2, 3}},
		{"several member counts", "?members=4,1", []int{1, 2, 3}},
		{"location", "?location=texas-usa", []int{1, 2}},
		{"location by state name", "?location=texas", []int{1, 2}},
		{"repeated location is OR", "?location=london&location=washington", []int{1, 3}},
		{"filters combine with AND", "?creation_min=2000&location=texas", []int{2}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ids, total := decodeFilter(t, get(h, "/api/filter"+tc.query))
			if !slices.Equal(ids, tc.want) {
				t.Errorf("artist_ids = %v, want %v", ids, tc.want)
			}
			if total != len(tc.want) {
				t.Errorf("total = %d, want %d", total, len(tc.want))
			}
		})
	}
}

func TestFilter_NoMatchGivesEmptyArrayNotNull(t *testing.T) {
	_, h := apiFixture()
	rec := get(h, "/api/filter?location=atlantis")
	if !strings.Contains(rec.Body.String(), `"artist_ids":[]`) {
		t.Errorf("body = %q, want \"artist_ids\":[]", rec.Body.String())
	}
}

func TestFilter_BadParamsAreJSON400(t *testing.T) {
	_, h := apiFixture()
	tests := []struct{ query, mention string }{
		{"?creation_min=abc", "creation_min"},
		{"?creation_max=1.5", "creation_max"},
		{"?first_album_min=-5", "first_album_min"},
		{"?first_album_max=x", "first_album_max"},
		{"?members=a", "members"},
		{"?members=0", "members"},
		{"?members=3,-1", "members"},
	}
	for _, tc := range tests {
		t.Run(tc.query, func(t *testing.T) {
			rec := get(h, "/api/filter"+tc.query)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", rec.Code)
			}
			mustBeJSON(t, rec)
			if !strings.Contains(rec.Body.String(), tc.mention) {
				t.Errorf("error %q should mention %q", rec.Body.String(), tc.mention)
			}
		})
	}
}

type geoResponse struct {
	Ready       bool               `json:"ready"`
	Coordinates []models.GeoResult `json:"coordinates"`
}

func TestGeo_NotReadyThenReadyAndChronological(t *testing.T) {
	srv, h := apiFixture()

	rec := get(h, "/api/artist/1/geo")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	mustBeJSON(t, rec)
	if !strings.Contains(rec.Body.String(), `"ready":false`) || !strings.Contains(rec.Body.String(), `"coordinates":[]`) {
		t.Errorf("before geocoding body = %q, want ready:false and coordinates:[]", rec.Body.String())
	}

	srv.Store.SetGeo(map[string]models.GeoResult{
		"london-uk": {Location: "london-uk", Lat: 51.5, Lng: -0.12, Resolved: true},
		"texas-usa": {Location: "texas-usa", Lat: 31, Lng: -99, Resolved: true},
	})

	var resp geoResponse
	rec = get(h, "/api/artist/1/geo")
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if !resp.Ready {
		t.Error("ready = false after SetGeo")
	}
	var order []string
	for _, c := range resp.Coordinates {
		order = append(order, c.Location)
	}
	// Queen played texas-usa on 01-02-2018 and london-uk on 10-06-2019.
	if !slices.Equal(order, []string{"texas-usa", "london-uk"}) {
		t.Errorf("order = %v, want chronological [texas-usa london-uk]", order)
	}
}

func TestGeo_BadIDAndUnknownArtist(t *testing.T) {
	_, h := apiFixture()
	tests := []struct {
		target string
		code   int
	}{
		{"/api/artist/abc/geo", http.StatusBadRequest},
		{"/api/artist/999/geo", http.StatusNotFound},
	}
	for _, tc := range tests {
		rec := get(h, tc.target)
		if rec.Code != tc.code {
			t.Errorf("%s: status = %d, want %d", tc.target, rec.Code, tc.code)
		}
		mustBeJSON(t, rec)
	}
}

func TestWriteJSON_EncodingFailureGivesJSON500(t *testing.T) {
	rec := httptest.NewRecorder()
	writeJSON(rec, http.StatusOK, map[string]any{"bad": func() {}}) // functions cannot be encoded
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
	mustBeJSON(t, rec)
}
