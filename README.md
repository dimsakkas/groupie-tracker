📖 Overview

Groupie Tracker is a Go web application that showcases data about music artists and bands — members, formation year, first album, and their concert history on an interactive map.

The app consumes the Groupie Trackers API for artist data and the Nominatim (OpenStreetMap) API for geocoding concert locations. It has no frontend framework or build step — server-rendered HTML with vanilla JavaScript for live interactivity.

A Zone01 Athens project.

Live demo: https://groupie-tracker-p7gn.onrender.com

🚀 Getting Started

You need Go 1.25 or later. From the project root:

go run ./cmd/server

Then open http://localhost:8080. The server also respects the PORT environment variable, which hosting platforms such as Render set automatically. Stop the server with Ctrl+C.

The first startup geocodes every concert location through Nominatim, rate-limited to 1 request/second; this can take a few minutes the first time. A disk cache (data/geocache.json) makes every later startup near-instant.

✨ Features

Browse all artists in a responsive card grid, with image, formation year, and member count.
Filter live, with no page reload: formation year range, first album year range, number of members, and concert location as a three-level Country → State/Region → City tree.
Search live across artists, members, locations, and dates, with a suggestions dropdown as you type and a dedicated full-results search page.
Open an artist page for full details, members, chronological concert history, and a tour map.
Compare two artists side by side: location count, total shows, and tour distance in kilometres.
View a world map of every geocoded concert location across all artists at once.
Save favorites locally in the browser.
Fall back to a saved snapshot of the API data if the live Groupie Trackers API is unreachable at startup.
🧭 Using the App

Browse the artist grid on the home page, or use the filters panel to narrow it down.
Use the search bar in the header for live suggestions, or press Enter / select Search for full results.
Select an artist card to open its page, with members, concert history, and tour map.
From an artist page, select "Compare with" to see it side by side with another artist.
Select "Show world map" on the home page to see every concert location at once.
Select the heart icon on any card to save it as a favorite.
🔄 Application Flow

cmd/server/main.go orchestrates startup: fetch data, build the in-memory store and search index, start the HTTP server, then geocode locations in the background.
internal/fetch fetches artists, locations, dates, and relations from the API in parallel, and keeps a snapshot on disk as a fallback for when the live API is down.
internal/store combines the raw data into one view per artist and answers filter queries.
internal/search indexes artist names, members, locations, and dates in a prefix-search trie for live suggestions.
internal/geocode resolves each unique concert location to coordinates via Nominatim, through a worker pool with a shared rate limiter and a disk cache.
internal/handlers serves the HTML pages and the JSON API, reading from the store and the search index — never calling either external API directly.
The frontend (web/static/js) talks to the JSON API for live filtering, search suggestions, favorites, and the maps (rendered with Leaflet).
📁 Project Structure
```
groupie-tracker/
├── cmd/
│   └── server/              # main.go: startup and wiring
├── internal/
│   ├── fetch/                # Groupie Trackers API client, snapshot fallback
│   ├── store/                 # combined per-artist data, filters
│   ├── search/                 # prefix-search trie
│   ├── geocode/                 # Nominatim client, worker pool, disk cache
│   ├── handlers/                 # HTTP routes, templates, JSON API
│   └── models/                    # shared data types
├── web/
│   ├── templates/             # HTML pages (html/template)
│   └── static/
│       ├── css/
│       └── js/
├── data/
│   ├── upstream.json          # backup copy of the API data
│   └── geocache.json           # cache of geocoded coordinates
├── docs/
│   ├── PRD.md                  # requirements and architecture decisions
│   └── TASK_CARDS.md            # task breakdown
└── README.md
```
🛠 Technical Details

Language: Go 1.25, standard library only (net/http, html/template)
Data sources: Groupie Trackers API (artists, locations, dates, relations) and Nominatim / OpenStreetMap (geocoding)
Concurrency: the four API endpoints are fetched in parallel; geocoding runs through a worker pool with a shared rate limiter (1 request/second, as required by Nominatim's usage policy)
Resilience: a disk snapshot (data/upstream.json) is used if the live API is unreachable or returns no artists at startup; a disk cache (data/geocache.json) avoids re-geocoding on every startup
Frontend: vanilla JavaScript, no framework or build step; Leaflet for maps; favorites stored in localStorage
Tests: every external dependency (both APIs) is replaced with mock HTTP servers, so the test suite needs no internet connection

HTTP API

| Route | Description |
|---|---|
| GET / | Home page, artist list |
| GET /artist/{id} | A single artist's page |
| GET /search?q= | Full search results |
| GET /compare?a=ID&b=ID | Compare two artists |
| GET /api/suggest?q= | JSON, live search suggestions |
| GET /api/filter?... | JSON, IDs of the artists matching the filters |
| GET /api/artist/{id}/geo | JSON, geocoded locations for one artist |
| GET /api/geo | JSON, all geocoded locations combined |

✅ Tests

go build ./...
go vet ./...
go test ./...
go test -race ./...

The app depends on the Groupie Trackers API and on Nominatim for anything beyond what is already cached, so the data shown — and how long the first geocoding pass takes — can vary with their availability.

📚 Project Documentation

Product requirements: docs/PRD.md
Task breakdown: docs/TASK_CARDS.md

Deploy

Runs on Render, free tier. Build command: go build -o server ./cmd/server. Start command: ./server. The free tier spins the server down after 15 minutes of inactivity (next request: ~30–60 second cold start); because its disk is ephemeral, data/upstream.json and the already-populated data/geocache.json stay committed to the repo so every redeploy starts ready to go.

Team

Zone01 Athens — dsakkas, npanagopoul, dpavaloa