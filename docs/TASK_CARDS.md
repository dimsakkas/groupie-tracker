# TASK CARDS — groupie-tracker
*Team: dsakkas, npanagopoul, dpavaloa | Zone01 Athens*
*Scope: core + filters + geolocalization + search bar + visualizations*

---

## TASK 00 — Repository Setup & Project Skeleton

**Owner:** dsakkas
**Depends on:** —

### What needs to be done
- Create empty repo `groupie-tracker` on the team's Git host
- Initialize Go module: `go mod init groupie-tracker` (Go 1.22+ for path-parameter routing)
- Create the folder skeleton:
  ```
  groupie-tracker/
  ├── main.go
  ├── go.mod
  ├── internal/
  │   ├── models/          # Go structs matching upstream API + our own types
  │   ├── client/          # Groupie Trackers API client
  │   ├── geocoding/       # Nominatim client + disk cache
  │   ├── store/           # In-memory data store + query helpers
  │   └── handlers/        # HTTP handlers (server role)
  ├── templates/           # html/template files
  ├── static/
  │   ├── css/
  │   ├── js/
  │   └── images/
  ├── data/                # geocache.json lives here (gitignored)
  └── README.md
  ```
- Add `.gitignore` (binaries, `.DS_Store`, IDE folders, **`data/geocache.json`**)
- First commit: skeleton only, `main.go` prints "hello" and exits

### Why the `data/` folder is gitignored
The geocache is generated from Nominatim calls. Committing it would create merge conflicts every time the team regenerates it, and would force us to keep it in sync with the upstream data. Fresh clones geocode from scratch (~3.5 min one-time), or the team can share the file out-of-band.

### No secrets required
Nominatim and Leaflet are keyless — nothing to hide. No `.env` file needed.

### Acceptance Criteria
- [ ] `go run .` prints "hello" and exits with code 0
- [ ] `go build ./...` succeeds
- [ ] `go.mod` has `go 1.22` or higher
- [ ] `.gitignore` excludes `data/geocache.json`
- [ ] Repo pushed to origin with the folder skeleton in place

---

## TASK 01 — Data Models

**Owner:** dsakkas
**Depends on:** TASK 00

### What needs to be done
In `internal/models/`, define structs. Draft (verify field names against the live API):

```go
// Upstream Groupie Trackers types
type Artist struct {
    ID           int      `json:"id"`
    Image        string   `json:"image"`
    Name         string   `json:"name"`
    Members      []string `json:"members"`
    CreationDate int      `json:"creationDate"`
    FirstAlbum   string   `json:"firstAlbum"`      // "DD-MM-YYYY"
    Locations    string   `json:"locations"`       // URL
    ConcertDates string   `json:"concertDates"`    // URL
    Relations    string   `json:"relations"`       // URL
}

type LocationsIndex struct {
    Index []struct {
        ID        int      `json:"id"`
        Locations []string `json:"locations"`      // e.g. "houston-usa"
    } `json:"index"`
}

type DatesIndex struct {
    Index []struct {
        ID    int      `json:"id"`
        Dates []string `json:"dates"`              // e.g. "*06-10-2019"
    } `json:"index"`
}

type RelationIndex struct {
    Index []struct {
        ID             int                 `json:"id"`
        DatesLocations map[string][]string `json:"datesLocations"`
    } `json:"index"`
}

// Our own domain types
type Coordinate struct {
    Lat, Lng      float64 `json:"lat,lng"`
    City          string  `json:"city"`          // "Houston"
    State         string  `json:"state"`         // "Texas" — for hierarchy match
    Country       string  `json:"country"`       // "USA"
    OriginalKey   string  `json:"original_key"`  // "houston-usa" — for lookups
}

type Suggestion struct {
    Text     string `json:"text"`      // "Phil Collins"
    Type     string `json:"type"`      // "artist" | "member" | "location" | "creation_date" | "first_album_date"
    Context  string `json:"context"`   // "Member of Genesis" — extra info shown in dropdown
    ArtistID int    `json:"artist_id"` // clicking navigates here
}

// Fully merged view for handlers/templates
type ArtistView struct {
    Artist
    ResolvedLocations []string
    ResolvedDates     []string
    DatesLocations    map[string][]string
    Coordinates       []Coordinate // sorted chronologically for the tour path
}
```

### Why the `OriginalKey` field on Coordinate
Nominatim's response normalizes "houston-usa" → "Houston, TX, USA". We need to keep the *original* form to match against artist locations later (the relation endpoint uses the un-normalized form).

### Acceptance Criteria
- [ ] All 4 upstream schemas represented as Go structs
- [ ] Every field has a `json:` tag matching the exact upstream key
- [ ] `Coordinate`, `Suggestion`, `ArtistView` defined
- [ ] `go build ./...` succeeds
- [ ] `models_test.go` exists with a round-trip Marshal/Unmarshal test for each struct

---

## TASK 02 — Groupie Trackers API Client (concurrent fetch)

**Owner:** dsakkas
**Depends on:** TASK 01

