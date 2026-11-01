# Groupie Tracker

A Go web application that turns the Groupie Trackers REST API (bands, concert
locations, dates and their relations) into a searchable, filterable website
with an interactive concert map. Built for the Zone01 curriculum as the
fully-featured version: the core subject plus all four optional subjects
(search bar, geolocalization, filters, visualizations).

<!-- screenshot

---

## Description

The server plays both roles of the client-server model. At startup it is a
**REST client**: it fetches the four API endpoints in parallel, joins them into
one unified artist model and keeps everything in memory. At runtime it is a
**REST server**: it renders HTML pages and answers small JSON requests that the
browser's JavaScript makes while the user types, filters or opens a map.

The backend uses the Go standard library only (`go.mod` has no `require`).
The one external dependency is [Leaflet](https://leafletjs.com) in the browser,
loaded from a CDN only when a map is shown.

---

## Features

**Core**
- Artist cards with image, name, creation year and number of members
- Artist page: members, first album, every concert location with its dates
  (oldest first)
- Custom 400, 404 and 500 pages that share the site layout; a panic in any
  handler becomes a 500 page and the server keeps running

**Search bar**
- Live suggestions while typing (debounced 250 ms), each one labeled by type:
  artist, member, location, creation year, first album date
- Case-insensitive, prefix based, backed by a trie; multi-word queries work
  (`billie joe`, `london-uk`, `05-08-1967`)
- Keyboard: `/` focuses the box, arrows move, Enter opens, Esc closes
- The search form is a plain `GET /search`, so full results also work without
  JavaScript

**Filters** (results update while you change them, without a page reload)
- Range: creation year, first album year
- Checkbox: number of members, concert locations grouped by country
  (ticking a country selects all its cities)
- Filters combine with AND; several selected locations combine with OR
- The filters live in the URL, so the back button restores them; "Clear all"
  resets everything

**Geolocalization**
- Leaflet map with OpenStreetMap tiles and attribution
- Numbered markers in chronological order, joined by a polyline (the tour path)
- Locations are geocoded with Nominatim in the background, so the server is
  available immediately; the map shows a message and retries until the
  coordinates are ready

**Visualizations**
- Consistent palette, spacing and typography; text/background pairs meet
  WCAG AA contrast (checked with axe-core)
- Responsive from 320 px to 1440 px and wider
- Shneiderman's rules: consistent layout, a `/` shortcut, loading indicators,
  reversible actions (clear filters, back button)

**Extras**
- **Tour distance**: total kilometres of the tour (haversine, straight lines)
- **Play tour**: an animated marker travels along the tour in order
  (respects `prefers-reduced-motion`)
- **Favorites**: a heart button on every card, saved in the browser, with a
  "favorites only" filter
- **World map**: every concert location of the artists that pass the current
  filters
- **Compare**: two artists side by side (`/compare`)

---

## Requirements

- Go 1.24 or later (routes use the Go 1.22+ `GET /{id}` patterns)
- An internet connection for the first start (the API, Nominatim, map tiles)
- No external Go dependencies, standard library only

---

## Installation & Usage

```bash
git clone https://platform.zone01.gr/git/dsakkas/groupie-tracker.git
cd groupie-tracker

go run ./cmd/server
```

Run it from the repository root, because it reads `web/` and `data/` with
relative paths. Then open `http://localhost:8080`. Set the `PORT` environment
variable to use another port, for example `PORT=9090 go run ./cmd/server`.

**First start:** the pages work at once, but geocoding the ~190 distinct
locations takes a few minutes (Nominatim allows one request per second). Until
it finishes, maps show "Looking up concert locations…" and keep checking. The
results are saved in `data/geocache.json`, so every later start has its
coordinates immediately.

---

## How It Works

1. **Fetch** — four goroutines call `/artists`, `/locations`, `/dates` and
   `/relation` in parallel. A successful fetch is also saved to
   `data/upstream.json`; if the API cannot be reached at boot, that saved copy
   is used instead. With neither, the server exits with a clear error.
2. **Store** — the four payloads are joined into one `Artist` per band and put
   in lookup maps. After this step the store is only read, so handlers need no
   locks for the artist data.
3. **Search index** — a trie built once. Every word of every name, member,
   location and date is a key, so `japan` finds `saitama-japan`.
4. **Geocoding** (background) — a worker pool resolves each unique location with
   Nominatim, gated by a shared 1 request/second limiter, one retry after 2 s,
   then the location is skipped (one missing marker, never a crash). Results are
   written atomically to `data/geocache.json`; a corrupted cache is deleted and
   rebuilt. The only shared mutable state, the geocoded coordinates, sits behind
   a `sync.RWMutex`.
5. **Serve** — handlers read from memory and render a template or JSON.
   Pages are rendered into a buffer first, so a template error can never send a
   half-written page.

---

## HTTP API

Every route is `GET`; the application is read-only.

**Pages**

| Route | Description | Success | Error |
|-------|-------------|---------|-------|
| `/` | Home: cards, filters, world map | 200 | 500 |
| `/artist/{id}` | Artist page with members, concerts, map | 200 | 400 (bad id) / 404 / 500 |
| `/search?q=...` | Full-page search results | 200 | 500 |
| `/compare?a=ID&b=ID` | Two artists side by side | 200 | 400 / 404 / 500 |
| `/static/*` | CSS and JavaScript | 200 | 404 |
| anything else | Custom 404 page | — | 404 |

**JSON** (errors are JSON too: `{"error": "..."}`)

| Route | Description |
|-------|-------------|
| `/api/suggest?q=...` | Up to 10 typed suggestions: `[{"value","type","context","artist_id"}]` |
| `/api/filter?...` | `{"artist_ids":[...],"total":N}` |
| `/api/artist/{id}/geo` | `{"ready":bool,"coordinates":[...],"distance_km":N}`, chronological |
| `/api/geo` | `{"ready":bool,"locations":[...]}` with the artists at each location |

`/api/filter` parameters: `creation_min`, `creation_max`, `first_album_min`,
`first_album_max` (whole numbers), `members=3,5` (comma separated) and a
repeatable `location` (for example `?location=texas-usa&location=georgia-usa`).
A missing parameter means "no filter"; a malformed one returns 400.

---

## Error Handling

| Code | When it happens |
|------|-----------------|
| 400 | Malformed artist id, malformed compare id, malformed filter parameter |
| 404 | Unknown artist, unknown route |
| 500 | Template failure, or a panic recovered by the middleware |

Failures of outside services never stop the server once it is running: a failed
geocode drops one marker, an unavailable map library leaves the concert list
readable, and a failed suggestion request leaves the form working.

---

## Testing

```bash
gofmt -l .                   # prints nothing when the code is formatted
go vet ./...
go test -race -cover ./...
```

Tests use `httptest` fake servers, so they need no network (a fake Nominatim, a
fake API, the real templates). `main.go` is intentionally untested: it only
wires the packages together.

---

## Project Structure

```
cmd/server/main.go          boot sequence and routes, no business logic
internal/
  fetch/                    concurrent API client + saved snapshot (upstream.json)
  models/                   shared structs
  store/                    combined dataset, lookup maps, filters, tour distance
  search/                   trie index and suggestion lookup
  geocode/                  Nominatim client, worker pool, rate limiter, disk cache
  handlers/                 page handlers, JSON API, error pages, panic recovery
web/
  templates/                layout + one template per page, 400/404/500
  static/css/               style.css, responsive.css
  static/js/                search, filters, favorites, map, worldmap
data/                       geocache.json and upstream.json, generated at runtime
```

---

## Design Notes

- **Fetch once, serve from memory.** The API is called at boot only; every
  request after that reads memory.
- **Geocoding is Nominatim only.** An earlier idea resolved locations from their
  text first (`texas-usa` is already a state), but it covered only 27 of 189
  locations, so it was dropped. Filters work on the raw location strings and do
  not depend on geocoding at all.
- **Nominatim request format.** The query parameters are sent as
  `format, limit, addressdetails, accept-language`. With another order,
  Nominatim answered with empty results for many locations; keep the order as
  it is.
- **Disk files are optimizations.** `data/geocache.json` can be deleted at any
  time; `data/upstream.json` only matters when the API is down.

---

## Known Limitations

- The server depends on the Groupie Trackers API at the first start. If the API
  is unreachable and there is no saved `data/upstream.json`, it cannot start.
- Artist images are served by the API host, so they do not load when that host
  is down.
- A few locations cannot be found by Nominatim (for example
  `willemstad-netherlands_antilles`, a country that no longer exists); they have
  no marker but still work in search and filters.
- Tour distance and the tour path are straight lines between locations, not
  routes.

---

## Authors

| Zone01 login | Responsibility |
|---|---|
| **dsakkas** | Data models · `fetch` · `geocode` · `store` · `search` packages |
| **npanagopoul** | HTTP server and routing · page handlers · JSON API handlers · error handlers and panic recovery |
| **dpavaloa** | HTML templates · CSS · JavaScript (autocomplete, live filtering, map and polyline) |

Tests and this README were written by the whole team.

## License

Built as part of the [Zone01](https://zone01.gr) curriculum.