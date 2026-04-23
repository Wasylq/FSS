# FSS — FullStudioScraper: Implementation Plan

A CLI tool that scrapes all scenes + metadata from a given studio URL.
Easily expandable to new sites. Outputs JSON/CSV per studio, with optional SQLite.

---

## Core concepts

**Studio** — the unit of work. You pass a studio URL; FSS fetches every scene that studio has published. A studio maps 1:1 to a creator page on a site (e.g. a ManyVids profile). Studios are tracked so you can:
- Know what you've scraped and when
- Label URLs with human-readable names (`--name "Bettie Bondage"`)
- Query across multiple studios in SQLite (`SELECT * FROM scenes JOIN studios ...`)
- Lay the groundwork for scraping many studios in one run (future feature — not in scope yet)

**Scene** — the atomic record. Every video/clip with its full metadata, price history, and scraper housekeeping fields.

**Price history** — every time you scrape, a new `PriceSnapshot` is appended. `LowestPrice` / `LowestPriceDate` are maintained automatically. Prices are never deleted — they reflect what was true at the time of each scrape.

**Soft-delete** — scenes are never removed from the store. If a scene disappears from the site on a `--refresh` run, `DeletedAt` is set. This preserves your data even if content is taken down.

**Incremental by default** — a plain `fss scrape <url>` only fetches what you don't already have, stopping pagination as soon as it hits a known scene ID. `--full` and `--refresh` override this.

---

## Data Model

```go
// PriceSnapshot captures pricing at a single point in time.
type PriceSnapshot struct {
    Date            time.Time `json:"date"`
    Regular         float64   `json:"regular"`
    Discounted      float64   `json:"discounted,omitempty"` // 0 if not on sale
    IsFree          bool      `json:"isFree"`
    IsOnSale        bool      `json:"isOnSale"`
    DiscountPercent int       `json:"discountPercent,omitempty"` // MV: promoRate
}

// Effective returns what you'd pay at this snapshot.
func (p PriceSnapshot) Effective() float64 {
    if p.IsFree { return 0 }
    if p.IsOnSale { return p.Discounted }
    return p.Regular
}

type Scene struct {
    // Identity
    ID        string // site-specific unique ID
    SiteID    string // e.g. "manyvids"
    StudioURL string // original studio URL passed by user

    // Core
    Title       string
    URL         string    // full URL to the scene page
    Date        time.Time // release/launch date
    Description string

    // Media
    Thumbnail string // MV: screenshot field (higher res)
    Preview   string // MV: preview mp4 URL

    // People & production
    Performers []string // MV: [model.displayName]
    Director   string
    Studio     string // MV: creator.stageName

    // Classification
    Tags       []string // MV: tagList[].label
    Categories []string // broader than tags; some sites distinguish

    // Series
    Series     string
    SeriesPart int

    // Technical
    Duration   int    // seconds, parsed from "MM:SS" or "HH:MM:SS"
    Resolution string // MV: "4K", "HD", "SD"
    Width      int    // MV: width
    Height     int    // MV: height
    Format     string // MV: "MP4"

    // Engagement
    Views    int // MV: viewsRaw
    Likes    int // MV: likesRaw
    Comments int // MV: comments

    // Pricing history
    PriceHistory    []PriceSnapshot
    LowestPrice     float64    // lowest effective price seen across all scrapes
    LowestPriceDate *time.Time // when that price was recorded

    // Scraper housekeeping
    ScrapedAt time.Time
    DeletedAt *time.Time // nil = active; non-nil = soft-deleted (not found on re-scrape)
}

// Studio tracks a registered studio URL and scrape history.
// One Studio = one creator/store page on one site.
type Studio struct {
    URL           string
    SiteID        string
    Name          string     // user-supplied label via --name; never cleared by a scrape without --name
    AddedAt       time.Time
    LastScrapedAt *time.Time
}
```

**Price update logic:** on each scrape, append a new `PriceSnapshot` to `PriceHistory`. Recompute `LowestPrice` / `LowestPriceDate` if the new effective price is lower than the current record.

