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
	Location string
	Lat      float64
	Lng      float64
	State    string
	Resolved bool
}

type SearchEntry struct {
	Value    string
	Type     string
	Context  string
	ArtistID int
}
