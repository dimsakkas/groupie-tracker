package main

import (
	"fmt"
	"log"
	"net/http"

	"groupie-tracker/internal/fetch"
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

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Artists (%d): \n%+v\n\n", len(data.Artists), data.Artists)
		fmt.Fprintf(w, "Locations (%d): \n%+v\n\n", len(data.Locations), data.Locations)
		fmt.Fprintf(w, "Dates (%d): \n%+v\n\n", len(data.Dates), data.Dates)
		fmt.Fprintf(w, "Relations (%d): \n%+v\n\n", len(data.Relations), data.Relations)
	})

	log.Println("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
