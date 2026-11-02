package handlers

import (
	"cmp"
	"net/http"
	"slices"

	"groupie-tracker/internal/models"
)

type cityOption struct {
	Value string // raw location, sent to /api/filter
	Label string
}

// stateGroup holds the cities within one geocoded state. Name is empty for
// countries where Nominatim has no state information (e.g. Japan) — the
// template skips the state header in that case and lists the cities
// directly under the country.
type stateGroup struct {
	Name   string
	Cities []cityOption
}

type countryGroup struct {
	Name   string
	States []stateGroup
}

// filterOptions holds everything the filter panel needs to draw itself.
type filterOptions struct {
	CreationMin, CreationMax int
	AlbumMin, AlbumMax       int
	MemberCounts             []int
	Countries                []countryGroup
}

// homeData is what index.html receives.
type homeData struct {
	Artists []models.Artist
	Filters filterOptions
}

// locationGroups groups the raw locations by country, then by geocoded
// state within each country, for the filter panel. states maps a raw
// location to its lowercased geocoded state (from Store.LocationStates());
// a location with no entry there (not geocoded yet, or the country has no
// state concept) is grouped under the country's unnamed state.
func locationGroups(locations []string, states map[string]string) []countryGroup {
	type countryKey struct{ byState map[string]*stateGroup }
	byCountry := make(map[string]*countryKey)
	var countryOrder []string

	for _, raw := range locations {
		city, country := splitLocation(raw)
		if country == "" {
			country = city // no "-" in the raw value: the place is its own group
		}

		ck, ok := byCountry[country]
		if !ok {
			ck = &countryKey{byState: make(map[string]*stateGroup)}
			byCountry[country] = ck
			countryOrder = append(countryOrder, country)
		}

		stateName := states[raw] // "" when not geocoded yet, or no state for this country
		sg, ok := ck.byState[stateName]
		if !ok {
			sg = &stateGroup{Name: titleCase(stateName)}
			ck.byState[stateName] = sg
		}
		sg.Cities = append(sg.Cities, cityOption{Value: raw, Label: city})
	}

	out := make([]countryGroup, 0, len(countryOrder))
	for _, country := range countryOrder {
		ck := byCountry[country]
		sgs := make([]stateGroup, 0, len(ck.byState))
		for _, sg := range ck.byState {
			slices.SortFunc(sg.Cities, func(a, b cityOption) int { return cmp.Compare(a.Label, b.Label) })
			sgs = append(sgs, *sg)
		}
		// Unnamed state (no geocoded state for this country) sorts first,
		// so e.g. a country with a "general" entry shows it before named states.
		slices.SortFunc(sgs, func(a, b stateGroup) int {
			if a.Name == "" {
				return -1
			}
			if b.Name == "" {
				return 1
			}
			return cmp.Compare(a.Name, b.Name)
		})
		out = append(out, countryGroup{Name: country, States: sgs})
	}
	slices.SortFunc(out, func(a, b countryGroup) int { return cmp.Compare(a.Name, b.Name) })
	return out
}

// HomeHandler renders the landing page: one card per artist plus the filter
// panel. The route is registered as "GET /{$}", so it only sees "/".
func (srv *Server) HomeHandler(w http.ResponseWriter, r *http.Request) {
	cMin, cMax, aMin, aMax := srv.Store.YearBounds()
	states := srv.Store.LocationStates() // ONE call, ONE lock — see store.go
	renderPage(w, http.StatusOK, "index.html", homeData{
		Artists: srv.Store.Artists,
		Filters: filterOptions{
			CreationMin: cMin, CreationMax: cMax,
			AlbumMin: aMin, AlbumMax: aMax,
			MemberCounts: srv.Store.MemberCounts(),
			Countries:    locationGroups(srv.Store.UniqueLocations(), states),
		},
	})
}