### What needs to be done
In `internal/client/`, implement `FetchArtists`, `FetchLocations`, `FetchDates`, `FetchRelations`, plus a `FetchAll` orchestrator that runs them **concurrently**:

```go
func FetchAll() (*RawData, error) {
    var (
        wg   sync.WaitGroup
        data RawData
        errs = make(chan error, 4)
    )

    wg.Add(4)
    go fetchInto(&wg, errs, FetchArtists,   &data.Artists)
    go fetchInto(&wg, errs, FetchLocations, &data.Locations)
    go fetchInto(&wg, errs, FetchDates,     &data.Dates)
    go fetchInto(&wg, errs, FetchRelations, &data.Relations)
    wg.Wait()
    close(errs)

    for err := range errs {
        if err != nil {
            return nil, fmt.Errorf("upstream fetch failed: %w", err)
        }
    }
    return &data, nil
}
```

Each fetch uses `http.Get`, checks `resp.StatusCode == 200`, defers `resp.Body.Close()`, and `json.Unmarshal`s.

### Why concurrent + `WaitGroup` + channel
- Sequential: 4 × ~500ms = 2s. Concurrent: max(4 × ~500ms) ≈ 500ms.
- `WaitGroup` waits for all to finish. Buffered error channel of size 4 means no goroutine blocks on send. Reading errors after `close` gives us all failures without races.
- Pockets the async bonus in the audit.

### Acceptance Criteria
- [ ] Each of the 4 endpoints has a dedicated `Fetch*` function
- [ ] `FetchAll` uses goroutines + `sync.WaitGroup` (not sequential)
- [ ] If *any* upstream call fails, `FetchAll` returns an error (does not return partial data)
- [ ] `resp.Body.Close()` is deferred in every fetch
- [ ] `client_test.go` uses `httptest.NewServer` to mock upstream, verifies (a) all four called, (b) errors propagate, (c) `-race` clean

---

## TASK 03 — Nominatim Geocoding Client + Disk Cache

**Owner:** dsakkas
**Depends on:** TASK 02

### What needs to be done
In `internal/geocoding/`, wrap the Nominatim API with a disk-backed cache and strict rate limiting.

```go
const (
    NominatimURL = "https://nominatim.openstreetmap.org/search"
    UserAgent    = "groupie-tracker-zone01-project/1.0"
    RateLimit    = 1 * time.Second // Nominatim fair-use: max 1 req/sec
)

type Client struct {
    cachePath string
    cache     map[string]models.Coordinate
    mu        sync.RWMutex
    lastCall  time.Time
    callMu    sync.Mutex
}

func NewClient(cachePath string) *Client { ... }

// Load reads geocache.json if it exists.
func (c *Client) Load() error { ... }

// Save writes the current cache to disk atomically (write to tmp, rename).
func (c *Client) Save() error { ... }

// Geocode returns the coordinate for a location, hitting cache first.
func (c *Client) Geocode(location string) (models.Coordinate, error) {
    c.mu.RLock()
    if hit, ok := c.cache[location]; ok {
        c.mu.RUnlock()
        return hit, nil
    }
    c.mu.RUnlock()

    // Miss — respect rate limit before hitting Nominatim
    c.callMu.Lock()
    if elapsed := time.Since(c.lastCall); elapsed < RateLimit {
        time.Sleep(RateLimit - elapsed)
    }
    c.lastCall = time.Now()
    c.callMu.Unlock()

    coord, err := c.callNominatim(location)
    if err != nil {
        return models.Coordinate{}, err
    }

    c.mu.Lock()
    c.cache[location] = coord
    c.mu.Unlock()
    return coord, nil
}

// GeocodeAll processes locations sequentially (rate-limited).
func (c *Client) GeocodeAll(locations []string) map[string]models.Coordinate { ... }

func (c *Client) callNominatim(location string) (models.Coordinate, error) {
    url := fmt.Sprintf("%s?q=%s&format=json&addressdetails=1&limit=1",
        NominatimURL, url.QueryEscape(location))

    req, _ := http.NewRequest("GET", url, nil)
    req.Header.Set("User-Agent", UserAgent) // REQUIRED by Nominatim policy

    resp, err := http.DefaultClient.Do(req)
    if err != nil { return models.Coordinate{}, err }
    defer resp.Body.Close()

    var results []struct {
        Lat, Lon string
        Address  struct {
            City, State, Country string
        } `json:"address"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
        return models.Coordinate{}, err
    }
    if len(results) == 0 {
        return models.Coordinate{}, fmt.Errorf("no results for %q", location)
    }

    lat, _ := strconv.ParseFloat(results[0].Lat, 64)
    lng, _ := strconv.ParseFloat(results[0].Lon, 64)
    return models.Coordinate{
        Lat:         lat,
        Lng:         lng,
        City:        results[0].Address.City,
        State:       results[0].Address.State,
        Country:     results[0].Address.Country,
        OriginalKey: location,
    }, nil
}
```

### Why the User-Agent header is required
Nominatim's usage policy **mandates** identifying user-agents. Requests without one may be rate-limited more aggressively or blocked outright. This is not optional.

### Why atomic file writes
If the process dies mid-write, a partial JSON file corrupts the cache. Write to `geocache.json.tmp` and `os.Rename` — rename is atomic on POSIX filesystems.

### Why `sync.RWMutex`
Reads (cache hits) are frequent — `RLock` allows many readers concurrently. Writes (cache misses) are rare and use exclusive `Lock`.

### Why the 1-second rate limit
Nominatim's fair-use policy: max 1 request per second per client. This is enforced serverside; violations get you IP-banned. The `callMu` mutex + `time.Sleep` ensures we never exceed it, even from multiple goroutines.

### Cold-start expectations
- ~200 unique locations × 1 sec/request = **~3.5 minutes** first-time
- All subsequent starts: **~50ms** from disk cache
- If a specific location fails to geocode, we log a warning and skip — that marker just won't appear on the map, but the rest of the site works

### Acceptance Criteria
- [ ] `Load` reads from disk if `data/geocache.json` exists, returns empty map otherwise
- [ ] `Save` writes atomically (tmp + rename)
- [ ] `Geocode` returns cache hits without hitting Nominatim
- [ ] `Geocode` misses call Nominatim with correct User-Agent header
- [ ] Rate limit enforced: two back-to-back cache-miss geocodes are ≥1 second apart
- [ ] Failed geocodes log a warning but don't crash
- [ ] `geocoding_test.go` uses `httptest.NewServer` mocking Nominatim, verifies (a) cache hit path, (b) miss path writes to cache, (c) User-Agent sent, (d) rate limit enforced, (e) `Save` and `Load` roundtrip preserves data

---

## TASK 04 — In-Memory Store + Query Helpers

**Owner:** dsakkas
**Depends on:** TASK 02, TASK 03

### What needs to be done
In `internal/store/`, wrap raw data + geocoded data and expose the query API the handlers need:

```go
type Store struct {
    Artists           []models.Artist
    LocationsByArtist map[int][]string             // id → [locations]
    DatesByArtist     map[int][]string             // id → [dates]
    RelationsByArtist map[int]map[string][]string  // id → {location: [dates]}
    Coordinates       map[string]models.Coordinate // location → coord
    Suggestions       []models.Suggestion          // pre-built index for autocomplete
}

