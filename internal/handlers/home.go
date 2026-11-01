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

type countryGroup struct {
	Name   string
	Cities []cityOption
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

// locationGroups groups the raw locations by country for the filter panel.
func locationGroups(locations []string) []countryGroup {
	byCountry := make(map[string]*countryGroup)
	var groups []*countryGroup
	for _, raw := range locations {
		city, country := splitLocation(raw)
		if country == "" {
			country = city // no "-" in the raw value: the place is its own group
		}
		g, ok := byCountry[country]
		if !ok {
			g = &countryGroup{Name: country}
			byCountry[country] = g
			groups = append(groups, g)
		}
		g.Cities = append(g.Cities, cityOption{Value: raw, Label: city})
	}

	out := make([]countryGroup, 0, len(groups))
	for _, g := range groups {
		slices.SortFunc(g.Cities, func(a, b cityOption) int { return cmp.Compare(a.Label, b.Label) })
		out = append(out, *g)
	}
	slices.SortFunc(out, func(a, b countryGroup) int { return cmp.Compare(a.Name, b.Name) })
	return out
}

// HomeHandler renders the landing page: one card per artist plus the filter
// panel. The route is registered as "GET /{$}", so it only sees "/".
func (srv *Server) HomeHandler(w http.ResponseWriter, r *http.Request) {
	cMin, cMax, aMin, aMax := srv.Store.YearBounds()
	renderPage(w, http.StatusOK, "index.html", homeData{
		Artists: srv.Store.Artists,
		Filters: filterOptions{
			CreationMin: cMin, CreationMax: cMax,
			AlbumMin: aMin, AlbumMax: aMax,
			MemberCounts: srv.Store.MemberCounts(),
			Countries:    locationGroups(srv.Store.UniqueLocations()),
		},
	})
}