**Storage notes:**
- JSON: `PriceHistory` serialises naturally as an array
- CSV: serialise `PriceHistory` as a JSON string in one column; `LowestPrice` / `LowestPriceDate` as regular columns
- SQLite: separate `price_history` table with `scene_id` foreign key; separate `studios` table

---

## Config File (XDG)

Optional YAML config at the XDG-standard path per platform:
- Linux:   `~/.config/fss/config.yaml`
- macOS:   `~/Library/Application Support/fss/config.yaml`
- Windows: `%APPDATA%\fss\config.yaml`

Uses `github.com/adrg/xdg` for path resolution.

```yaml
# fss config.yaml — all values are defaults, overridden by CLI flags
workers: 3
output: json          # json | csv | json,csv
out_dir: .
db: ""                # empty = SQLite disabled
sort: date            # date | featured
```

Config is loaded first; CLI flags take precedence over config values.

---

## CLI Interface

```
fss scrape <studio-url> [flags]

Flags:
  --workers  int     Max parallel scene fetchers (default: 3)
  --full             Ignore existing data, scrape everything from scratch
  --refresh          Re-fetch metadata for all known scenes (implies full list traversal)
  --output   string  Export formats: json, csv, or json,csv (default: json)
  --out      string  Output directory (default: current dir)
  --db       string  Enable SQLite and set path (e.g. --db ./fss.db)
  --name     string  Human-readable label for this studio (stored in DB if --db is set)

fss list-scrapers              Print all registered scrapers and their URL patterns
fss list-studios --db <path>   List all studios tracked in the SQLite database
```

Behaviour:
- Default run: fetch scene list, skip scenes already in output file, add new ones only
- `--full`: re-scrape everything, overwrite output
- `--refresh`: re-fetch metadata for all scenes, mark missing ones as deleted (soft-delete)
- `--db`: enables SQLite as source of truth; JSON/CSV become exports from it
- Without `--db`: JSON/CSV are the primary store (loaded on start, saved on finish)
- `--name`: stored on first scrape; subsequent scrapes without `--name` do not clear it

---

## Architecture

```
fss/
  main.go
  go.mod
  models/                   ← PUBLIC: importable by external projects
    scene.go                ← Scene + PriceSnapshot structs, AddPrice()
    studio.go               ← Studio struct
  scraper/                  ← PUBLIC: importable by external projects
    interface.go            ← StudioScraper interface, ListOpts, SceneResult
    registry.go             ← Register(), All(), ForURL()
  cmd/
    root.go                 ← cobra root, loads config, sets global flags
    scrape.go               ← `fss scrape` subcommand
    list_scrapers.go        ← `fss list-scrapers` subcommand
    list_studios.go         ← `fss list-studios` subcommand (SQLite only)
  internal/                 ← private: app logic, not importable externally
    config/
      config.go             ← Config struct, Load() via adrg/xdg
    scrapers/               ← site implementations (internal — interface is the public contract)
      manyvids/
        manyvids.go
        manyvids_test.go
        integration_test.go ← live API test (build tag: integration)
    store/
      interface.go          ← Store interface
      flat.go               ← JSON/CSV flat-file store (default)
      sqlite.go             ← SQLite store (optional, --db flag)
      export_json.go
      export_csv.go
      sqlite_test.go
```

**Public vs internal rationale:**
- `models` and `scraper` are public so external projects can work with FSS data types and implement the scraper interface without forking.
- Scraper *implementations* stay internal — they're tightly coupled to FSS internals and exposing them would mean committing to a stable per-site API. The public contract is the interface, not the implementations.

### StudioScraper interface

```go
type StudioScraper interface {
    ID()       string   // e.g. "manyvids"
    Patterns() []string // URL patterns this scraper handles (informational + registry use)
    MatchesURL(url string) bool
    ListScenes(ctx context.Context, studioURL string, opts ListOpts) (<-chan SceneResult, error)
}

type ListOpts struct {
    Workers int
}

type SceneResult struct {
    Scene models.Scene
    Err   error
}
```