func Build(raw *client.RawData, coords map[string]models.Coordinate) *Store { ... }

// Query methods used by handlers:
func (s *Store) ArtistByID(id int) (*models.ArtistView, bool)
func (s *Store) Search(q string) []models.Artist
func (s *Store) Suggest(q string, limit int) []models.Suggestion
func (s *Store) Filter(f FilterParams) []int  // returns matching artist IDs
```

**`FilterParams` struct:**
```go
type FilterParams struct {
    CreationMin, CreationMax     int      // 0 = no bound
    FirstAlbumMin, FirstAlbumMax int      // year
    Members                      []int    // empty = any
    Locations                    []string // ["texas-usa", "uk"] — hierarchical, empty = any
}
```

**Location hierarchy matching:**
When filter includes `"texas-usa"`:
- Match if artist has any location whose `Coordinate.State == "Texas"` AND `Country == "USA"`
- When filter is `"uk"` (country only), match if `Country == "UK"` for any of the artist's locations

**Suggestion index build (once at Build time):**
For each artist, add rows for:
- name (Type: "artist", Context: "Since {year} · {members} members")
- each member (Type: "member", Context: "Member of {ArtistName}")
- each location (Type: "location", Context: "Concert location · {N} artists played here")
- creation date year (Type: "creation_date")
- first album date (Type: "first_album_date", both full "DD-MM-YYYY" and year forms)

Dedupe locations and dates (same value from multiple artists → one entry with aggregated context).

### Why pre-build the suggestion index
`Suggest(q)` becomes a simple `for _, s := range index { if strings.Contains(strings.ToLower(s.Text), q) { ... } }` — microseconds, no allocations per keystroke.

### Acceptance Criteria
- [ ] `Build` produces a store with all fields populated
- [ ] `ArtistByID` returns merged `ArtistView` with sorted chronological coordinates
- [ ] `Search("queen")` returns both Queen (artist) and any Queensland location matches — case-insensitive
- [ ] `Suggest("phil", 10)` returns both "Phil Collins - artist" and "Phil Collins - member"
- [ ] `Filter` with `Locations=["texas-usa"]` matches artists who played in Houston, Dallas, etc.
- [ ] `Filter` with combined params (creation range + member count) applies AND logic
- [ ] `store_test.go` covers every audit filter scenario (see PRD Section 7)

---

## TASK 05 — HTTP Server, Routing, Static Files

**Owner:** npanagopoul
**Depends on:** TASK 04

### What needs to be done
Wire the server in `main.go` after startup finishes:

```go
mux := http.NewServeMux()

fs := http.FileServer(http.Dir("static"))
mux.Handle("/static/", http.StripPrefix("/static/", fs))

// Page routes
mux.HandleFunc("GET /{$}",           handlers.Home(store))
mux.HandleFunc("GET /artist/{id}",   handlers.ArtistDetail(store))
mux.HandleFunc("GET /search",        handlers.Search(store))

