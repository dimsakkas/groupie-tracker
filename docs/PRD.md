# PRD — groupie-tracker
*Team: dsakkas, npanagopoul, dpavaloa | Zone01 Athens*
*Scope: core + filters + geolocalization + search bar + visualizations (all 4 optionals bundled)*

---

## 1. What We're Building and Why

Groupie Tracker is a Go web application that consumes an external REST API (`groupietrackers.herokuapp.com/api`) and presents its data — bands, concert locations, dates, and their relations — through a user-friendly website. This is the team's first project where we play **both roles** of the client-server model: **REST client** (fetching from external APIs) *and* **REST server** (serving HTML and JSON).

We ship the **fully-featured version** — the core project plus all four optional subjects in one deliverable:

- **Core** — consume the API, display bands, one real client-server event
- **Search bar** — live typed suggestions across 5 fields, backed by a trie (O(k) lookup)
- **Geolocalization** — Leaflet + OSM tiles with concert markers + historical tour path; geocoding via hybrid token-first, Nominatim fallback
- **Filters** — 4 filters, 2 types (range + checkbox), async live updates
- **Visualizations** — Schneiderman-compliant UX, responsive, custom 404, consistent design

**Design goals (in priority order):**

1. **Correctness against all 5 audits** — every documented test case must pass
2. **Speed and memory discipline** — API fetched once at boot, everything served from memory afterward
3. **Above-junior architecture that stays learnable** — trie, worker pool, hybrid resolution — used where they solve real problems, not for their own sake
4. **Genuine concurrency** — goroutines and channels for parallel upstream fetch and rate-limited geocoding, not sprinkled cosmetically

Only Go standard library on the backend. Leaflet (client-side JS) is the only external dependency, and it doesn't violate the stdlib-only constraint since it's not a Go import.

---

## 2. Design Principles

- **Fetch once, serve from memory.** External API called once at boot. Every subsequent request reads from in-memory structures.
- **Build once, look up in O(1) or O(k).** Expensive structures (unified artist model, lookup maps, trie, geocode cache) built during boot.
- **Prefer information you already have over external calls.** Location hierarchy resolved from string tokens first (`"texas-usa"` is already the state); Nominatim is a fallback.
- **Never crash.** Every external dependency has a bounded failure path — one failed geocode drops one marker, not the site.
- **Disk cache is optimization, never dependency.** Server must run correctly even if `data/geocache.json` is missing, unreadable, or corrupted. Corruption auto-recovers by deleting and rebuilding.
- **Single responsibility per package** — with pragmatism. Fetching, search, geocoding, and handlers are separate packages. Store bundles data + queries + filtering because they share the same lookup structures and don't warrant separate packages at this project's size.
- **Concurrency where it solves problems.** Goroutines for parallel upstream fetch (real speedup) and rate-limited geocoding (real orchestration). Not sprinkled for the checkbox.

---

## 3. Architecture Overview

One Go binary. One boot sequence. One in-memory store, queried by everything downstream — HTTP handlers never talk to the upstream API or to Nominatim directly; they only read from the store, the trie, and the geocode cache.

```
Boot:
  fetch (concurrent, 4 goroutines)
    → combine into unified Artist model (via `relation`)
    → build lookup maps (store)
    → build search trie (search)
    → build geocode cache (geocode: token-first, Nominatim fallback,
      worker pool, rate limiter, bounded retry, disk cache with corruption recovery)
  → start HTTP server

Runtime (per request):
  handler → store / search / geocode (all in-memory reads)
  → render HTML or JSON
```

---

## 4. Boot Sequence — Detail

1. **Concurrent fetch.** Four goroutines call `/artists`, `/locations`, `/dates`, `/relation` in parallel with `sync.WaitGroup`. If any call fails, program logs a clear error and exits — no meaningful way to serve the site without the core dataset.

