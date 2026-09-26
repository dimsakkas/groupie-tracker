package store

import (
	"groupie-tracker/internal/fetch"
	"groupie-tracker/internal/models"
)

type Store struct {
	ArtistsByID map[int]models.Artist
}

func Build(raw fetch.RawData) *Store {
	artists := BuildArtists(raw)
	artistsByID := make(map[int]models.Artist)
	for _, artist := range artists {
		artistsByID[artist.ID] = artist
	}
	return &Store{
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