// JSON API routes (async client-server events)
mux.HandleFunc("GET /api/suggest",           handlers.Suggest(store))
mux.HandleFunc("GET /api/filter",            handlers.Filter(store))
mux.HandleFunc("GET /api/artist/{id}/geo",   handlers.Geo(store))

// Catch-all → 404
mux.HandleFunc("/", handlers.NotFound)

srv := &http.Server{Addr: ":8080", Handler: handlers.Recover(mux)}
log.Println("listening on :8080")
if err := srv.ListenAndServe(); err != nil {
    log.Fatal(err)
}
```

### Why `GET /{$}` vs `GET /`
`/{$}` matches only the literal `/`. `/` is the catch-all — matches every unmatched path, so it goes to `NotFound`.

### Why `handlers.Recover` wraps the mux
See TASK 08. Turns panics from any handler into 500 responses instead of crashing the goroutine and leaving the client hanging.

### Acceptance Criteria
- [ ] Server starts on `:8080` after startup completes
- [ ] `/static/css/style.css` serves the file when present
- [ ] Every route registered with `GET` method
- [ ] Unknown paths route to `NotFound`
- [ ] Server won't start if data fetch or geocoding load failed

---

## TASK 06 — Page Handlers (home, artist detail, search results)

**Owner:** npanagopoul
**Depends on:** TASK 05

### What needs to be done

Three HTML handlers. All render templates with `html/template` and rely on the panic-recover safety net.

**Home** — cards grid + search input + filters button + concert calendar section:
```go
func Home(store *Store) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        renderPage(w, r, "home.html", struct {
            Artists []models.Artist
            Total   int
        }{store.Artists, len(store.Artists)})
    }
}
```

**ArtistDetail** — parses `{id}`, fetches merged view, renders detail page. No API key needed (Leaflet needs none):
```go
func ArtistDetail(store *Store) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        id, err := strconv.Atoi(r.PathValue("id"))
        if err != nil { http.Error(w, "invalid id", http.StatusBadRequest); return }
        view, ok := store.ArtistByID(id)
        if !ok { NotFound(w, r); return }
        renderPage(w, r, "artist.html", view)
    }
}
```

**Search** — full-page results (no-JS fallback):
```go
func Search(store *Store) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        q := strings.TrimSpace(r.URL.Query().Get("q"))
        results := store.Search(q)
        renderPage(w, r, "search.html", struct {
            Query   string
            Results []models.Artist
        }{q, results})
    }
}
```

### Why no key threading through handlers
Leaflet + OSM don't require an API key. The template loads Leaflet from unpkg CDN directly. Nothing to secure, nothing to pass through the request lifecycle.

### Why `renderPage` helper
DRY: template parsing, error handling, and `WriteHeader` sequencing should live in one place.

### Acceptance Criteria
- [ ] `/` renders the home page with cards for every artist
- [ ] `/artist/1` renders Queen with all 7 members, correct firstAlbum, correct locations
- [ ] `/artist/9999` renders 404 page (via handler, not raw text)
- [ ] `/artist/abc` returns 400
- [ ] `/search?q=queen` renders results including both "Queen" and location matches

---

## TASK 07 — JSON API Handlers (async client-server events)

**Owner:** npanagopoul
**Depends on:** TASK 05

### What needs to be done

Three JSON endpoints. All set `Content-Type: application/json` and use `json.NewEncoder(w).Encode(...)`.

**Suggest** — autocomplete:
```go
func Suggest(store *Store) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        q := strings.TrimSpace(r.URL.Query().Get("q"))
        if len(q) < 1 {
            writeJSON(w, []models.Suggestion{})
            return
        }
        writeJSON(w, store.Suggest(q, 10))
    }
}
```

**Filter** — parse query params into `FilterParams`, run filter, return matching IDs:
```go
func Filter(store *Store) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        params, err := parseFilterParams(r.URL.Query())
        if err != nil {
            http.Error(w, err.Error(), http.StatusBadRequest)
            return
        }
        ids := store.Filter(params)
        writeJSON(w, struct {
            ArtistIDs []int `json:"artist_ids"`
            Total     int   `json:"total"`
        }{ids, len(ids)})
    }
}
```

`parseFilterParams` extracts:
- `creation_min`, `creation_max`, `first_album_min`, `first_album_max` → int
- `members` → comma-separated list of ints
- `location` → repeated query param (`?location=uk&location=texas-usa`)

**Geo** — coordinates for one artist's concerts, sorted chronologically:
```go
func Geo(store *Store) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        id, err := strconv.Atoi(r.PathValue("id"))
        if err != nil { http.Error(w, "invalid id", http.StatusBadRequest); return }
        view, ok := store.ArtistByID(id)
        if !ok { http.NotFound(w, r); return }
        writeJSON(w, view.Coordinates) // already sorted by date
    }
}
```

### Why IDs (not full artists) in the filter response
Client already has all artists rendered in the DOM. Sending just IDs is dramatically smaller (~100 bytes vs ~50KB). Client uses `.querySelectorAll('.card')` + the ID list to hide non-matching cards.

### Why suggestions capped at 10
Dropdown UX: more than 10 is scroll-hell. Server-side limit avoids unbounded response sizes.

### Acceptance Criteria
- [ ] `GET /api/suggest?q=phil` returns Phil Collins as both artist and member
- [ ] `GET /api/suggest?q=` returns empty array (not error)
- [ ] `GET /api/filter?creation_min=1995&creation_max=2000` returns the audit's 9 artists
- [ ] `GET /api/filter?members=6` returns Pink Floyd, Arctic Monkeys, Linkin Park, Foo Fighters
- [ ] `GET /api/filter?location=texas-usa` returns R3HAB, Logic, Joyner Lucas, Twenty One Pilots
- [ ] `GET /api/artist/1/geo` returns Queen's 8 coordinates sorted by date
- [ ] All endpoints return valid JSON, always (even on error, use `http.Error`)
- [ ] `handlers_test.go` uses `httptest.NewRecorder` to test each

---

## TASK 08 — Error Handlers (404 & 500) + Panic Recovery

**Owner:** npanagopoul
**Depends on:** TASK 05

### What needs to be done

```go
func NotFound(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusNotFound)
    tmpl := template.Must(template.ParseFiles("templates/layout.html", "templates/404.html"))
    _ = tmpl.Execute(w, nil)
}

