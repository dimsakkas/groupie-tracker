package store

import (
	"cmp"
	"groupie-tracker/internal/fetch"
	"groupie-tracker/internal/models"
	"maps"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Store keeps all data in memory. Artists/ArtistsByID are written
// once in Build and only read afterwards (safe without a lock).
// geo/ready are written later by the background geocoding, so
// they are protected by mu.
type Store struct {
	Artists     []models.Artist
	ArtistsByID map[int]models.Artist

	mu             sync.RWMutex
	geo            map[string]models.GeoResult
	locationStates map[string]string // raw location -> geocoded State (lowercased), for filtering
	ready          bool
}

type FilterParams struct {
	CreationMin   int
	CreationMax   int
	FirstAlbumMin int
	FirstAlbumMax int
	Members       []int
	Locations     []string
}

func Build(raw fetch.RawData) *Store {
	return newStore(BuildArtists(raw))
}

func newStore(artists []models.Artist) *Store {
	sorted := make([]models.Artist, len(artists))
	copy(sorted, artists)
	slices.SortFunc(sorted, func(a, b models.Artist) int {
		return cmp.Compare(a.ID, b.ID)
	})
	artistsByID := make(map[int]models.Artist, len(sorted))
	for _, artist := range sorted {
		artistsByID[artist.ID] = artist
	}
	return &Store{
		Artists:     sorted,
		ArtistsByID: artistsByID,
	}
}

func BuildArtists(raw fetch.RawData) []models.Artist {

	locationsByID := make(map[int]models.RawLocationResp)
	for _, loc := range raw.Locations {
		locationsByID[loc.ID] = loc
	}

	datesByID := make(map[int]models.RawDatesResp)
	for _, date := range raw.Dates {
		datesByID[date.ID] = date
	}

	relationsByID := make(map[int]models.RawRelationsResp)
	for _, rel := range raw.Relations {
		relationsByID[rel.ID] = rel
	}

	var artists []models.Artist
	for _, artist := range raw.Artists {
		rel := relationsByID[artist.ID]
		loc := locationsByID[artist.ID]
		date := datesByID[artist.ID]

		var concerts []models.ConcertLocation
		for location, dates := range rel.DatesLocations {
			concerts = append(concerts, models.ConcertLocation{
				Location: location,
				Dates:    dates,
			})
		}
		// Ranging over a map gives a random order: make it deterministic.
		slices.SortFunc(concerts, func(a, b models.ConcertLocation) int {
			return strings.Compare(a.Location, b.Location)
		})
		artists = append(artists, models.Artist{
			ID:               artist.ID,
			Name:             artist.Name,
			Members:          artist.Members,
			CreationDate:     artist.CreationDate,
			FirstAlbum:       artist.FirstAlbum,
			Image:            artist.Image,
			Locations:        loc.Locations,
			Dates:            date.Dates,
			ConcertLocations: concerts,
		})
	}
	return artists
}

// Filter returns the IDs of the artists that pass ALL filters (AND).
// A filter that was not given (0 or an empty list) excludes nobody. Within
// Locations, OR applies: one of the requested locations is enough.
// The result is never nil, so the JSON output is [] and not null.
func (s *Store) Filter(p FilterParams) []int {
	wanted := normalizeTerms(p.Locations)
	ids := []int{}
	s.mu.RLock()
	locationStates := s.locationStates
	s.mu.RUnlock()
	for _, artist := range s.Artists {
		if p.CreationMin != 0 && artist.CreationDate < p.CreationMin {
			continue
		}
		if p.CreationMax != 0 && artist.CreationDate > p.CreationMax {
			continue
		}

		if p.FirstAlbumMin != 0 || p.FirstAlbumMax != 0 {
			year, ok := firstAlbumYear(artist.FirstAlbum)
			if !ok {
				continue
			}
			if p.FirstAlbumMin != 0 && year < p.FirstAlbumMin {
				continue
			}
			if p.FirstAlbumMax != 0 && year > p.FirstAlbumMax {
				continue
			}
		}

		if len(p.Members) > 0 && !slices.Contains(p.Members, len(artist.Members)) {
			continue
		}

		if len(wanted) > 0 && !matchesAnyLocation(artist.Locations, wanted, locationStates) {
			continue
		}

		ids = append(ids, artist.ID)
	}
	return ids
}

// firstAlbumYear extracts the year from "DD-MM-YYYY". It returns ok=false for
// any other format, instead of panicking with index out of range.
func firstAlbumYear(firstAlbum string) (int, bool) {
	parts := strings.Split(firstAlbum, "-")
	if len(parts) != 3 {
		return 0, false
	}
	year, err := strconv.Atoi(parts[2])
	if err != nil {
		return 0, false
	}
	return year, true
}

// normalize lowercases and replaces "-" and "_" with a space, so
// "north carolina", "North_Carolina" and "texas-usa" compare correctly.
func normalize(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, "_", " ")
	s = strings.ReplaceAll(s, "-", " ")
	return strings.Join(strings.Fields(s), " ")
}

