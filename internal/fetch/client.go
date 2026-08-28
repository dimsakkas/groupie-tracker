package fetch

import (
	"encoding/json"
	"fmt"
	"groupie-tracker/internal/models"
	"log"
	"net/http"
)

type artistsResult struct {
	data []models.RawArtist
	err  error
}

type locationResult struct {
	data []models.RawLocationResp
	err  error
}

type datesResult struct {
	data []models.RawDatesResp
	err  error
}

type relationsResult struct {
	data []models.RawRelationsResp
	err  error
}

type RawData struct {
	Artists   []models.RawArtist
	Locations []models.RawLocationResp
	Dates     []models.RawDatesResp
	Relations []models.RawRelationsResp
}

func fetchArtists(ch chan<- artistsResult) {
	resp, err := http.Get("https://groupietrackers.herokuapp.com/api/artists")
	if err != nil {
		log.Printf("fetchArtists: network error %v", err)
		ch <- artistsResult{nil, fmt.Errorf("fetch artists: %w", err)}
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("fetchArtists: bad status %d", resp.StatusCode)
		ch <- artistsResult{err: fmt.Errorf("fetch artists: status %d", resp.StatusCode)}
		return
	}

	var artists []models.RawArtist
	if err := json.NewDecoder(resp.Body).Decode(&artists); err != nil {
		log.Printf("fetchArtists: decode error %v", err)
		ch <- artistsResult{err: fmt.Errorf("decode artists: %w", err)}
		return
	}
	ch <- artistsResult{data: artists}
}

func fetchLocations(ch chan<- locationResult) {
	resp, err := http.Get("https://groupietrackers.herokuapp.com/api/locations")
	if err != nil {
		log.Printf("fetchLocations: network error %v", err)
		ch <- locationResult{nil, fmt.Errorf("fetch locations: %w", err)}
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("fetchLocations: bad status %d", resp.StatusCode)
		ch <- locationResult{err: fmt.Errorf("fetch locations: status %d", resp.StatusCode)}
		return
	}

	var wrapper models.LocationsIndexResp
	if err := json.NewDecoder(resp.Body).Decode(&wrapper); err != nil {
		log.Printf("fetchLocations: decode error %v", err)
		ch <- locationResult{err: fmt.Errorf("decode locations: %w", err)}
		return
	}
	ch <- locationResult{data: wrapper.Index}
}

func fetchDates(ch chan<- datesResult) {
	resp, err := http.Get("https://groupietrackers.herokuapp.com/api/dates")
	if err != nil {
		log.Printf("fetchDates: network error %v", err)
		ch <- datesResult{nil, fmt.Errorf("fetch dates: %w", err)}
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("fetchDates: bad status %d", resp.StatusCode)
		ch <- datesResult{err: fmt.Errorf("fetch dates: status %d", resp.StatusCode)}
		return
	}

	var wrapper models.DatesIndexResp
	if err := json.NewDecoder(resp.Body).Decode(&wrapper); err != nil {
		log.Printf("fetchDates: decode error %v", err)
		ch <- datesResult{err: fmt.Errorf("decode dates: %w", err)}
		return
	}
	ch <- datesResult{data: wrapper.Index}
}

func fetchRelations(ch chan<- relationsResult) {
	resp, err := http.Get("https://groupietrackers.herokuapp.com/api/relation")
	if err != nil {
		log.Printf("fetchRelations: network error %v", err)
		ch <- relationsResult{nil, fmt.Errorf("fetch relations: %w", err)}
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("fetchRelations: bad status %d", resp.StatusCode)
		ch <- relationsResult{err: fmt.Errorf("fetch relations: status %d", resp.StatusCode)}
		return
	}

	var wrapper models.RelationsIndexResp
	if err := json.NewDecoder(resp.Body).Decode(&wrapper); err != nil {
		log.Printf("fetchRelations: decode error %v", err)
		ch <- relationsResult{err: fmt.Errorf("decode relations: %w", err)}
		return
	}
	ch <- relationsResult{data: wrapper.Index}
}

func FetchAll() (RawData, error) {
	artistsCh := make(chan artistsResult, 1)
	locationCh := make(chan locationResult, 1)
	datesCh := make(chan datesResult, 1)
	relationsCh := make(chan relationsResult, 1)

	go fetchArtists(artistsCh)
	go fetchLocations(locationCh)
	go fetchDates(datesCh)
	go fetchRelations(relationsCh)

	artistsRes := <-artistsCh
	if artistsRes.err != nil {
		return RawData{}, artistsRes.err
	}

	locationRes := <-locationCh
	if locationRes.err != nil {
		return RawData{}, locationRes.err
	}

	datesRes := <-datesCh
	if datesRes.err != nil {
		return RawData{}, datesRes.err
	}

	relationsRes := <-relationsCh
	if relationsRes.err != nil {
		return RawData{}, relationsRes.err
	}
	return RawData{
		Artists:   artistsRes.data,
		Locations: locationRes.data,
		Dates:     datesRes.data,
		Relations: relationsRes.data,
	}, nil
}