func InternalError(w http.ResponseWriter, r *http.Request, err error) {
    log.Printf("500 %s %s: %v", r.Method, r.URL.Path, err)
    w.WriteHeader(http.StatusInternalServerError)
    tmpl := template.Must(template.ParseFiles("templates/layout.html", "templates/500.html"))
    _ = tmpl.Execute(w, nil)
}

func Recover(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        defer func() {
            if rec := recover(); rec != nil {
                InternalError(w, r, fmt.Errorf("panic: %v", rec))
            }
        }()
        next.ServeHTTP(w, r)
    })
}
```

### Why WriteHeader before Execute
Once you write to the body, Go locks the status at 200. Set it explicitly first to send real 404/500.

### Acceptance Criteria
- [ ] `/nonexistent` returns 404 with custom page
- [ ] A handler that panics returns 500 (via Recover)
- [ ] Server stays alive after a panic
- [ ] Every 500 logged with method, path, error

---

## TASK 09 — HTML Templates

**Owner:** dpavaloa
**Depends on:** TASK 01 (data shapes)

### What needs to be done
Create in `templates/`:

- `layout.html` — base HTML head/nav/footer + `{{block "content" .}}{{end}}`
- `home.html` — search bar + filters button + concert calendar + cards grid
- `artist.html` — hero image + info + members + concert list + Leaflet map container
- `search.html` — search input + results grid
- `404.html` — friendly not-found
- `500.html` — friendly server error

**Artist detail template — the Leaflet integration point:**
```html
{{define "content"}}
<article class="artist-detail">
  <!-- ... hero, members, etc ... -->
  <section id="concert-map-section">
    <h2>Concert map</h2>
    <div id="concert-map" data-artist-id="{{.ID}}" style="height: 400px;"></div>
  </section>
</article>

<link rel="stylesheet"
      href="https://unpkg.com/leaflet@1.9.4/dist/leaflet.css"
      integrity="sha256-p4NxAoJBhIIN+hmNHrzRCf9tD/miZyoHS5obTRR9BMY="
      crossorigin="" />
<script src="https://unpkg.com/leaflet@1.9.4/dist/leaflet.js"
        integrity="sha256-20nQCchB9co0qIjJZRGuk2/Z9VM+kNiyxNV1lvTlZBo="
        crossorigin=""></script>
