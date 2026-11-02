package handlers

import (
	"math"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"groupie-tracker/internal/models"
	"groupie-tracker/internal/store"
)

// searchPageLimit caps the full-page search results.
const searchPageLimit = 100

// ---------- text helpers ----------

// titleCase upper-cases the first letter of every space-separated word.
func titleCase(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		r, size := utf8.DecodeRuneInString(w)
		words[i] = strings.ToUpper(string(r)) + w[size:]
	}
	return strings.Join(words, " ")
}

// splitLocation splits "los_angeles-usa" into a readable city ("Los Angeles")
// and country ("USA"). Short country names (usa, uk, uae) are upper-cased.
// A location without "-" is returned as the city with an empty country.
func splitLocation(raw string) (city, country string) {
	pretty := func(s string) string { return titleCase(strings.ReplaceAll(s, "_", " ")) }
	i := strings.LastIndex(raw, "-")
	if i < 0 {
		return pretty(raw), ""
	}
	city, country = pretty(raw[:i]), strings.ReplaceAll(raw[i+1:], "_", " ")
	if utf8.RuneCountInString(country) <= 3 {
		country = strings.ToUpper(country)
	} else {
		country = titleCase(country)
	}
	return city, country
}

// prettyLocation turns "los_angeles-usa" into "Los Angeles, USA".
func prettyLocation(raw string) string {
	city, country := splitLocation(raw)
	if country == "" {
		return city
	}
	return city + ", " + country
}

// formatKm writes a distance with thousands separators: 12313 -> "12,313 km".
func formatKm(km int) string {
	digits := strconv.Itoa(km)
	var b strings.Builder
	for i, d := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(d)
	}
	return b.String() + " km"
}

// searchTypeLabel is the human name of a search entry type.
func searchTypeLabel(t string) string {
	switch t {
	case "artist":
		return "Artist"
	case "member":
		return "Member"
	case "location":
		return "Location"
	case "creation_date":
		return "Creation year"
	case "first_album_date":
		return "First album"
	}
	return t
}

// ---------- artist page ----------

type concertView struct {
	Location string
	Dates    []string
}

type artistOption struct {
	ID   int
	Name string
}

type artistData struct {
	Artist    models.Artist
	Concerts  []concertView
	ShowCount int
	Others    []artistOption // every other artist, for the compare form
}

// artistOptions lists all artists except the one with ID skip (0 keeps all).
func (srv *Server) artistOptions(skip int) []artistOption {
	options := make([]artistOption, 0, len(srv.Store.Artists))
	for _, a := range srv.Store.Artists {
		if a.ID != skip {
			options = append(options, artistOption{a.ID, a.Name})
		}
	}
	slices.SortFunc(options, func(a, b artistOption) int {
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	return options
}

// ArtistHandler serves GET /artist/{id}: 400 for a malformed id, 404 for an
// unknown one.
func (srv *Server) ArtistHandler(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		BadRequest(w, r)
		return
	}
	artist, ok := srv.Store.ArtistsByID[id]
	if !ok {
		NotFound(w, r)
		return
	}

	data := artistData{Artist: artist, Others: srv.artistOptions(id)}
	for _, c := range store.ChronologicalConcerts(artist.ConcertLocations) {
		data.Concerts = append(data.Concerts, concertView{Location: prettyLocation(c.Location), Dates: c.Dates})
		data.ShowCount += len(c.Dates)
	}
	renderPage(w, http.StatusOK, "artist.html", data)
}

// ---------- search page ----------

type searchResult struct {
	Value    string
	Type     string
	Context  string
	ArtistID int
}

type searchData struct {
	Query    string
	Searched bool
	Results  []searchResult
}

// SearchHandler serves GET /search?q=...: the full-page results that also
// work without JavaScript (the header search form submits here).
func (srv *Server) SearchHandler(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	data := searchData{Query: q, Searched: q != "", Results: []searchResult{}}
	if q != "" {
		for _, e := range srv.searchEntries(q, searchPageLimit, false) {
			value := e.Value
			if e.Type == "location" {
				value = prettyLocation(value)
			}
			name := srv.Store.ArtistsByID[e.ArtistID].Name
			context := e.Context
			if e.Type != "artist" && e.Type != "member" {
				context = "Artist: " + name
			}
			data.Results = append(data.Results, searchResult{value, searchTypeLabel(e.Type), context, e.ArtistID})
		}
	}
	renderPage(w, http.StatusOK, "search.html", data)
}

// ---------- compare page ----------

type compareColumn struct {
	Artist        models.Artist
	LocationCount int
	ShowCount     int
	DistanceReady bool
	Distance      string // e.g. "12,313 km"
}

type compareData struct {
	All  []artistOption
	AID  int // selected IDs, 0 when none
	BID  int
	A, B *compareColumn // nil until both artists are chosen
}

func (srv *Server) compareColumn(artist models.Artist) *compareColumn {
	col := &compareColumn{Artist: artist, LocationCount: len(artist.ConcertLocations)}
	for _, c := range artist.ConcertLocations {
		col.ShowCount += len(c.Dates)
	}
	km, ready, _ := srv.Store.TourDistanceKm(artist.ID)
	col.DistanceReady = ready
	col.Distance = formatKm(int(math.Round(km)))
	return col
}

// CompareHandler serves GET /compare?a=ID&b=ID. Without both parameters it
// shows the chooser. A malformed id is a 400, an unknown one a 404.
func (srv *Server) CompareHandler(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	data := compareData{All: srv.artistOptions(0)}

	parse := func(key string) (id int, given bool, ok bool) {
		raw := strings.TrimSpace(query.Get(key))
		if raw == "" {
			return 0, false, true
		}
		id, err := strconv.Atoi(raw)
		return id, true, err == nil
	}
	aID, aGiven, aOK := parse("a")
	bID, bGiven, bOK := parse("b")
	if !aOK || !bOK {
		BadRequest(w, r)
		return
	}
	data.AID, data.BID = aID, bID

	if aGiven && bGiven {
		a, okA := srv.Store.ArtistsByID[aID]
		b, okB := srv.Store.ArtistsByID[bID]
		if !okA || !okB {
			NotFound(w, r)
			return
		}
		data.A, data.B = srv.compareColumn(a), srv.compareColumn(b)
	}
	renderPage(w, http.StatusOK, "compare.html", data)
}