`Patterns()` returns all URL patterns the scraper handles — used by `fss list-scrapers` and by the registry. A scraper may declare multiple patterns (e.g. different URL formats for the same site, or sites sharing a platform backend).

### Store interface

```go
type Store interface {
    Load(studioURL string) ([]models.Scene, error)
    Save(studioURL string, scenes []models.Scene) error
    MarkDeleted(studioURL string, ids []string) error
    Export(format, path, studioURL string) error // no-op for flat store
    UpsertStudio(studio models.Studio) error     // no-op for flat store
    ListStudios() ([]models.Studio, error)       // empty for flat store
}
```

---

## Implementation Phases

### Phase 1 — Scaffold ✓
- [x] `README.md` — initial skeleton: project description, install, config file location/format, pointer to MANUAL.md
- [x] `MANUAL.md` — scaffold: config file keys/defaults, all CLI flags, data model field table (stubs okay)
- [x] `go mod init github.com/Wasylq/FSS`
- [x] Install deps: `cobra`, `gopkg.in/yaml.v3`, `github.com/adrg/xdg`
- [x] `internal/config/config.go` → Config struct + `Load()` (XDG path, YAML parse, defaults)
- [x] `main.go` → entry point
- [x] `cmd/root.go` → cobra root, config loading via PersistentPreRunE
- [x] `cmd/scrape.go` → scrape subcommand with all flags wired up (no logic yet)
- [x] `cmd/list_scrapers.go` → list-scrapers subcommand (prints registry)
- [x] `models/scene.go` → Scene + PriceSnapshot structs, AddPrice() helper (PUBLIC)
- [x] `scraper/interface.go` → StudioScraper + ListOpts + SceneResult (PUBLIC)
- [x] `scraper/registry.go` → Register(), All(), ForURL() (PUBLIC)
- [x] `internal/store/interface.go` → Store interface
- [x] Restructured: `models/` and `scraper/` are public; scraper implementations go in `internal/scrapers/`

### Phase 2 — Flat file store (default) ✓
- [x] `internal/store/flat.go` → load/save scenes from JSON file keyed by studio URL
- [x] `internal/store/export_json.go` → write JSON output
- [x] `internal/store/export_csv.go` → write CSV with all Scene fields
- [x] `MANUAL.md` — output formats section: JSON structure with example, CSV column list, price history shape, all field descriptions

### Phase 3 — ManyVids scraper ✓
> API confirmed. No auth required for public content.

**API endpoints:**
- List:   `GET https://api.manyvids.com/store/videos/{creatorId}?sort=date&page={N}`
  - Page size fixed at 9; `limit` param ignored
  - Fields: id, title, slug, duration, creator, thumbnail, preview, price, likes, views
  - Missing: tags, description, launchDate — need detail endpoint for those
- Detail: `GET https://api.manyvids.com/store/video/{videoId}`
  - Full fields: tagList, description, launchDate, resolution, model, url, screenshot, price

**Sorting:** `?sort=date` (newest first) — enables early-stop on incremental runs once a known ID is hit.

**Creator ID extraction:** parse from studio URL — `/Profile/{creatorId}/...`

**Scene URL construction:** `https://www.manyvids.com` + `data.url` (e.g. `/Video/7342578/fostering-the-bully`)

**Thumbnail:** use `screenshot` field from detail endpoint (higher res than `thumbnail`)

**Two-pass fetch strategy:**
1. Paginate list endpoint → collect all scene IDs (fast, cheap). Stop early if incremental and known ID found.
2. For each new scene ID → fetch detail endpoint in parallel (controlled by `--workers`)
3. On `--refresh`: re-fetch detail for all known IDs; soft-delete IDs no longer in list

