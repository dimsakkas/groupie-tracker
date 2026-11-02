package models

type RawArtist struct {
	ID           int      `json:"id"`
	Image        string   `json:"image"`
	Name         string   `json:"name"`
	Members      []string `json:"members"`
	CreationDate int      `json:"creationDate"`
	FirstAlbum   string   `json:"firstAlbum"`
	LocationURL  string   `json:"locations"`
	DatesURL     string   `json:"concertDates"`
	RelationsURL string   `json:"relations"`
}

type RawLocationResp struct {
	ID        int      `json:"id"`
	Locations []string `json:"locations"`
	DatesURL  string   `json:"dates"`
}

type LocationsIndexResp struct {
	Index []RawLocationResp `json:"index"`
}

type RawDatesResp struct {
	ID    int      `json:"id"`
	Dates []string `json:"dates"`
}

type DatesIndexResp struct {
	Index []RawDatesResp `json:"index"`
}

type RawRelationsResp struct {
	ID             int                 `json:"id"`
	DatesLocations map[string][]string `json:"datesLocations"`
}

type RelationsIndexResp struct {
	Index []RawRelationsResp `json:"index"`
}

type ConcertLocation struct {
	Location string
	Dates    []string
}

type Artist struct {
	ID               int
	Name             string
	Image            string
	Members          []string
	CreationDate     int
	FirstAlbum       string
	Locations        []string
	Dates            []string
	ConcertLocations []ConcertLocation
}

type GeoResult struct {
	Location string  `json:"location"`
	Lat      float64 `json:"lat"`
	Lng      float64 `json:"lng"`
	City     string  `json:"city"`
	State    string  `json:"state"`
	Country  string  `json:"country"`
	Resolved bool    `json:"resolved"`
}

type SearchEntry struct {
	Value    string `json:"value"`
	Type     string `json:"type"`
	Context  string `json:"context"`
	ArtistID int    `json:"artist_id"`
}
