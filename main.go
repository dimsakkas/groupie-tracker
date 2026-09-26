package main

import (
	"fmt"
	"log"
	"net/http"

	"groupie-tracker/internal/fetch"
	"groupie-tracker/internal/store"
)

func recoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("PANIC recovered: %v", rec)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func main() {
	data, err := fetch.FetchAll()
	if err != nil {
		log.Fatalf("boot failed: %v", err)
	}
	log.Printf("fetched %d artists, %d location entries, %d date entries, %d relation entries", len(data.Artists), len(data.Locations), len(data.Dates), len(data.Relations))

	s := store.Build(data)
	log.Printf("store built: %d artists indexed by ID", len(s.ArtistsByID))

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		for id, artist := range s.ArtistsByID {
			fmt.Fprintf(w, "ID %d: %+v\n\n", id, artist)
		}
	})
	log.Println("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", recoverMiddleware(mux)))
}