**Field mapping (detail endpoint → Scene):**
| API field | Scene field |
|---|---|
| `id` | ID |
| `launchDate` | Date |
| `title` | Title |
| `url` (+ base URL) | URL |
| `screenshot` | Thumbnail |
| `preview.url` (from list) | Preview |
| `model.displayName` | Performers[0] |
| `model.displayName` | Studio |
| `tagList[].label` | Tags |
| `videoDuration` ("MM:SS") | Duration (seconds) |
| `description` (HTML-unescaped) | Description |
| `resolution` | Resolution |
| `width` / `height` | Width / Height |
| `extension` | Format |
| `viewsRaw` | Views |
| `likesRaw` | Likes |
| `comments` | Comments |
| `price.regular` | PriceHistory[n].Regular |
| `price.free` | PriceHistory[n].IsFree |
| `price.onSale` | PriceHistory[n].IsOnSale |
| `price.discountedPrice` | PriceHistory[n].Discounted |
| `price.promoRate` | PriceHistory[n].DiscountPercent |

- [x] `internal/scrapers/manyvids/manyvids.go` — full implementation
- [x] `internal/scrapers/manyvids/manyvids_test.go` — unit + httptest tests, all passing
- [x] `internal/scrapers/manyvids/integration_test.go` — live API test (build tag: integration)
- [x] Live test passed: 5 scenes, 0.7s, all fields correct, HTML entities stripped from description
- [x] `MANUAL.md` — "Adding a scraper" and "Modifying a scraper" sections (ManyVids as worked example)
- [x] `README.md` — usage section with real example commands

### Phase 4 — SQLite store + studio tracking ✓
- [x] `internal/store/sqlite.go` — `scenes` + `price_history` + `studios` tables, full CRUD
- [x] `Export(format, path, studioURL)` — updated signature on interface + flat store
- [x] `models/studio.go` — Studio struct
- [x] `UpsertStudio` / `ListStudios` on Store interface; SQLite full implementation; flat store no-op
- [x] Studio upsert: `--name` value sets/updates the label; omitting `--name` never clears an existing name
- [x] `cmd/scrape.go` — `--name` flag added
- [x] `cmd/list_studios.go` — `fss list-studios --db <path>` command
- [x] Tests: Save/Load round-trip, idempotent save, price history accumulation, MarkDeleted idempotency, studio upsert/list/name preservation — all passing
- [x] Wire up: `--db` → SQLite store + UpsertStudio called at scrape time (Phase 5)
- [x] `MANUAL.md` — SQLite section: schema (scenes, price_history, studios), enabling, example queries

### Phase 5 — Scrape logic + store wiring (in `cmd/scrape.go`) ✓
- [x] Resolve flags against config (workers, output, out, db, name)
- [x] Select store: `--db` → SQLite, otherwise flat
- [x] Look up scraper via `scraper.ForURL(studioURL)`
- [x] Default run: load existing scene IDs, pass to scraper for early-stop, merge new scenes, save
- [x] `--full`: skip loading existing, scrape all pages, save
- [x] `--refresh`: scrape full list, re-fetch all metadata, soft-delete missing IDs, save
- [x] Call `store.UpsertStudio` at end of each successful scrape (update `last_scraped_at`)
- [x] `MANUAL.md` — resume/update behaviour section

### Phase 6 — Polish ✓
- [x] Progress output: live `\r` counter during fetch; per-mode summary; error count
- [x] Graceful shutdown on Ctrl+C: context cancellation stops scraper, partial results saved, clear "Partial save" message
- [x] Final pass on `README.md` and `MANUAL.md` — verified current and consistent

---

## Future / out of scope for now

- **Multi-studio in one command** — `fss scrape url1 url2 url3` or `fss scrape-all --db ./fss.db`. The data model and store already support it; it's a CLI orchestration addition.
- **Stash integration** — Phase 7 (see below).
- **Auth / paywalled content** — currently public content only; session handling deferred.

---

## Open Questions / Decisions Pending

