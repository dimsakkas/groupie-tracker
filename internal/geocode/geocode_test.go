package geocode

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func init() {
	// Without these, every test would wait 1-2 seconds.
	rateLimiter = time.NewTicker(time.Millisecond)
	retryDelay = time.Millisecond
}

// fakeNominatim starts a fake server and points NominatimURL at it.
func fakeNominatim(t *testing.T, h http.HandlerFunc) {
	t.Helper()
	srv := httptest.NewServer(h)
	old := NominatimURL
	NominatimURL = srv.URL
	t.Cleanup(func() {
		NominatimURL = old
		srv.Close()
	})
}

func TestGeocode_Success(t *testing.T) {
	var gotQuery, gotUA, gotLang string
	fakeNominatim(t, func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query().Get("q")
		gotUA = r.Header.Get("User-Agent")
		gotLang = r.URL.Query().Get("accept-language")
		w.Write([]byte(`[{"lat":"35.5","lon":"-80.1","address":{"state":"North Carolina","country":"United States"}}]`))
	})

	got, err := Geocode("north_carolina-usa")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotQuery != "north carolina, usa" {
		t.Errorf("query = %q, want %q", gotQuery, "north carolina, usa")
	}
	if gotLang != "en" {
		t.Errorf("accept-language = %q, want %q", gotLang, "en")
	}
	if gotUA != UserAgent {
		t.Errorf("User-Agent = %q, want %q", gotUA, UserAgent)
	}
	if got.Location != "north_carolina-usa" {
		t.Errorf("Location = %q, want the ORIGINAL string", got.Location)
	}
	if got.Lat != 35.5 || got.Lng != -80.1 {
		t.Errorf("coords = (%v, %v), want (35.5, -80.1)", got.Lat, got.Lng)
	}
	if got.State != "North Carolina" || got.Country != "United States" || !got.Resolved {
		t.Errorf("unexpected result: %+v", got)
	}
}

func TestGeocode_TownUsedWhenNoCity(t *testing.T) {
	fakeNominatim(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[{"lat":"1","lon":"2","address":{"town":"Landgraaf","country":"Netherlands"}}]`))
	})
	got, err := Geocode("landgraaf-netherlands")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.City != "Landgraaf" {
		t.Errorf("City = %q, want %q", got.City, "Landgraaf")
	}
}

func TestGeocode_Errors(t *testing.T) {
	tests := []struct {
		name string
		body string
		code int
	}{
		{"no results", `[]`, http.StatusOK},
		{"server error", `oops`, http.StatusInternalServerError},
		{"malformed json", `{not json`, http.StatusOK},
		{"bad latitude", `[{"lat":"abc","lon":"2","address":{}}]`, http.StatusOK},
		{"bad longitude", `[{"lat":"1","lon":"xyz","address":{}}]`, http.StatusOK},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fakeNominatim(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.code)
				w.Write([]byte(tc.body))
			})
			if _, err := Geocode("x-y"); err == nil {
				t.Fatal("expected an error, got nil")
			}
		})
	}
}

func TestGeocode_UnreachableServerReturnsError(t *testing.T) {
	old := NominatimURL
	NominatimURL = "http://127.0.0.1:1" // nothing is listening here
	t.Cleanup(func() { NominatimURL = old })
	if _, err := Geocode("x-y"); err == nil {
		t.Fatal("expected an error, got nil")
	}
}

func TestGeocodeAll_CacheHitsSkipNetworkAndFailuresAreSkipped(t *testing.T) {
	var mu sync.Mutex
	hits := map[string]int{}
	fakeNominatim(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		mu.Lock()
		hits[q]++
		mu.Unlock()
		if q == "london, uk" {
			w.Write([]byte(`[{"lat":"51.5","lon":"-0.12","address":{"city":"London","country":"United Kingdom"}}]`))
			return
		}
		w.Write([]byte(`[]`))
	})

	path := t.TempDir() + "/data/geocache.json"
	cache := LoadCache(path)
	cache.Set("texas-usa", modelsResult("texas-usa"))

	got := GeocodeAll([]string{"texas-usa", "london-uk", "nowhere-xx"}, cache)

	if len(got) != 2 {
		t.Fatalf("got %d results, want 2 (texas from cache, london from network): %+v", len(got), got)
	}
	if _, ok := got["nowhere-xx"]; ok {
		t.Error("a failed location must be skipped, not returned")
	}
	if hits["texas, usa"] != 0 {
		t.Errorf("cached location hit the network %d times", hits["texas, usa"])
	}
	if hits["london, uk"] != 1 {
		t.Errorf("london requested %d times, want 1", hits["london, uk"])
	}
	if hits["nowhere, xx"] != 2 {
		t.Errorf("failing location requested %d times, want 2 (1 try + 1 retry)", hits["nowhere, xx"])
	}

	// GeocodeAll must have saved the cache to disk.
	reloaded := LoadCache(path)
	if _, ok := reloaded.Get("london-uk"); !ok {
		t.Error("london-uk was not persisted to the disk cache")
	}
}
