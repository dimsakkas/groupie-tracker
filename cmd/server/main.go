package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"groupie-tracker/internal/fetch"
	"groupie-tracker/internal/geocode"
	"groupie-tracker/internal/handlers"
	"groupie-tracker/internal/search"
	"groupie-tracker/internal/store"
)

// main only orchestrates the boot sequence; the logic lives in the packages.
//
// Run it from the repository root (go run ./cmd/server) so the relative
// paths web/ and data/ resolve correctly.
func main() {
	// 1. Fetch everything once; without the core data there is nothing to serve.
	data, err := fetch.FetchAll()
	if err != nil {
		log.Fatalf("boot failed: %v", err)
	}
	log.Printf("fetched %d artists, %d location entries, %d date entries, %d relation entries",
		len(data.Artists), len(data.Locations), len(data.Dates), len(data.Relations))

	// 2. Build the in-memory store and the search index. After this point
	// both are only read, so request handlers can use them without locks.
	s := store.Build(data)
	log.Printf("store built: %d artists", len(s.Artists))
	trie := search.BuildIndex(s.Artists)
	srv := &handlers.Server{Store: s, Trie: trie}

	// 3. Geocode in the background so the server is available immediately.
	// Until it finishes, /api/artist/{id}/geo answers {"ready": false}.
	cache := geocode.LoadCache(filepath.Join("data", "geocache.json"))
	locations := s.UniqueLocations()
	go func() {
		start := time.Now()
		log.Printf("geocoding %d locations in the background", len(locations))
		results := geocode.GeocodeAll(locations, cache)
		s.SetGeo(results)
		log.Printf("geocoding finished: %d/%d locations resolved in %s",
			len(results), len(locations), time.Since(start).Round(time.Second))
	}()

	// 4. Routes.
	mux := http.NewServeMux()
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))
	mux.HandleFunc("GET /{$}", srv.HomeHandler)
	mux.HandleFunc("GET /api/suggest", srv.SuggestHandler)
	mux.HandleFunc("GET /api/filter", srv.FilterHandler)
	mux.HandleFunc("GET /api/artist/{id}/geo", srv.GeoHandler)
	mux.HandleFunc("/", handlers.NotFound) // catch-all: every unknown path

	// 5. Serve. Recover turns a panic in any handler into a 500 page; the
	// timeouts stop slow or stuck clients from holding connections forever.
	addr := ":8080"
	if port := os.Getenv("PORT"); port != "" {
		addr = ":" + port
	}
	httpServer := &http.Server{
		Addr:              addr,
		Handler:           handlers.Recover(mux),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	log.Printf("listening on %s", addr)
	log.Fatal(httpServer.ListenAndServe())
}
