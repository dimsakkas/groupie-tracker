package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"groupie-tracker/internal/models"
	"groupie-tracker/internal/store"
	"log"
	"math"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
)

// suggestLimit caps the autocomplete dropdown: more than 10 entries is
// scroll-hell for the user and an unbounded response for the server.
const suggestLimit = 10

// searchPoolSize is how many raw trie hits searchEntries collects before it
// sorts and truncates them. The trie walks its children in random map order,
// so without a larger pool the visible results would change between requests.
const searchPoolSize = 1000

// searchTypeRank orders result types: the artist itself first, dates last.
var searchTypeRank = map[string]int{
	"artist": 0, "member": 1, "location": 2, "creation_date": 3, "first_album_date": 4,
}

func isSearchSeparator(r rune) bool { return r == ' ' || r == '-' || r == '_' }

// searchEntries answers a query that may contain several words
// ("billie joe", "london-uk", "05-08-1967"). The trie only knows single
// tokens, so we look up the first token and keep the hits in which every other
// token is the prefix of some word. The result is sorted (type, then value,
// then artist) so it is the same on every request.
//
// With collapse=true, location and date entries are reduced to one per
// (type, value): the dropdown wants "Japan" once, not once per artist.
func (srv *Server) searchEntries(q string, limit int, collapse bool) []models.SearchEntry {
	tokens := strings.FieldsFunc(strings.ToLower(q), isSearchSeparator)
	if len(tokens) == 0 {
		return []models.SearchEntry{}
	}

	hits := srv.Trie.Search(tokens[0], searchPoolSize)
	out := make([]models.SearchEntry, 0, len(hits))
	for _, entry := range hits {
		if hasWordPrefixes(entry.Value, tokens[1:]) {
			out = append(out, entry)
		}
	}

	slices.SortFunc(out, func(a, b models.SearchEntry) int {
		if c := searchTypeRank[a.Type] - searchTypeRank[b.Type]; c != 0 {
			return c
		}
		if c := strings.Compare(strings.ToLower(a.Value), strings.ToLower(b.Value)); c != 0 {
			return c
		}
		return a.ArtistID - b.ArtistID
	})

	if collapse {
		type key struct{ typ, value string }
		seen := make(map[key]bool)
		out = slices.DeleteFunc(out, func(e models.SearchEntry) bool {
			if e.Type == "artist" || e.Type == "member" {
				return false
			}
			k := key{e.Type, e.Value}
			dup := seen[k]
			seen[k] = true
			return dup
		})
	}

	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

// hasWordPrefixes reports whether every token is the prefix of some word of
// value (words are separated like in the trie).
func hasWordPrefixes(value string, tokens []string) bool {
	if len(tokens) == 0 {
		return true
	}
	words := strings.FieldsFunc(strings.ToLower(value), isSearchSeparator)
	for _, token := range tokens {
		if !slices.ContainsFunc(words, func(w string) bool { return strings.HasPrefix(w, token) }) {
			return false
		}
	}
	return true
}

// writeJSON encodes v into a buffer first and writes it only on success, so an
// encoding error can never leave a half-written JSON body behind.
func writeJSON(w http.ResponseWriter, status int, v any) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(v); err != nil {
		log.Printf("cannot encode JSON response: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal server error"}` + "\n"))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	buf.WriteTo(w)
}

// writeJSONError keeps API errors valid JSON, so the browser's JS can always
// call response.json(), also when something went wrong.
func writeJSONError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// SuggestHandler serves GET /api/suggest?q=... (autocomplete).
func (srv *Server) SuggestHandler(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		writeJSON(w, http.StatusOK, []models.SearchEntry{})
		return
	}
	// searchEntries never returns nil, so the JSON is [] and not null.
	writeJSON(w, http.StatusOK, srv.searchEntries(q, suggestLimit, true))
}

// FilterHandler serves GET /api/filter?... and returns only the IDs of the
// matching artists: the browser already has every card in the DOM, so sending
// IDs is far smaller than sending whole artists.
func (srv *Server) FilterHandler(w http.ResponseWriter, r *http.Request) {
	params, err := parseFilterParams(r.URL.Query())
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	ids := srv.Store.Filter(params)
	writeJSON(w, http.StatusOK, struct {
		ArtistIDs []int `json:"artist_ids"`
		Total     int   `json:"total"`
	}{ids, len(ids)})
}

// GeoHandler serves GET /api/artist/{id}/geo. While the background geocoding
// is still running, "ready" is false and "coordinates" is empty: the map
// script can then show a loading message and ask again a bit later.
func (srv *Server) GeoHandler(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid artist id")
		return
	}
	coords, ready, found := srv.Store.GeoFor(id)
	if !found {
		writeJSONError(w, http.StatusNotFound, "artist not found")
		return
	}
	writeJSON(w, http.StatusOK, struct {
		Ready       bool               `json:"ready"`
		Coordinates []models.GeoResult `json:"coordinates"`
		DistanceKm  int                `json:"distance_km"`
	}{ready, coords, int(math.Round(store.PathLengthKm(coords)))})
}

// WorldGeoHandler serves GET /api/geo: every geocoded concert location once,
// with the IDs of the artists that play there. The home page map uses it and
// hides the artists that the filters exclude.
func (srv *Server) WorldGeoHandler(w http.ResponseWriter, r *http.Request) {
	points, ready := srv.Store.WorldPoints()
	writeJSON(w, http.StatusOK, struct {
		Ready     bool               `json:"ready"`
		Locations []store.WorldPoint `json:"locations"`
	}{ready, points})
}

// parseFilterParams reads the filter query string:
//
//	creation_min, creation_max, first_album_min, first_album_max  whole numbers
//	members                                                       comma-separated, e.g. 3,5
//	location                                                      repeatable, e.g. ?location=uk&location=texas
//
// A missing parameter means "no filter". A malformed one is an error.
func parseFilterParams(values url.Values) (store.FilterParams, error) {
	var p store.FilterParams
	var err error

	if p.CreationMin, err = intParam(values, "creation_min"); err != nil {
		return p, err
	}
	if p.CreationMax, err = intParam(values, "creation_max"); err != nil {
		return p, err
	}
	if p.FirstAlbumMin, err = intParam(values, "first_album_min"); err != nil {
		return p, err
	}
	if p.FirstAlbumMax, err = intParam(values, "first_album_max"); err != nil {
		return p, err
	}

	for _, part := range strings.Split(values.Get("members"), ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		n, err := strconv.Atoi(part)
		if err != nil || n < 1 {
			return p, fmt.Errorf("members must be a comma-separated list of positive whole numbers")
		}
		p.Members = append(p.Members, n)
	}

	p.Locations = values["location"]
	return p, nil
}

// intParam reads one optional non-negative integer parameter (missing -> 0).
func intParam(values url.Values, key string) (int, error) {
	raw := strings.TrimSpace(values.Get(key))
	if raw == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be a whole number", key)
	}
	if n < 0 {
		return 0, fmt.Errorf("%s must not be negative", key)
	}
	return n, nil
}
