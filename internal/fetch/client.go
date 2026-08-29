package fetch

import (
	"encoding/json"
	"fmt"
	"groupie-tracker/internal/models"
	"net/http"
	"sync"
)

const ArtistURL = "https://groupietrackers.herokuapp.com/api/artists"
const LocationURL = "https://groupietrackers.herokuapp.com/api/locations"
const DatesURL = "https://groupietrackers.herokuapp.com/api/dates"
const RelationURL = "https://groupietrackers.herokuapp.com/api/relation"

type RawData struct {
	Artists   []models.Artist
	Locations models.LocationsIndex
	Dates     models.DatesIndex
	Relations models.RelationsIndex
}

func FetchArtists() ([]models.Artist, error) {
	resp, err := http.Get(ArtistURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Unexpected status code: %d", resp.StatusCode)
	}
	var artists []models.Artist
	err = json.NewDecoder(resp.Body).Decode(&artists)
	if err != nil {
		return nil, err
	}
	return artists, nil
}

func FetchLocations() (models.LocationsIndex, error) {
	resp, err := http.Get(LocationURL)
	if err != nil {
		return models.LocationsIndex{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return models.LocationsIndex{}, fmt.Errorf("Unexpected status code: %d", resp.StatusCode)
	}
	var locations models.LocationsIndex
	err = json.NewDecoder(resp.Body).Decode(&locations)
	if err != nil {
		return models.LocationsIndex{}, err
	}
	return locations, nil
}

func FetchDates() (models.DatesIndex, error) {
	resp, err := http.Get(DatesURL)
	if err != nil {
		return models.DatesIndex{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return models.DatesIndex{}, fmt.Errorf("Unexpected status code: %d", resp.StatusCode)
	}
	var dates models.DatesIndex
	err = json.NewDecoder(resp.Body).Decode(&dates)
	if err != nil {
		return models.DatesIndex{}, err
	}
	return dates, nil
}

func FetchRelations() (models.RelationsIndex, error) {
	resp, err := http.Get(RelationURL)
	if err != nil {
		return models.RelationsIndex{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return models.RelationsIndex{}, fmt.Errorf("Unexpected status code: %d", resp.StatusCode)
	}
	var relations models.RelationsIndex
	err = json.NewDecoder(resp.Body).Decode(&relations)
	if err != nil {
		return models.RelationsIndex{}, err
	}
	return relations, nil
}

func FetchAll() (*RawData, error) {
	var wg sync.WaitGroup
	var data RawData
	errs := make(chan error, 4)
	wg.Add(4)
	go func() {
		defer wg.Done()
		artists, err := FetchArtists()
		if err != nil {
			errs <- err
			return
		}
		data.Artists = artists
	}()
	go func() {
		defer wg.Done()
		locations, err := FetchLocations()
		if err != nil {
			errs <- err
			return
		}
		data.Locations = locations
	}()
	go func() {
		defer wg.Done()
		dates, err := FetchDates()
		if err != nil {
			errs <- err
			return
		}
		data.Dates = dates
	}()
	go func() {
		defer wg.Done()
		relations, err := FetchRelations()
		if err != nil {
			errs <- err
			return
		}
		data.Relations = relations
	}()
	wg.Wait()
	close(errs)
	for err := range errs {
		return nil, err
	}
	return &data, nil
}