2. **Combine.** The four raw payloads are joined via `relation` into one `Artist` struct per band. This logic lives inside `internal/store/store.go` as the first step of store build — not in a separate package.

3. **Store build.** Lookup maps for O(1) access by ID and normalized name. Filter methods hang off the store for the same reason (see §7 Decisions Resolved).

4. **Trie build.** Every artist name, member, location, creation date, and first-album date tokenized (split on `-`, `_`, whitespace) and inserted. Each end-of-token node holds a **list** of `(value, type, artistID)` entries.

5. **Geocode cache build**, per unique location string:
   - **Try to load** `data/geocache.json` from disk. If exists and parses, its entries are used directly — no network calls for locations already present.
   - **If the file is missing, unreadable, or corrupted**, delete it and proceed as if no cache existed. Corrupted cache never blocks boot.
   - For every location still unresolved, run hybrid resolution via a worker pool with 1 req/sec rate limiter:
     - **Token layer first** (free, instant): if a token matches a known state/country, hierarchy resolved with zero external calls
     - **Nominatim fallback**: only when token layer insufficient. `addressdetails=1` returns coordinates + address breakdown in one call
     - **Bounded retry**: one retry after 2s delay on failure
     - **Graceful skip**: if second attempt fails, location skipped — no map marker, no participation in location filters, but the artist page, search, and everything else continue normally
     - A failing/slow location only blocks its own goroutine; the pool as a whole is unaffected
   - **Save** completed cache back to `data/geocache.json` atomically (temp file, then rename) so a crash mid-write never corrupts it
   - **Disk persistence is pure optimization**: if disk unavailable or any read/write fails, server continues normally using only in-memory cache for that run

6. **Server start.**

---

## 5. How We Split the Work

| Part | Owner | Reviewer |
|------|-------|----------|
| Data models (all structs) | dsakkas | npanagopoul |
| Fetch package (concurrent upstream client) | dsakkas | npanagopoul |
| Geocode package (worker pool + rate limiter + hybrid + cache) | dsakkas | dpavaloa |
| Store package (combine + lookup maps + filters) | dsakkas | npanagopoul |
| Search package (trie + tokenization) | dsakkas | npanagopoul |
| HTTP server + routing + static files | npanagopoul | dsakkas |
| Page handlers (home, artist, search) | npanagopoul | dpavaloa |
| JSON API handlers (suggest, filter, geo) | npanagopoul | dsakkas |
| Error handlers + panic recovery | npanagopoul | dpavaloa |
| HTML templates | dpavaloa | dsakkas |
| CSS (Schneiderman + responsive) | dpavaloa | npanagopoul |
| JS — autocomplete dropdown | dpavaloa | npanagopoul |
| JS — live filtering | dpavaloa | npanagopoul |
| JS — Leaflet map + polyline | dpavaloa | dsakkas |
| Unit tests | TEAM | TEAM |
| README | TEAM | TEAM |

Reviewer ≠ author. Every PR merges only after review from the listed reviewer.

---

## 6. Repository Structure

