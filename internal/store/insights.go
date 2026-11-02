package store

import (
	"math"
	"slices"
	"sort"
	"strings"

	"groupie-tracker/internal/models"
)

// earthRadiusKm is the mean radius of the Earth, used by haversineKm.
const earthRadiusKm = 6371.0088

// haversineKm returns the great-circle distance between two points given in
// degrees. It is accurate to well under 1% for our purpose (tour length).
func haversineKm(lat1, lng1, lat2, lng2 float64) float64 {
	toRad := func(deg float64) float64 { return deg * math.Pi / 180 }
	dLat := toRad(lat2 - lat1)
	dLng := toRad(lng2 - lng1)
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(toRad(lat1))*math.Cos(toRad(lat2))*math.Sin(dLng/2)*math.Sin(dLng/2)
	return 2 * earthRadiusKm * math.Asin(math.Min(1, math.Sqrt(a)))
}

// PathLengthKm returns the length of the path that visits the coordinates in
// the given order. Fewer than two points give 0.
func PathLengthKm(coords []models.GeoResult) float64 {
	var km float64
	for i := 1; i < len(coords); i++ {
		km += haversineKm(coords[i-1].Lat, coords[i-1].Lng, coords[i].Lat, coords[i].Lng)
	}
	return km
}

// TourDistanceKm returns the length of the artist's tour: the sum of the
// distances between consecutive concert locations in chronological order.
// ready is false while geocoding is still running; found is false for an
// unknown artist. With fewer than two geocoded locations the distance is 0.
func (s *Store) TourDistanceKm(id int) (km float64, ready bool, found bool) {
	coords, ready, found := s.GeoFor(id)
	if !found || !ready {
		return 0, ready, found
	}
	return PathLengthKm(coords), true, true
}

// ChronologicalConcerts returns a copy of the concerts with the "*" prefix
// stripped from the dates, the dates of each location sorted oldest first,
// and the locations ordered by their earliest date. Unparseable dates sort
// last. The input is not modified.
func ChronologicalConcerts(in []models.ConcertLocation) []models.ConcertLocation {
	out := make([]models.ConcertLocation, len(in))
	for i, concert := range in {
		dates := make([]string, len(concert.Dates))
		for j, d := range concert.Dates {
			dates[j] = strings.TrimPrefix(strings.TrimSpace(d), "*")
		}
		slices.SortStableFunc(dates, func(a, b string) int {
			return earliestDate([]string{a}).Compare(earliestDate([]string{b}))
		})
		out[i] = models.ConcertLocation{Location: concert.Location, Dates: dates}
	}
	slices.SortStableFunc(out, func(a, b models.ConcertLocation) int {
		if c := earliestDate(a.Dates).Compare(earliestDate(b.Dates)); c != 0 {
			return c
		}
		return strings.Compare(a.Location, b.Location)
	})
	return out
}

// WorldPoint is one geocoded location together with every artist that plays
// there. The embedded GeoResult is flattened into the JSON object.
type WorldPoint struct {
	models.GeoResult
	ArtistIDs []int `json:"artist_ids"`
}

// WorldPoints returns every geocoded concert location once, with the IDs of
// the artists that play there, sorted by location. ready is false while
// geocoding is running. The slice is never nil.
func (s *Store) WorldPoints() (points []WorldPoint, ready bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	points = []WorldPoint{}
	if !s.ready {
		return points, false
	}

	byLocation := make(map[string][]int)
	for _, artist := range s.Artists {
		for _, concert := range artist.ConcertLocations {
			if _, ok := s.geo[concert.Location]; !ok {
				continue // geocoding failed for this location
			}
			ids := byLocation[concert.Location]
			if len(ids) == 0 || ids[len(ids)-1] != artist.ID {
				byLocation[concert.Location] = append(ids, artist.ID)
			}
		}
	}

	for location, ids := range byLocation {
		points = append(points, WorldPoint{GeoResult: s.geo[location], ArtistIDs: ids})
	}
	sort.Slice(points, func(i, j int) bool { return points[i].Location < points[j].Location })
	return points, true
}

// YearBounds returns the smallest and largest creation year and first-album
// year in the data; the filter sliders use them as their limits. Artists
// whose first album date cannot be parsed are ignored for the album range.
func (s *Store) YearBounds() (creationMin, creationMax, albumMin, albumMax int) {
	for i, artist := range s.Artists {
		if i == 0 || artist.CreationDate < creationMin {
			creationMin = artist.CreationDate
		}
		if artist.CreationDate > creationMax {
			creationMax = artist.CreationDate
		}
		year, ok := firstAlbumYear(artist.FirstAlbum)
		if !ok {
			continue
		}
		if albumMin == 0 || year < albumMin {
			albumMin = year
		}
		if year > albumMax {
			albumMax = year
		}
	}
	return creationMin, creationMax, albumMin, albumMax
}

// MemberCounts returns the distinct group sizes present in the data, sorted.
func (s *Store) MemberCounts() []int {
	counts := []int{}
	for _, artist := range s.Artists {
		if n := len(artist.Members); !slices.Contains(counts, n) {
			counts = append(counts, n)
		}
	}
	slices.Sort(counts)
	return counts
}