<script src="/static/js/map.js"></script>
{{end}}
```

### Why `html/template` (not `text/template`)
Automatic contextual escaping — a hypothetical band name with `<script>` in it gets rendered safely as text, not executed.

### Why Subresource Integrity (SRI) on Leaflet
The `integrity` attribute pins the exact hash of the file we expect. If unpkg gets compromised and serves malicious JS, the browser refuses to execute it. Standard practice for external scripts. Get the current SRI hashes from `leafletjs.com/download.html`.

### Why load Leaflet only on the detail page
No point loading ~150KB of JS + CSS on the home page where there's no map. Loading it inside the `{{define "content"}}` block of `artist.html` means home.html never pulls it in.

### Acceptance Criteria
- [ ] Layout renders consistently across all pages
- [ ] Home cards link to `/artist/{{.ID}}`
- [ ] Artist detail shows all 4 data types (info, members, locations, relations)
- [ ] Map container `<div id="concert-map">` present on detail page
- [ ] Leaflet CSS + JS loaded only on detail page, not on home
- [ ] Leaflet loaded with `integrity` + `crossorigin` attributes (SRI)
- [ ] 404 / 500 use the same layout
- [ ] No `{{.Field}}` visible in rendered output (typo detection)

---

## TASK 10 — CSS (Schneiderman rules + responsive)

**Owner:** dpavaloa
**Depends on:** TASK 09

### What needs to be done
`static/css/style.css`:

- CSS reset (`box-sizing: border-box`, sensible defaults)
- CSS custom properties for palette, spacing, typography — one source of truth (**Schneiderman #1 — consistency**)
- Responsive grid (`grid-template-columns: repeat(auto-fill, minmax(200px, 1fr))`)
- Breakpoints at 640px and 1024px
- Custom slider styling (dual-range for creation/album date filters)
- Custom checkbox styling (pill and tree variants)
- Autocomplete dropdown positioned absolutely under the search bar
- Loading spinner (`.spinner` class) shown during async fetches (**Schneiderman #3 — feedback**)
- Focus rings visible on all interactive elements (`:focus-visible` — a11y)
- Custom 404 / 500 pages styled with same design language
- **Leaflet overrides**: the default Leaflet CSS uses different fonts and colors than our design. Override `.leaflet-popup-content`, `.leaflet-container`, and marker styles to match the site palette.

### Key CSS variables
```css
:root {
    --color-primary: /* pick */;
    --color-bg: /* pick */;
    --color-text: /* pick */;
    --color-border: /* pick */;
    --space-1: 4px;
    --space-2: 8px;
    --space-3: 16px;
    --space-4: 24px;
    --radius: 12px;
    --font-ui: system-ui, -apple-system, sans-serif;
    --font-mono: ui-monospace, monospace;
}
```

### Why one source of truth for palette/spacing
Change one variable, everything updates. This *is* Schneiderman #1 encoded in CSS.

### Acceptance Criteria
- [ ] Layout works at 320px width up through 1440px+ (no horizontal scroll)
- [ ] All text meets WCAG AA contrast
- [ ] Hover, focus, active states visible on all interactive elements
- [ ] Dual-range slider works visually (both handles independently draggable)
- [ ] Loading spinner appears during `/api/filter` and `/api/suggest` fetches
- [ ] Leaflet popup styling matches site design (not default Leaflet look)
- [ ] `Ctrl+F5` reload doesn't show flash of unstyled content

---

## TASK 11 — JS: Autocomplete Dropdown

**Owner:** dpavaloa
**Depends on:** TASK 07, TASK 10

### What needs to be done
`static/js/search.js` — debounced autocomplete:

```javascript
const input = document.querySelector('input[name="q"]')
const dropdown = document.getElementById('suggest-dropdown')
let timer
let activeIndex = -1

// Global shortcut: "/" focuses the search
document.addEventListener('keydown', (e) => {
    if (e.key === '/' && document.activeElement !== input) {
        e.preventDefault()
        input.focus()
    }
})

input.addEventListener('input', () => {
    clearTimeout(timer)
    const q = input.value.trim()
    if (!q) { dropdown.hidden = true; return }

    timer = setTimeout(async () => {
        const res = await fetch(`/api/suggest?q=${encodeURIComponent(q)}`)
        if (!res.ok) return
        const suggestions = await res.json()
        renderDropdown(suggestions)
    }, 250)
})

// Arrow keys navigate, Enter selects, Esc closes
input.addEventListener('keydown', (e) => {
    const items = dropdown.querySelectorAll('.suggest')
    if (e.key === 'ArrowDown') { activeIndex = (activeIndex + 1) % items.length; ... }
    else if (e.key === 'ArrowUp') { ... }
    else if (e.key === 'Enter' && activeIndex >= 0) { window.location = items[activeIndex].dataset.href }
    else if (e.key === 'Escape') { dropdown.hidden = true; activeIndex = -1 }
})

function renderDropdown(suggestions) {
    dropdown.innerHTML = suggestions.map(s => `
        <a class="suggest" data-href="/artist/${s.artist_id}">
            <div class="suggest-text">
                <div class="suggest-main">${escapeHtml(s.text)}</div>
                <div class="suggest-context">${escapeHtml(s.context)}</div>
            </div>
            <span class="suggest-type type-${s.type}">${s.type}</span>
        </a>
    `).join('')
    dropdown.hidden = suggestions.length === 0
}
```

### Why `escapeHtml`
User input flows through this template. Without escaping, a suggestion with `<script>` in it would execute. Small function, big protection.

### Why 250ms debounce
Sweet spot: feels instant to typing, but only fires once per pause. Reduces server load by ~80% during active typing.

### Acceptance Criteria
- [ ] Typing "phil" shows dropdown with Phil Collins × 2 (artist + member)
- [ ] Arrow keys highlight suggestions; Enter navigates; Esc closes
- [ ] `/` keyboard shortcut focuses search from anywhere
- [ ] Rapid typing does not fire per-keystroke requests (debounce)
- [ ] Search form still submits and works with JS disabled (form action `/search`)

---

## TASK 12 — JS: Live Filtering

**Owner:** dpavaloa
**Depends on:** TASK 07, TASK 10

### What needs to be done
`static/js/filter.js` — every filter change triggers a debounced fetch:

```javascript
const filtersPanel = document.getElementById('filters-panel')
const grid = document.getElementById('cards-grid')
const counter = document.getElementById('result-counter')
let timer