// normalizeTerms normalizes the terms and drops the empty ones:
// strings.Contains(x, "") is always true and would match every artist.
func normalizeTerms(terms []string) []string {
	var out []string
	for _, t := range terms {
		if n := normalize(t); n != "" {
			out = append(out, n)
		}
	}
	return out
}

// matchesAnyLocation reports whether at least one of the artist's locations
// matches one of the wanted terms — either because the term appears in the
// normalized raw string itself ("houston" matches "houston-usa"), or
// because the term equals the location's geocoded state ("texas" matches
// "houston-usa" once we know Houston is in Texas).
func matchesAnyLocation(locations []string, wanted []string, locationStates map[string]string) bool {
	for _, loc := range locations {
		n := normalize(loc)
		state := locationStates[loc] // already lowercased in SetGeo; "" if not geocoded yet
		for _, w := range wanted {
			if strings.Contains(n, w) {
				return true
			}
			if state != "" && state == w {
				return true
			}
		}
	}
	return false
}

// UniqueLocations returns every location once, sorted.
func (s *Store) UniqueLocations() []string {
	unique := make(map[string]struct{})
	for _, artist := range s.Artists {
		for _, loc := range artist.Locations {
			unique[loc] = struct{}{}
		}
	}
	return slices.Sorted(maps.Keys(unique))
}

// LocationStates returns a snapshot of the raw-location-to-state lookup,
// taken under a single lock so the caller sees one consistent view instead
// of 189 independently-locked reads that could straddle a SetGeo update.
func (s *Store) LocationStates() map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.locationStates
}

// SetGeo is called once, when geocoding finishes. The caller must not
// modify the map after the call.
func (s *Store) SetGeo(results map[string]models.GeoResult) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.geo = results

	states := make(map[string]string, len(results))
	for raw, r := range results {
		if r.State != "" {
			states[raw] = strings.ToLower(r.State)
		}
	}
	s.locationStates = states

	s.ready = true
}

// GeoFor returns the coordinates of an artist's concerts in chronological order.
// Everything is read under the same lock, so ready and coords are always
// consistent with each other. found=false means an unknown artist.
// coords is never nil.
func (s *Store) GeoFor(id int) (coords []models.GeoResult, ready bool, found bool) {
	artist, ok := s.ArtistsByID[id]
	if !ok {
		return []models.GeoResult{}, false, false
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	coords = []models.GeoResult{}
	if !s.ready {
		return coords, false, true
	}

	type visit struct {
		when   time.Time
		result models.GeoResult
	}
	visits := make([]visit, 0, len(artist.ConcertLocations))
	for _, concert := range artist.ConcertLocations {
		result, ok := s.geo[concert.Location]
		if !ok {
			continue // geocoding failed for this location: no marker
		}
		visits = append(visits, visit{earliestDate(concert.Dates), result})
	}

	slices.SortFunc(visits, func(a, b visit) int {
		if c := a.when.Compare(b.when); c != 0 {
			return c
		}
		return strings.Compare(a.result.Location, b.result.Location)
	})
	for _, v := range visits {
		coords = append(coords, v.result)
	}
	return coords, true, true
}

// earliestDate returns the earliest valid date in "DD-MM-YYYY" format (the
// dates endpoint prefixes "*", which we strip). If none can be parsed,
// it returns a far-future date so the location sorts last.
func earliestDate(dates []string) time.Time {
	earliest := time.Date(9999, time.December, 31, 0, 0, 0, 0, time.UTC)
	for _, d := range dates {
		d = strings.TrimPrefix(strings.TrimSpace(d), "*")
		t, err := time.Parse("02-01-2006", d)
		if err != nil {
			continue
		}
		if t.Before(earliest) {
			earliest = t
		}
	}
	return earliest
}