- [x] **ManyVids API shape** — confirmed: `api.manyvids.com/store/videos/{id}` + `api.manyvids.com/store/video/{id}`, no auth
- [x] **`per_page` / `limit` param** — not honored; page size fixed at 9. 700 scenes = 78 list requests
- [x] **Module path** — `github.com/Wasylq/FSS`
- [x] **Config format** — YAML via `gopkg.in/yaml.v3`, path via `adrg/xdg`
- [x] **Multi-pattern scrapers** — `Patterns() []string` added to interface
- [x] **Scene struct** — fat struct with all fields; Price/Preview/Series/Resolution all included
- [x] **Studio tracking** — `studios` table in SQLite; `--name` flag; `fss list-studios`; flat store is no-op

---

## Documentation

Two living documents, both updated at the end of every phase.

### README.md
Quick-start focused. Someone clones the repo and can be running in 5 minutes.

- Project description and install
- Config file location and format
- Common usage examples (`scrape`, `list-scrapers`, `list-studios`)
- Output file overview (what you get and where)
- Pointer to `MANUAL.md` for deeper reference

### MANUAL.md
Full technical reference. Never needs to be read end-to-end — written to be searched.

- **All CLI flags** — every flag, its default, its interaction with config
- **Data model reference** — every `Scene` and `Studio` field, its type, which sites populate it, what it means
- **Output formats in depth** — JSON structure with example, CSV column list, price history shape, SQLite schema + example queries
- **Adding a scraper** — step-by-step with a minimal annotated template; what each interface method must do; how to register; how to test
- **Modifying a scraper** — how to add a field to `models.Scene` and what else must change (CSV headers, SQLite schema, existing scraper mapping)
- **Resume / update behaviour** — exactly what default / `--full` / `--refresh` do to the store
- **Config file** — all keys, types, defaults, precedence over flags

**Per-phase doc updates:**
- Phase 1: README skeleton + MANUAL scaffold (config, CLI flags, data model stub)
- Phase 2: MANUAL output formats section (JSON/CSV field list, price history shape)
- Phase 3: MANUAL "Adding a scraper" using ManyVids as worked example; README usage section with real commands
- Phase 4: MANUAL SQLite section (schema, enabling, example queries)
- Phase 5: MANUAL resume/update behaviour section
- Phase 6: final pass on both, verify everything is current

---

## Notes

- New site = new file in `internal/scrapers/<site>/`, implement interface, register in `registry.go`. Nothing else to change.
- `Patterns()` lets a scraper claim multiple URL formats or shared-platform URLs without duplicating the implementation.
- Soft-delete: `DeletedAt` is set when a scene is no longer found on re-scrape. Never removed automatically.
- SQLite is off by default. Without `--db`, the JSON file is the source of truth (loaded at start, diffed, saved at end).
- Studio tracking is SQLite-only. The flat store silently ignores `UpsertStudio` / `ListStudios`.
- CommunityScrapers can be referenced for ManyVids field mapping but this project always targets the full studio list, not a single scene.
- Price fields reflect the price at time of scrape — they will go stale and that is expected.

---

## Phase 7 — Stash Integration

### Context

FSS scrapes studio catalogs into JSON/CSV/SQLite. The downstream workflow is: scrape → download videos → organize in Stash. Currently there's no bridge — metadata from FSS must be manually entered into Stash. This phase adds `fss stash` subcommands that connect to a Stash instance, find unmatched scenes, match them by filename against FSS JSON output, and push merged metadata back.

### Commands

#### `fss stash unmatched`

List scenes in Stash that have no metadata yet (no StashDB IDs).

```
fss stash unmatched [--url localhost:9999] [--api-key KEY]
                     [--performer "Name"] [--studio "Name"]
```

Queries `findScenes` with `stash_id_count == 0`. Optional performer/studio filters narrow results. Prints a table: `ID | Filename | Title | Performers`.

#### `fss stash import`

Match FSS JSON scenes against Stash scenes by filename, then push metadata.

```
fss stash import [--url localhost:9999] [--api-key KEY]
                  [--dir .] [--json file.json ...]
                  [--tag fss_import] [--resolution-tags]
                  [--organized] [--scrape]
                  [--include-stashbox] [--stashbox-tag fss_stashbox_override]
                  [--apply]
```

**Default is dry-run** — shows what would change. Pass `--apply` to actually write to Stash.

