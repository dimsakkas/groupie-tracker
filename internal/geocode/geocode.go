package geocode

import (
	"encoding/json"
	"fmt"
	"groupie-tracker/internal/models"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// nominatimResult is the shape of the JSON that Nominatim returns.
// lat/lon arrive as strings, not numbers. For small places Nominatim fills
// town or village instead of city, so we read all three.
type nominatimResult struct {
	Lat     string `json:"lat"`
	Lon     string `json:"lon"`
	Address struct {
		City    string `json:"city"`
		Town    string `json:"town"`
		Village string `json:"village"`
		State   string `json:"state"`
		Country string `json:"country"`
	} `json:"address"`
}

// These are variables (not constants) so tests can replace them.
var (
	NominatimURL = "https://nominatim.openstreetmap.org/search"

	// One ticker shared by ALL workers: 1 request/sec in total.
	rateLimiter = time.NewTicker(1 * time.Second)

	// http.DefaultClient has no timeout: a hung request would block
	// a worker forever. That is why we use our own client.
	httpClient = &http.Client{Timeout: 10 * time.Second}
)

const UserAgent = "groupie-tracker-zone01-project/1.0"

// callNominatim sends exactly ONE query to Nominatim for the given search
// string and returns the result, or an error if nothing was found. It does
// not know about the "city-country" convention of our location keys — that
// decision belongs to Geocode, which calls this once or twice.
func callNominatim(location string) (models.GeoResult, error) {
	// 1. "north_carolina-usa" -> "north carolina, usa"
	query := strings.ReplaceAll(location, "_", " ")
	query = strings.ReplaceAll(query, "-", ", ")

	// 2. QueryEscape takes care of spaces, commas and special characters.
	// accept-language=en asks for English place names; without it Nominatim
	// answers in the local language (for example 名古屋市 instead of Nagoya).
	requrl := fmt.Sprintf("%s?q=%s&format=json&limit=1&addressdetails=1&accept-language=en",
		NominatimURL, url.QueryEscape(query))

	// 3. Build the request and set the User-Agent (required by Nominatim's usage policy).
	req, err := http.NewRequest(http.MethodGet, requrl, nil)
	if err != nil {
		return models.GeoResult{}, err
	}
	req.Header.Set("User-Agent", UserAgent)

	// 4. Wait for the ticker first, then send: this is what enforces 1 request/sec.
	<-rateLimiter.C
	resp, err := httpClient.Do(req)
	if err != nil {
		return models.GeoResult{}, err
	}
	defer resp.Body.Close()

	// 5. Failure checks.
	if resp.StatusCode != http.StatusOK {
		return models.GeoResult{}, fmt.Errorf("nominatim status %d for %q", resp.StatusCode, location)
	}
	var results []nominatimResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return models.GeoResult{}, err
	}
	if len(results) == 0 {
		return models.GeoResult{}, fmt.Errorf("no results for location: %s", location)
	}

	// 6. Parse the coordinates of the first result.
	first := results[0]
	lat, err := strconv.ParseFloat(first.Lat, 64)
	if err != nil {
		return models.GeoResult{}, err
	}
	lng, err := strconv.ParseFloat(first.Lon, 64)
	if err != nil {
		return models.GeoResult{}, err
	}

	city := first.Address.City
	if city == "" {
		city = first.Address.Town
	}
	if city == "" {
		city = first.Address.Village
	}

	// 7. Location is the ORIGINAL string (the key for cache and filter), not the query.
	return models.GeoResult{
		Location: location,
		Lat:      lat,
		Lng:      lng,
		City:     city,
		State:    first.Address.State,
		Country:  first.Address.Country,
		Resolved: true,
	}, nil
}

// Geocode turns a location string like "north_carolina-usa" into
// coordinates. It first asks for the full string; if that fails, it falls
// back to the city alone, split on the last "-". This matters for places
// whose "country" suffix no longer exists as a political entity — for
// example "willemstad-netherlands_antilles" (Netherlands Antilles was
// dissolved in 2010): Nominatim finds nothing for the full string, but
// does find "willemstad" on its own. Without this fallback that one
// location is permanently unresolved on every run.
func Geocode(location string) (models.GeoResult, error) {
	result, err := callNominatim(location)
	if err == nil {
		return result, nil
	}

	city := strings.Split(location, "-")[0]
	if city == location {
		// no "-" to split on: there is nothing left to fall back to
		return models.GeoResult{}, err
	}

	result, fallbackErr := callNominatim(city)
	if fallbackErr != nil {
		return models.GeoResult{}, fallbackErr
	}
	// The map lookup elsewhere uses the ORIGINAL key, not the city alone.
	result.Location = location
	return result, nil
}
