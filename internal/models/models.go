package models

type Artist struct {
	ID           int      `json:"id"`
	Name         string   `json:"name"`
	Image        string   `json:"image"`
	Members      []string `json:"members"`
	CreationDate int      `json:"creationDate"`
	FirstAlbum   string   `json:"firstAlbum"`
	Locations    string   `json:"locations"`
	ConcertDates string   `json:"concertDates"`
	Relations    string   `json:"relations"`
}

type LocationsEntry struct {
	ID        int      `json:"id"`
	Locations []string `json:"locations"`
	Dates     string   `json:"dates"`
}

type LocationsIndex struct {
	Index []LocationsEntry `json:"index"`
}

type DatesEntry struct {
	ID    int      `json:"id"`
	Dates []string `json:"dates"`
}

type DatesIndex struct {
	Index []DatesEntry `json:"index"`
}

type RelationsEntry struct {
	ID             int                 `json:"id"`
	DatesLocations map[string][]string `json:"datesLocations"`
}

type RelationsIndex struct {
	Index []RelationsEntry `json:"index"`
}

type Coordinate struct {
	Lat         float64
	Lng         float64
	City        string
	State       string
	Country     string
	OriginalKey string
	Resolved    bool
}

type Suggestion struct {
	Text     string `json:"text"`
	Type     string `json:"type"`
	Context  string `json:"context"`
	ArtistID int    `json:"artist_id"`
}

type ArtistView struct {
	Artist
	ResolvedLocations []string
	ResolvedDates     []string
	DatesLocations    map[string][]string
	Coordinates       []Coordinate
}
