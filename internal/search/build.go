package search

import (
	"groupie-tracker/internal/models"
	"strconv"
)

// BuildIndex takes the store's already-combined artists and plants every
// searchable field into a fresh Trie. It depends only on models.Artist --
// no import of the store package -- so it can be built and tested with a
// plain []models.Artist slice.
//
// Indexed fields, and why:
//   - Name, Members, Locations: the core cross-type match requirement
//     (search audit: "queen" -> Queen the artist AND Queensland the place).
//   - CreationDate, FirstAlbum: bonus scope, not required by the audit but
//     cheap to add once the trie mechanics exist.
//
// Deliberately NOT indexed: Image (a URL, never something a person types
// into a search box), Dates and ConcertLocations (redundant with Locations,
// would just insert the same location strings a second time).
func BuildIndex(artists []models.Artist) *Trie {
	trie := NewTrie()

	for _, artist := range artists {
		trie.InsertMultiToken(artist.Name, models.SearchEntry{
			Value:    artist.Name,
			Type:     "artist",
			ArtistID: artist.ID,
		})

		for _, member := range artist.Members {
			trie.InsertMultiToken(member, models.SearchEntry{
				Value:    member,
				Type:     "member",
				Context:  "Member of " + artist.Name,
				ArtistID: artist.ID,
			})
		}

		for _, loc := range artist.Locations {
			trie.InsertMultiToken(loc, models.SearchEntry{
				Value:    loc,
				Type:     "location",
				ArtistID: artist.ID,
			})
		}

		creationYear := strconv.Itoa(artist.CreationDate)
		trie.InsertMultiToken(creationYear, models.SearchEntry{
			Value:    creationYear,
			Type:     "creation_date",
			ArtistID: artist.ID,
		})

		trie.InsertMultiToken(artist.FirstAlbum, models.SearchEntry{
			Value:    artist.FirstAlbum,
			Type:     "first_album_date",
			ArtistID: artist.ID,
		})
	}

	return trie
}