- `--include-stashbox`: also process scenes that already have StashDB IDs. These scenes get an extra tracking tag (`fss_stashbox_override` by default, configurable via `--stashbox-tag`) so overrides are easy to find and revert in Stash.

### New Packages & Files

```
internal/stash/
  client.go      — GraphQL client (FindScenes, SceneUpdate, FindTags, TagCreate, etc.)
  match.go       — Load FSS JSONs, build title index, match against Stash filenames
  merge.go       — Cross-site scene merging (combine URLs, earliest date, union tags)

cmd/
  stash.go           — parent "stash" command group + shared flags (--url, --api-key)
  stash_unmatched.go — "stash unmatched" subcommand
  stash_import.go    — "stash import" subcommand
```

### Stash GraphQL Client (`internal/stash/client.go`)

Thin wrapper using `httpx.Do` for HTTP + JSON encoding of GraphQL requests.

```go
type Client struct {
    url    string       // e.g. "http://localhost:9999/graphql"
    apiKey string       // sent as ApiKey header, empty = no auth
    http   *http.Client
}
```

Methods needed:
- `FindScenes(ctx, filter) → []StashScene`
- `FindTagByName(ctx, name) → (id string, found bool)`
- `CreateTag(ctx, name) → id string`
- `FindPerformerByName(ctx, name) → (id string, found bool)`
- `CreatePerformer(ctx, name) → id string`
- `FindStudioByName(ctx, name) → (id string, found bool)`
- `CreateStudio(ctx, name) → id string`
- `UpdateScene(ctx, input SceneUpdateInput) → error`
- `ScrapeSceneURL(ctx, url) → ScrapedScene` (for optional --scrape)

All find-or-create pairs can be a single helper: `EnsureTag(ctx, name) → id`.

### Matching Strategy (`internal/stash/match.go`)

Files are typically named after the scene title but sometimes garbled. Matching defaults to filename-to-title comparison.

#### Loading FSS data

Read all `*.json` files from `--dir` (or specific `--json` files). Parse each as `store.studioFile`. Build a dual index:

```go
type SceneIndex struct {
    byTitle map[string][]models.Scene  // normalized title → scenes from multiple sites
    all     []models.Scene             // flat list for substring/fuzzy search
}
```

Normalization: lowercase, strip all non-alphanumeric, collapse whitespace, trim.
E.g. `"MILF JOI Countdown!!!"` → `"milf joi countdown"`

#### Matching against Stash scenes

For each Stash scene's `files[].basename`:
1. Strip extension → raw filename
2. Normalize the same way
3. **Pass 1 — exact**: normalized filename == normalized title → `[exact]`
4. **Pass 2 — substring**: normalized title is a substring of normalized filename → `[substring]`
5. **Pass 3 — best substring**: if multiple substring matches, pick the longest title
6. No match → `[skip]`

Ambiguous matches (multiple different titles match the same filename) are flagged as `[ambiguous]` in dry-run and skipped in apply mode.

#### Confidence display in dry-run

```
EXACT      scene.mp4           →  "Fostering the Bully" (manyvids + clips4sale)
SUBSTR     studio-title.mp4    →  "JOI Countdown" (iwantclips)
AMBIGUOUS  vague-name.mp4      →  2 candidates, skipped
SKIP       random_file.mp4     →  no match
```

### Cross-Site Merging (`internal/stash/merge.go`)

When a scene title appears in multiple FSS JSONs (e.g., scraped from both ManyVids and C4S):

| Field | Strategy |
|-------|----------|
| URLs | Union — all site URLs |
| Date | Earliest non-zero across ALL sources (FSS sites + existing Stash date) |
| Title | First non-empty (prefer site with more detail) |
| Description | Longest non-empty |
| Performers | Union (deduplicated) |
| Tags | Union (deduplicated) |
| Duration | Maximum (highest quality source) |
| Resolution/Width/Height | Highest resolution available |

### Tag Strategy

Tags applied to each matched Stash scene:

1. **Import marker tag**: configurable via `--tag` (default `"fss_import"`). Created once, reused for all imports.