```
groupie-tracker/
├── cmd/
│   └── server/
│       └── main.go              # boot orchestration only, no business logic
│
├── internal/
│   ├── fetch/                   # concurrent calls to the 4 API endpoints
│   │   ├── client.go
│   │   └── client_test.go
│   │
│   ├── models/                  # shared structs: Artist, Location, Date, Relation, Coordinate, Suggestion
│   │   └── models.go
│   │
│   ├── store/                   # combine raw payloads, in-memory dataset, lookup maps, filters
│   │   ├── store.go             # combine + lookup maps + filter methods (single file)
│   │   └── store_test.go
│   │
│   ├── search/                  # trie index, tokenization, suggestion lookup
│   │   ├── trie.go
│   │   └── trie_test.go
│   │
│   ├── geocode/                 # token-first + Nominatim fallback, worker pool, rate limiter, retry
│   │   ├── geocode.go
│   │   ├── pool.go              # worker pool orchestration
│   │   ├── cache.go             # disk-persisted cache (load/save, atomic write, corruption recovery)
│   │   └── geocode_test.go
│   │
│   └── handlers/                # HTTP routing, error handling
│       ├── home.go
│       ├── artist.go
│       ├── search.go
│       ├── filter.go
│       ├── geo.go
│       ├── errors.go
│       └── handlers_test.go
│
├── web/
│   ├── templates/
│   │   ├── layout.html
│   │   ├── index.html
│   │   ├── artist.html
│   │   ├── 400.html
│   │   ├── 404.html
│   │   └── 500.html
│   └── static/
│       ├── css/
│       │   ├── style.css        # palette, contrast, Schneiderman rules
│       │   └── responsive.css
│       └── js/
│           ├── search.js        # live suggestions
│           ├── filters.js       # async filter updates
│           └── map.js           # Leaflet, markers, chronological polyline
│
├── data/
│   └── geocache.json            # generated at boot; gitignored, never committed
│
├── docs/
│   ├── PRD.md
│   └── TASK_CARDS.md
│
├── go.mod
├── .gitignore
└── README.md
```

**Notes:**
- No `go.sum` — zero external Go dependencies
- `data/geocache.json` generated automatically at boot, excluded via `.gitignore`
- `internal/` prevents these packages from being imported by other Go modules — reinforces boundaries
- Filter methods live in the same `store.go` file — the whole store package is small enough (~300 lines) that splitting into multiple files would add noise without benefit

---

## 7. Ideas We Can Use

Decisions made and rationale.

**Upstream fetching:**
- **Fetch once at boot, concurrent goroutines** — chosen: fast, respectful, pockets async bonus
- ~~Fetch per request~~ — rejected: wasteful, slow, hammers upstream

**Search backing structure:**
- **Trie with token-splitting** — chosen: O(k) prefix lookup; token-splitting means `"japan"` matches `"saitama-japan"`; list-per-node handles duplicates (`"Phil Collins"` = both member and artist)
- ~~Linear scan~~ — rejected: O(n) per keystroke, misses the learning opportunity

**Geocoding strategy:**
- **Hybrid: token-first, Nominatim fallback** — chosen: many locations already contain their state (`"texas-usa"`), skip API call entirely; only cities that need it hit Nominatim
- ~~Always Nominatim~~ — rejected: wasteful, slower cold start
- ~~Google Maps~~ — rejected: requires credit card

**Nominatim orchestration:**
- **Worker pool + shared rate limiter** — workers pull from channel-based queue; `time.Ticker` gates all workers to 1 req/sec total
- **Bounded retry**: 1 retry after 2s, then graceful skip
- **User-Agent header** — required: `groupie-tracker-zone01-project/1.0`

**Disk cache:**
- **`data/geocache.json` with atomic writes (tmp + rename)** — chosen: subsequent starts skip geocoding entirely
- **Corruption recovery**: on load failure, delete file and rebuild from scratch — cache never blocks boot
- **In-memory fallback**: if disk unavailable for any reason, server continues normally
- **Disk cache is optimization, not dependency** — this principle drives all failure paths above

**Location hierarchy for filter (`"Texas, USA"` matches all Texas cities):**
- **Store city, state, country alongside each coordinate** — chosen: hierarchy match becomes simple field comparison

**Filter API contract:**
- **`GET /api/filter?creation_min=X&members=1,2&location=texas-usa`** — chosen: idempotent, cacheable, curl-testable
- Response: `{"artist_ids": [1, 5, 12], "total": 3}` — client uses IDs to filter already-rendered cards

**Map library:**
- **Leaflet + OSM tiles** — chosen: 100% free, no key, mature
- Loaded from unpkg CDN only on `/artist/{id}` pages
- ~~Google Maps JS API~~ — rejected: requires billing