function currentFilterState() {
    return {
        creation_min: document.getElementById('creation-min').value,
        creation_max: document.getElementById('creation-max').value,
        first_album_min: /* ... */,
        first_album_max: /* ... */,
        members: [...document.querySelectorAll('.member-pill.active')].map(el => el.dataset.value),
        location: [...document.querySelectorAll('.location-cb:checked')].map(el => el.dataset.value),
    }
}

function buildQueryString(state) {
    const p = new URLSearchParams()
    if (state.creation_min) p.set('creation_min', state.creation_min)
    if (state.creation_max) p.set('creation_max', state.creation_max)
    state.members.forEach(m => p.append('members', m))
    state.location.forEach(l => p.append('location', l))
    return p.toString()
}

filtersPanel.addEventListener('input', () => {
    clearTimeout(timer)
    counter.textContent = 'Filtering...'
    timer = setTimeout(async () => {
        const qs = buildQueryString(currentFilterState())
        const res = await fetch(`/api/filter?${qs}`)
        if (!res.ok) { counter.textContent = 'Error'; return }
        const { artist_ids, total } = await res.json()
        applyFilterToGrid(new Set(artist_ids))
        counter.textContent = `${total} of ${window.TOTAL_ARTISTS} artists`
    }, 250)
})

function applyFilterToGrid(matchingIds) {
    grid.querySelectorAll('.card').forEach(card => {
        card.hidden = !matchingIds.has(Number(card.dataset.artistId))
    })
}
```

### Why hide/show existing DOM (not re-render)
Cards are already rendered by the server on page load. Hiding via `hidden` attribute is instant, no layout thrashing, no lost image loads. Just IDs on the wire.

### Why "Filtering..." placeholder
Schneiderman #3 — informative feedback. User knows something is happening.

### Acceptance Criteria
- [ ] Every filter change updates the counter and grid within ~350ms
- [ ] All audit filter combinations produce the correct results
- [ ] "Clear all" resets everything and shows all 52 artists
- [ ] Grid transitions are smooth (no full page reload)

---

## TASK 13 — JS: Leaflet Map Integration

**Owner:** dpavaloa
**Depends on:** TASK 07 (needs `/api/artist/{id}/geo`), TASK 09 (map container + Leaflet loaded)

### What needs to be done
`static/js/map.js` — loads coordinates and renders map with markers + polyline:

```javascript
async function initConcertMap() {
    const container = document.getElementById('concert-map')
    if (!container) return
    const artistId = container.dataset.artistId

    let coords
    try {
        const res = await fetch(`/api/artist/${artistId}/geo`)
        if (!res.ok) throw new Error(`HTTP ${res.status}`)
        coords = await res.json()   // sorted chronologically by server
    } catch (err) {
        container.textContent = 'Map unavailable'
        return
    }

    if (!coords || coords.length === 0) {
        container.textContent = 'No concert locations'
        return
    }

    // Initialize Leaflet map
    const map = L.map('concert-map', { zoomControl: true })

    // OSM tile layer with required attribution
    L.tileLayer('https://tile.openstreetmap.org/{z}/{x}/{y}.png', {
        attribution: '© <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors',
        maxZoom: 18,
    }).addTo(map)

    // Custom numbered marker icon
    const numberedIcon = (n) => L.divIcon({
        className: 'concert-marker',
        html: `<div class="concert-marker-inner">${n}</div>`,
        iconSize: [24, 24],
        iconAnchor: [12, 12],
    })

    // Place markers
    const path = []
    coords.forEach((c, i) => {
        L.marker([c.lat, c.lng], { icon: numberedIcon(i + 1) })
            .addTo(map)
            .bindPopup(`<strong>${i + 1}. ${c.city || c.original_key}</strong><br>${c.country || ''}`)
        path.push([c.lat, c.lng])
    })

    // Bonus: historical tour path polyline
    if (path.length > 1) {
        L.polyline(path, {
            color: '#185FA5',
            weight: 2,
            opacity: 0.7,
            dashArray: '6, 6',
        }).addTo(map)
    }

    // Auto-fit map to show all markers
    map.fitBounds(L.latLngBounds(path), { padding: [30, 30] })
}