2. **Stashbox override tag**: `"fss_stashbox_override"` (configurable via `--stashbox-tag`). Only applied when `--include-stashbox` is used and a scene with existing StashDB data is modified. Makes it trivial to filter in Stash UI and revert if needed.

   **Changelog**: When `--include-stashbox` modifies a scene, a structured JSON log entry is appended to `fss-stashbox-changelog.json` (in output dir). Each entry records:
   ```json
   {
     "stash_scene_id": "42",
     "timestamp": "2026-04-23T...",
     "filename": "scene.mp4",
     "matched_to": "Fostering the Bully",
     "changes": {
       "date":  {"from": "2026-02-01", "to": "2026-01-01"},
       "urls":  {"added": ["https://manyvids.com/..."]},
       "tags":  {"added": ["JOI", "4K Available"]},
       "title": {"from": "Old Title", "to": "Fostering the Bully"}
     }
   }
   ```
   This allows reverting individual fields if StashDB data was better. The file is append-only — multiple import runs accumulate history.

3. **Scene tags from FSS**: All tags from the merged scene. Find-or-create each in Stash.

4. **Resolution tags** (when `--resolution-tags` is set):
   - `"4K Available"` if width ≥ 3840
   - `"Full HD Available"` if width ≥ 1920
   - `"HD Available"` if width ≥ 1280

5. All tags are **additive** — existing tags on the Stash scene are preserved.

### Import Flow (`cmd/stash_import.go`)

```
1. Connect to Stash (validate connection with a simple query)
2. Load FSS JSON files → build SceneIndex
3. Ensure import tag exists in Stash (find-or-create)
4. Query Stash for target scenes:
   - Default: stash_id_count == 0 (skip anything with StashDB data entirely)
   - With --include-stashbox: also process scenes that have StashDB IDs
     → these get an extra tag "fss_stashbox_override" (configurable) so
       changes are easy to find and revert in Stash
   - Optional performer/studio filters applied on top
5. For each Stash scene:
   a. Extract basename from files[0].path
   b. Look up in SceneIndex
   c. If no match → skip
   d. Merge cross-site FSS scenes
   e. Compare with existing Stash scene data:
      - Date: use earliest across FSS + existing Stash date
      - URLs: union of FSS URLs + existing Stash URLs
      - Tags: additive (existing + new)
   f. Dry-run: print what would change
   g. Apply mode:
      - EnsurePerformer for each performer → get IDs
      - EnsureStudio → get ID
      - EnsureTags for all tags → get IDs
      - SceneUpdate with: title, details, date, urls, performer_ids,
        studio_id, tag_ids (merged)
      - If --organized: set organized: true
      - If --scrape: call Stash's scrapeSceneURL with first URL for extra data
6. Print summary: N matched, M updated, K skipped, J already up-to-date
```

### Config Additions (`internal/config/config.go`)

```go
type StashConfig struct {
    URL            string `yaml:"url"`              // default "http://localhost:9999"
    APIKey         string `yaml:"api_key"`           // default ""
    Tag            string `yaml:"tag"`               // default "fss_import"
    StashboxTag    string `yaml:"stashbox_tag"`      // default "fss_stashbox_override"
    ResolutionTags bool   `yaml:"resolution_tags"`   // default true
    Scrape         bool   `yaml:"scrape"`            // default false
}
```

CLI flags override config. The API key can also be set via `FSS_STASH_API_KEY` env var.

### Files Modified

- `internal/config/config.go` — add `Stash StashConfig` field
- `cmd/root.go` — no change (config loading is inherited)
- `main.go` — no change

### Verification

1. `go build ./... && go test ./...` — compilation and unit tests
2. `fss stash unmatched` against a running Stash instance — should list scenes
3. `fss stash import` (dry-run) — should show matches without modifying Stash
4. `fss stash import --apply` — should update scenes, verify in Stash UI
5. Verify cross-site merge: scrape same performer from two sites, import, check that Stash scene gets both URLs and earliest date
