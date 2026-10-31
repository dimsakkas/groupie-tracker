package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"groupie-tracker/internal/models"
	"groupie-tracker/internal/store"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// suggestLimit caps the autocomplete dropdown: more than 10 entries is
// scroll-hell for the user and an unbounded response for the server.
const suggestLimit = 10

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
	results := srv.Trie.Search(q, suggestLimit)
	if results == nil {
		results = []models.SearchEntry{} // JSON [] and not null
	}
	writeJSON(w, http.StatusOK, results)
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
	}{ready, coords})
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