// Kick off after DOM ready
document.addEventListener('DOMContentLoaded', initConcertMap)
```

### Why custom `divIcon` for numbered markers
Leaflet's default marker is a plain blue pin — no way to show numbers on it. `L.divIcon` lets us render any HTML inside a marker. The `.concert-marker` CSS class in style.css makes it look good.

### Why `map.fitBounds` and not a fixed zoom
Auto-zooms to fit all markers with padding. Better UX than a fixed world view (would zoom out too far for a US-only tour) or a fixed city view (would crop off distant stops).

### Graceful degradation
If `/api/artist/{id}/geo` returns 404 or the fetch fails, we show text "Map unavailable" instead of a broken map. If coords is empty (artist with no concerts), we show "No concert locations". Never a broken state.

### Acceptance Criteria
- [ ] Detail page for Queen shows 8 markers at the audit-required locations
- [ ] Markers are numbered 1..N reflecting chronological order
- [ ] Clicking a marker shows a popup with city + country
- [ ] A dashed polyline connects markers in order (bonus)
- [ ] Map auto-fits to the markers (not a fixed world view)
- [ ] OpenStreetMap attribution visible in the bottom-right of the map
- [ ] If geocoding failed for some locations, remaining markers still show
- [ ] Map absent (`container.textContent = 'No concert locations'`) if the artist has no locations, no crash

---

## TASK 14 — Unit Tests

**Owner:** TEAM (each writes tests for own files)
**Depends on:** the file being tested

### What needs to be done
Test file for every Go source file **except** `main.go`, all JS files, all CSS files.

**Required coverage:**
- `internal/models/models_test.go` — round-trip Marshal/Unmarshal
- `internal/client/client_test.go` — `httptest` mocked upstream, all 4 fetches, error propagation
- `internal/geocoding/geocoding_test.go` — `httptest` mocked Nominatim, cache load/save, hit/miss paths, User-Agent verified, rate limit verified
- `internal/store/store_test.go` — every audit filter scenario, `Search`, `Suggest`, `ArtistByID`
- `internal/handlers/handlers_test.go` — `httptest.NewRecorder` for each handler

### Every audit filter scenario as a test case
```go
func TestFilter_AuditScenarios(t *testing.T) {
    store := buildTestStore(t) // uses fixtures/*.json
    tests := []struct {
        name    string
        params  FilterParams
        wantIDs []int
    }{
        {"creation 1995-2000", FilterParams{CreationMin: 1995, CreationMax: 2000}, []int{/* SOJA, ... */}},
        {"exactly 6 members", FilterParams{Members: []int{6}}, []int{/* Pink Floyd, ... */}},
        // ... one per audit scenario
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) { ... })
    }
}
```

### Acceptance Criteria
- [ ] `go test ./...` passes with no failures
- [ ] `go test -race ./...` passes clean (no data races in concurrent fetch/geocode)
- [ ] Every audit filter scenario has a corresponding test case
- [ ] Tests use `httptest`, no live upstream calls
- [ ] Coverage report (`go test -cover`) shows ≥70% for `store` and `handlers` packages

---

## TASK 15 — Deployment to Fly.io

**Owner:** dsakkas
**Depends on:** TASK 14

### What needs to be done
- Install `flyctl`, `flyctl auth signup`
- `flyctl launch` from repo root — generates `fly.toml`
- Add `Dockerfile` (multi-stage Go build):
  ```dockerfile
  FROM golang:1.22-alpine AS build
  WORKDIR /app
  COPY . .
  RUN go build -o /out/server .

  FROM alpine:latest
  WORKDIR /app
  COPY --from=build /out/server .
  COPY templates/ ./templates/
  COPY static/ ./static/
  RUN mkdir -p data
  EXPOSE 8080
  CMD ["./server"]
  ```
- Configure persistent volume for `data/`:
  ```toml
  [[mounts]]
    source = "geocache_data"
    destination = "/app/data"
  ```
- Deploy: `flyctl deploy`
- Verify at `https://groupie-tracker.fly.dev`

### Why the persistent volume
`data/geocache.json` needs to survive restarts. Without a volume, every restart triggers a cold geocoding pass (~3.5 minutes of app being unavailable). With the volume, the file survives across deploys.

### No secrets to manage
Nominatim needs no API key, Leaflet needs no API key. No `flyctl secrets set` required.

### Acceptance Criteria
- [ ] Site reachable at public URL
- [ ] Leaflet map loads on detail pages
- [ ] Geocache persists across restarts (verify with `flyctl restart`)
- [ ] URL documented in `README.md`

---

## TASK 16 — README

**Owner:** TEAM
**Depends on:** TASK 15

### What needs to be done
`README.md` sections:
- Project name + one-line description
- Screenshots (home, detail with map, filter panel)
- Features (bulleted list of all 4 optionals + core)
- How it works (architecture diagram + concurrent fetch story)
- HTTP API table (both HTML and JSON routes)
- External dependencies:
  - Groupie Trackers API (upstream data)
  - Nominatim (geocoding, free, no key)
  - Leaflet + OpenStreetMap tiles (map rendering, free, no key)
- Local setup: `git clone`, `go run .`, browser to `localhost:8080`
- First-run note: cold-start geocoding takes ~3.5 minutes; subsequent starts are instant
- Deployment (link + how to redeploy)
- Testing: `go test ./...`, `go test -race ./...`, `go test -cover ./...`
- Project structure (folder tree)
- Authors table with responsibilities
- Public URL
- Attribution: "Map data © OpenStreetMap contributors"

### Acceptance Criteria
- [ ] A stranger can clone, run `go run .`, and see the site
- [ ] All routes documented
- [ ] Authors listed with responsibilities
- [ ] Public URL prominent at the top
- [ ] OSM attribution present in README
- [ ] No placeholder text left over

---

*Last updated: 2026-07-27 | Team: dsakkas, npanagopoul, dpavaloa*