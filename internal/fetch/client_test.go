package fetch

import (
	"encoding/json"
	"groupie-tracker/internal/models"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const realArtistsJSON = `[{"id":1,"image":"x","name":"Queen","members":["Freddie Mercury","Brian May"],"creationDate":1970,"firstAlbum":"14-12-1973","locations":"https://x/locations/1","concertDates":"https://x/dates/1","relations":"https://x/relation/1"}]`
const realLocationsJSON = `{"index":[{"id":1,"locations":["north_carolina-usa","georgia-usa"],"dates":"https://x/dates/1"}]}`
const realDatesJSON = `{"index":[{"id":1,"dates":["*23-08-2019","*22-08-2019"]}]}`
const realRelationJSON = `{"index":[{"id":1,"datesLocations":{"georgia-usa":["22-08-2019"],"north_carolina-usa":["23-08-2019"]}}]}`

func TestDecode_Artists_RealShape(t *testing.T) {
	var artists []models.RawArtist
	if err := json.NewDecoder(strings.NewReader(realArtistsJSON)).Decode(&artists); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if len(artists) != 1 || artists[0].Name != "Queen" || artists[0].ID != 1 {
		t.Fatalf("unexpected result: %+v", artists)
	}
}

func TestDecode_Locations_RealShape(t *testing.T) {
	var wrapper models.LocationsIndexResp
	if err := json.NewDecoder(strings.NewReader(realLocationsJSON)).Decode(&wrapper); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if len(wrapper.Index) != 1 || wrapper.Index[0].ID != 1 {
		t.Fatalf("unexpected result: %+v", wrapper)
	}
}

func TestDecode_Dates_RealShape(t *testing.T) {
	var wrapper models.DatesIndexResp
	if err := json.NewDecoder(strings.NewReader(realDatesJSON)).Decode(&wrapper); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if len(wrapper.Index) != 1 || wrapper.Index[0].ID != 1 {
		t.Fatalf("unexpected result: %+v", wrapper)
	}
}

func TestDecode_Relations_RealShape(t *testing.T) {
	var wrapper models.RelationsIndexResp
	if err := json.NewDecoder(strings.NewReader(realRelationJSON)).Decode(&wrapper); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if len(wrapper.Index) != 1 || wrapper.Index[0].ID != 1 {
		t.Fatalf("unexpected result: %+v", wrapper)
	}
}

// ---- NEW: real end-to-end test, exercising the actual FetchAll() via
// httptest.NewServer, proving the URL-override mechanism genuinely works.
func TestFetchAll_Success_Mocked(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/artists", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(realArtistsJSON))
	})
	mux.HandleFunc("/locations", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(realLocationsJSON))
	})
	mux.HandleFunc("/dates", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(realDatesJSON))
	})
	mux.HandleFunc("/relation", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(realRelationJSON))
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	// Override the package-level URLs, restore afterwards.
	origArtists, origLocations, origDates, origRelations := artistsURL, locationsURL, datesURL, relationsURL
	artistsURL = ts.URL + "/artists"
	locationsURL = ts.URL + "/locations"
	datesURL = ts.URL + "/dates"
	relationsURL = ts.URL + "/relation"
	defer func() {
		artistsURL = origArtists
		locationsURL = origLocations
		datesURL = origDates
		relationsURL = origRelations
	}()

	data, err := FetchAll()
	if err != nil {
		t.Fatalf("FetchAll returned unexpected error: %v", err)
	}
	if len(data.Artists) != 1 || data.Artists[0].Name != "Queen" {
		t.Fatalf("unexpected artists: %+v", data.Artists)
	}
	if len(data.Locations) != 1 || len(data.Locations[0].Locations) != 2 {
		t.Fatalf("unexpected locations: %+v", data.Locations)
	}
	if len(data.Dates) != 1 || len(data.Dates[0].Dates) != 2 {
		t.Fatalf("unexpected dates: %+v", data.Dates)
	}
	if len(data.Relations) != 1 || len(data.Relations[0].DatesLocations) != 2 {
		t.Fatalf("unexpected relations: %+v", data.Relations)
	}
}

func TestFetchAll_PropagatesError_WhenOneEndpointFails(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/artists", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	mux.HandleFunc("/locations", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(realLocationsJSON))
	})
	mux.HandleFunc("/dates", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(realDatesJSON))
	})
	mux.HandleFunc("/relation", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(realRelationJSON))
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	origArtists, origLocations, origDates, origRelations := artistsURL, locationsURL, datesURL, relationsURL
	artistsURL = ts.URL + "/artists"
	locationsURL = ts.URL + "/locations"
	datesURL = ts.URL + "/dates"
	relationsURL = ts.URL + "/relation"
	defer func() {
		artistsURL = origArtists
		locationsURL = origLocations
		datesURL = origDates
		relationsURL = origRelations
	}()

	_, err := FetchAll()
	if err == nil {
		t.Fatalf("expected FetchAll to return an error when /artists fails, got nil")
	}
}