**Filter types (audit requires "at least two types"):**
- **Range**: creation date, first album date (dual sliders)
- **Checkbox**: members (pills 1..8+), locations (hierarchical tree)

**Async filter updates:**
- Debounced fetch on every filter change (250ms) — results change *while* changing

**404 & 500:**
- Custom templates sharing site layout (Schneiderman #1)

---

## 8. Decisions Resolved During Design

Aside from the decisions catalogued in §7 (evaluated once upfront), some decisions were made during the design phase itself:

- **Combine logic placement**: kept inside `internal/store/store.go` as the first step of store build, not split into a separate `internal/aggregate/` package. Rationale: runs once, from one call site, and one fewer package outweighs isolated testability at this project's size.
- **Filter inside store, single file**: filter logic lives in `store.go` alongside combine and lookup logic, not in a separate package or even a separate file. Rationale: filter methods read from the same lookup structures the store builds, and at ~100 lines the split would add noise without any benefit. If the store package grows past ~400 lines, we can revisit splitting into `store.go` + `filter.go` (same package).
- **Geocode cache persistence with corruption recovery**: decided to persist to disk (`data/geocache.json`) with atomic writes and automatic delete-and-rebuild on corruption, plus full fallback to in-memory-only if disk fails. See §4 step 5.

---

## 9. Endpoints

**HTML routes (browser hits):**

| Method | Route | Description | Success | Error |
|--------|-------|-------------|---------|-------|
| GET | `/` | Home — cards grid + search + filters + concert calendar | 200 | 500 |
| GET | `/artist/{id}` | Detail page — info + members + Leaflet map | 200 | 400 / 404 / 500 |
| GET | `/search` | Full-page search results (no-JS fallback) | 200 | 500 |
| GET | `/static/*` | CSS, JS, images | 200 | 404 |
| ANY | `/*` (unmatched) | Custom 404 | — | 404 |

**JSON API routes (async):**

| Method | Route | Description | Success | Error |
|--------|-------|-------------|---------|-------|
| GET | `/api/suggest?q=...` | Autocomplete suggestions from trie | 200 | 400 |
| GET | `/api/filter?...` | Filtered artist IDs | 200 | 400 |
| GET | `/api/artist/{id}/geo` | Pre-geocoded coordinates | 200 | 404 |

Every route uses **GET** — application is read-only, no state mutations.

**External APIs consumed (Go backend):**

| URL | Purpose | When |
|-----|---------|------|
| `https://groupietrackers.herokuapp.com/api/{artists,locations,dates,relation}` | Bands data | Startup, once (concurrent) |
| `https://nominatim.openstreetmap.org/search?q=...&format=json&addressdetails=1` | Coordinates + address (fallback only) | Startup, per unresolved location, max 1/sec |

**External resources loaded by browser (detail page only):**

- Leaflet CSS + JS from unpkg CDN
- OSM tiles from `https://tile.openstreetmap.org/`

---

## 10. Error Handling

- **`recover()` middleware** wraps every handler — a panic becomes a 500 response, never crashes the server
- **Invalid routes** → custom 404 template
- **Invalid query params** (malformed filter, unknown artist ID) → 400
- **Internal errors** (template execution failure, etc.) → 500
- **All error pages share the site layout** (Schneiderman #1)

---

## 11. Concurrency (Async Bonus Coverage)

Two genuine, non-cosmetic uses:

1. **Startup fetch** — 4 goroutines fetch the 4 API endpoints in parallel
2. **Geocoding worker pool** — bounded set of workers pull unique locations from a queue, gated by channel-based rate limiter (1 req/sec to Nominatim), with bounded retry

Both are real solutions to real problems (reducing boot latency; respecting external rate limits without unnecessary serialization) — not concurrency for the audit checkbox.

---

## 12. Acceptance Criteria

### Core
- [ ] Data from `artists`, `locations`, `dates`, `relation` all used
- [ ] Queen → 7 correct members
- [ ] Gorillaz → `firstAlbum: 26-03-2001`
- [ ] Travis Scott → exact 10 audit locations
- [ ] Foo Fighters → 6 correct members
- [ ] One real client-server event (GET-based)
- [ ] Server never crashes; correct HTTP methods; custom 404; 500 handled

### Search (Optional 3)
- [ ] All 5 categories: artist, member, location, first album, creation date
- [ ] Case-insensitive, live suggestions, each suggestion typed
- [ ] "Billie Joe" → Billie Joe Armstrong
- [ ] "Japan" → saitama-japan, osaka-japan, nagoya-japan
- [ ] "Scorpions" → Scorpions
- [ ] "Jimi Hendrix" → The Jimi Hendrix Experience
- [ ] "Phil Collins" → Phil Collins AND Genesis
- [ ] "london-uk" → Pink Floyd, Led Zeppelin, Aerosmith, Alec Benjamin, Nickelback, Eagles, Linkin Park, Coldplay
- [ ] "queen" → Queen AND Queensland
- [ ] "05-08-1967" → Pink Floyd
- [ ] "1973" → ACDC
- [ ] "1965" → Scorpions and Pink Floyd

### Geolocalization (Optional 2)
- [ ] Nominatim used with correct User-Agent, ≤1 req/sec
- [ ] Token-first resolution avoids Nominatim for obvious locations
- [ ] Failed geocodes retry once after 2s, then skip gracefully
- [ ] Corrupted `geocache.json` auto-deleted and rebuilt
- [ ] Queen (8), ACDC (4), Imagine Dragons (12), Guns N' Roses, Post Malone, Red Hot Chili Peppers → correct markers
- [ ] OSM attribution visible on map
- [ ] **Bonus**: chronological tour path polyline

### Filters (Optional 1)
- [ ] Range filter present; checkbox filter present
- [ ] Creation 1995–2000 → 9 audit artists
- [ ] First album 1990–1992 → Pearl Jam, Red Hot Chili Peppers
- [ ] Exactly 6 members → Pink Floyd, Arctic Monkeys, Linkin Park, Foo Fighters
- [ ] Texas, USA → R3HAB, Logic, Joyner Lucas, Twenty One Pilots
- [ ] Combined filters all pass (date + member count; location + member count)
- [ ] "Clear all" restores all 52 artists
- [ ] **Bonus**: async — results update while changing filters

### Visualizations (Optional 4)
- [ ] WCAG-AA contrast, consistent palette/spacing/typography
- [ ] Responsive 320px to 1440px+
- [ ] Custom 404 matches site design language
- [ ] Fast page loads (in-memory data, no unnecessary requests)
- [ ] Keyboard shortcut `/` focuses search (Schneiderman #2)
- [ ] Loading indicators during async operations (Schneiderman #3)
- [ ] Reversible actions — clear filters, back button preserved (Schneiderman #6)

### Code Quality
- [ ] Only Go stdlib used (empty `require` in `go.mod`)
- [ ] Correct HTTP methods (GET everywhere)
- [ ] Panic recovery middleware in place
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l .`, `go test -race ./...` all clean
- [ ] Test file per Go source file except `main.go`

---

## 13. Out of Scope

Explicitly not building — if requested later, follow-up:

- User authentication / login
- Persistent database (all data in memory + geocache.json)
- Editing/creating/deleting artist data
- User comments or ratings
- Real-time updates (upstream fetched once at boot)
- Multi-language support (English only)
- Analytics / user tracking
- Payment / subscriptions

Keeps scope tight, avoids scope creep, keeps audit surface predictable.

---

## 14. Open Decisions

- **Deployment (DNS bonus)**: Fly.io free tier status needs verification (may require credit card as of 2026). Not required for audit passage. Lowest priority.

---

*Last updated: 2026-08-23 | Team: dsakkas, npanagopoul, dpavaloa*