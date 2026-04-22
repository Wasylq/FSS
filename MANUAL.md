# FSS Manual

Full technical reference. Use the section links to jump to what you need.

- [CLI flags](#cli-flags)
- [Config file](#config-file)
- [Data model](#data-model)
- [Output formats](#output-formats)
- [Adding a scraper](#adding-a-scraper)
- [Modifying a scraper](#modifying-a-scraper)
- [Resume and update behaviour](#resume-and-update-behaviour)
- [SQLite](#sqlite)

---

## CLI flags

### `fss scrape <studio-url>`

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--workers`, `-w` | int | 3 | Max parallel metadata fetchers |
| `--full` | bool | false | Ignore existing data, scrape everything from scratch |
| `--refresh` | bool | false | Re-fetch metadata for all known scenes; soft-delete missing ones |
| `--output`, `-o` | string | `json` | Export format(s): `json`, `csv`, or `json,csv` |
| `--out` | string | `.` | Output directory |
| `--db` | string | _(disabled)_ | Path to SQLite database; enables SQLite store |
| `--delay` | int | `0` | Milliseconds to sleep between page requests; applies per page fetch |

`--full` and `--refresh` are mutually exclusive. `--full` ignores all existing data. `--refresh` traverses the full scene list but re-uses existing IDs to update metadata in place and detect deletions.

### `fss list-scrapers`

Prints all registered scrapers and the URL patterns each one handles. No flags.

---

## Config file

Located at the XDG config path for your platform (see README). All keys are optional — missing keys use the defaults shown below.

```yaml
workers: 3        # int   — parallel metadata fetchers
output: json      # str   — json | csv | json,csv
out_dir: .        # str   — output directory path
db: ""            # str   — SQLite path; empty string disables SQLite
delay: 0          # int   — ms between page requests; 0 disables
```

CLI flags take precedence over config values. Config values take precedence over built-in defaults.

---

## Data model

### Scene

All fields that FSS stores per scene. Fields marked _optional_ are omitted from JSON output when empty.

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `id` | string | yes | Site-specific unique ID |
| `siteId` | string | yes | Scraper ID (e.g. `manyvids`) |
| `studioUrl` | string | yes | Studio URL passed on the command line |
| `title` | string | yes | |
| `url` | string | yes | Full URL to the scene page |
| `date` | time | yes | Release / launch date |
| `description` | string | optional | |
| `thumbnail` | string | optional | URL to the main preview image |
| `preview` | string | optional | URL to the preview video clip |
| `performers` | []string | optional | |
| `director` | string | optional | |
| `studio` | string | optional | Creator / brand name |
| `tags` | []string | optional | |
| `categories` | []string | optional | Broader groupings; some sites distinguish from tags |
| `series` | string | optional | |
| `seriesPart` | int | optional | |
| `duration` | int | optional | Seconds |
| `resolution` | string | optional | e.g. `4K`, `HD`, `SD` |
| `width` | int | optional | Pixels |
| `height` | int | optional | Pixels |
| `format` | string | optional | e.g. `MP4` |
| `views` | int | optional | At time of scrape |
| `likes` | int | optional | At time of scrape |
| `comments` | int | optional | At time of scrape |
| `priceHistory` | []PriceSnapshot | optional | See below |
| `lowestPrice` | float64 | optional | Lowest effective price seen across all scrapes |
| `lowestPriceDate` | time | optional | When that lowest price was recorded |
| `scrapedAt` | time | yes | When this record was last written |
| `deletedAt` | time | optional | Set when scene is no longer found on re-scrape; never removed |

### PriceSnapshot

One entry is appended to `priceHistory` on each scrape run.

| Field | Type | Notes |
|-------|------|-------|
| `date` | time | When this snapshot was taken |
| `regular` | float64 | Full price |
| `discounted` | float64 | Sale price; 0 if not on sale |
| `isFree` | bool | |
| `isOnSale` | bool | |
| `discountPercent` | int | e.g. `50` for 50% off |

**Effective price** = `discounted` if on sale, `0` if free, otherwise `regular`.

---

## Output formats

### File naming

Output files are named by sanitising the studio URL into a slug:

```
https://www.manyvids.com/Profile/590705/bettie-bondage/Store/Videos
→ manyvids.com-profile-590705-bettie-bondage-store-videos.json
→ manyvids.com-profile-590705-bettie-bondage-store-videos.csv
```

### JSON

The JSON file wraps the scene list with a small header:

```json
{
  "studioUrl": "https://www.manyvids.com/Profile/590705/bettie-bondage/Store/Videos",
  "scrapedAt": "2026-04-22T10:00:00Z",
  "sceneCount": 700,
  "scenes": [ ... ]
}
```

Each scene is a JSON object with the fields listed in the [Data model](#data-model) section. Optional fields are omitted when empty.

JSON is **always written** by the flat store — it is the backing format for incremental updates. Even if you request `--output csv` only, a JSON file is also created alongside it.

### CSV

One row per scene. Column order matches the table below exactly.

Multi-value fields (`performers`, `tags`, `categories`) use `|` as a separator — e.g. `Alice|Bob`.

`priceHistory` is serialised as a JSON string within its column. Use a JSON-aware tool (e.g. `jq`, DuckDB, Python) to query it; otherwise treat it as opaque.

| Column | Type | Notes |
|--------|------|-------|
| `id` | string | Site-specific unique ID |
| `siteId` | string | e.g. `manyvids` |
| `studioUrl` | string | |
| `title` | string | |
| `url` | string | Full scene page URL |
| `date` | RFC3339 | Release date |
| `description` | string | |
| `thumbnail` | string | URL |
| `preview` | string | URL to preview clip |
| `performers` | string | `\|`-separated |
| `director` | string | |
| `studio` | string | |
| `tags` | string | `\|`-separated |
| `categories` | string | `\|`-separated |
| `series` | string | |
| `seriesPart` | int | |
| `duration` | int | Seconds |
| `resolution` | string | e.g. `4K`, `HD` |
| `width` | int | Pixels |
| `height` | int | Pixels |
| `format` | string | e.g. `MP4` |
| `views` | int | At time of scrape |
| `likes` | int | At time of scrape |
| `comments` | int | At time of scrape |
| `lowestPrice` | float | Lowest effective price seen |
| `lowestPriceDate` | RFC3339 | When that price was recorded |
| `priceHistory` | JSON string | Array of PriceSnapshot objects |
| `scrapedAt` | RFC3339 | |
| `deletedAt` | RFC3339 | Empty if active |

---

## Adding a scraper

### Steps

1. Create `internal/scrapers/<site>/<site>.go` in package `<site>`
2. Implement the four-method `scraper.StudioScraper` interface
3. Register in `init()` so the binary picks it up automatically
4. Add a blank import in `main.go`
5. Write tests in `<site>_test.go` (same package)

### The interface

```go
type StudioScraper interface {
    ID()       string                 // stable lowercase key, e.g. "clips4sale"
    Patterns() []string               // URL patterns — shown by `fss list-scrapers`
    MatchesURL(url string) bool       // runtime URL lookup used by the registry
    ListScenes(ctx context.Context, studioURL string, opts ListOpts) (<-chan SceneResult, error)
}
```

`ListScenes` must close the returned channel when done and respect `ctx` cancellation.

### Minimal template

```go
package mysite

import (
    "context"
    "github.com/Wasylq/FSS/models"
    "github.com/Wasylq/FSS/scraper"
)

type Scraper struct{ /* http client, base URLs */ }

func init() { scraper.Register(&Scraper{}) }

func (s *Scraper) ID() string       { return "mysite" }
func (s *Scraper) Patterns() []string { return []string{"mysite.com/studio/*"} }
func (s *Scraper) MatchesURL(u string) bool { return strings.Contains(u, "mysite.com/studio/") }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
    out := make(chan scraper.SceneResult)
    go func() {
        defer close(out)
        // paginate, fetch detail, map to models.Scene, send
        out <- scraper.SceneResult{Scene: models.Scene{ /* ... */ }}
    }()
    return out, nil
}
```

### ManyVids as a worked example

ManyVids uses two API endpoints:

| Purpose | Endpoint |
|---------|----------|
| List all scene IDs (paginated, 9/page) | `GET api.manyvids.com/store/videos/{creatorId}?sort=date&page=N` |
| Full metadata for one scene | `GET api.manyvids.com/store/video/{id}` |

The creator ID is extracted from the studio URL with a regex:
`manyvids\.com/Profile/(\d+)/[^/]+/Store/Videos`

**Worker pool pattern** — `ListScenes` runs a goroutine that:
1. Paginates the list endpoint, sending IDs into a `work` channel
2. N worker goroutines read from `work`, call `fetchDetail`, send `SceneResult` to the output channel
3. When pagination is done, `work` is closed; workers drain and exit; output channel is closed

The `apiBase` and `siteBase` fields on the scraper struct are overridable — tests inject an `httptest.Server` URL so no real network calls are made.

### Clips4Sale as a worked example

Clips4Sale returns full scene metadata in the paginated list — no separate detail endpoint is needed.

**List endpoint** (24 clips per page):

```
GET https://www.clips4sale.com/en/studio/{studioId}/{slug}/Cat0-AllCategories/Page{N}/C4SSort-recommended/Limit24/?onlyClips=true&storeSimilarClips=false&_data=routes/($lang).studio.$id_.$studioSlug.$
```

Returns `{"clips": [...], "onSaleClips": [...]}`. The `clips` array contains all scenes including on-sale ones.

The studio ID and slug are extracted from the studio URL with a regex:
`clips4sale\.com/studio/(\d+)/([a-zA-Z][^/?]*)` — the slug must start with a letter to distinguish studio URLs from individual clip URLs (which use a numeric clip ID as the second path segment).

**Single-goroutine pattern** — `ListScenes` runs a single goroutine that paginates the list endpoint and emits one `SceneResult` per clip. No worker pool is needed.

**Incremental mode caveat** — the default sort is `C4SSort-recommended`, not date order. Pagination cannot stop early when a known ID is encountered. Instead, known IDs are skipped per-clip; all pages are always enumerated in incremental mode.

**Key field mappings:**

| JSON field | Scene field | Notes |
|-----------|-------------|-------|
| `clipId` | `ID` | String |
| `link` | `URL` | Relative; prepend `https://www.clips4sale.com` |
| `date_display` | `Date` | Format `"1/2/06 3:04 PM"` |
| `description` | `Description` | HTML — tags stripped, entities unescaped |
| `cdn_previewlg_link` | `Thumbnail` | |
| `customPreviewUrl` | `Preview` | |
| `performers[].stage_name` | `Performers` | |
| `studioTitle` | `Studio` | |
| `category_name` | `Categories` | Primary C4S category |
| `related_category_links[].category` + `keyword_links[].keyword` | `Tags` | Deduped |
| `time_minutes × 60` | `Duration` | Seconds |
| `resolution_text` (uppercased) | `Resolution` | e.g. `4K` |
| `screen_size` | `Width` / `Height` | Split on `x` |
| `format` (uppercased) | `Format` | e.g. `MP4` |
| `price` | `PriceSnapshot.Regular` | |
| `discounted_price` | `PriceSnapshot.Discounted` | null → 0 |
| `onSale` | `PriceSnapshot.IsOnSale` | null → false |

The `siteBase` and `pageLimit` fields are overridable for testing via `httptest.Server`.

### Wiring it up

In `main.go`, add a blank import so the `init()` function runs:

```go
import _ "github.com/Wasylq/FSS/internal/scrapers/mysite"
```

Run `fss list-scrapers` to confirm registration.

---

## Modifying a scraper

### Adding a new field to `models.Scene`

1. **`models/scene.go`** — add the field with JSON tag
2. **The scraper** — populate the field in `toScene()` (or equivalent mapping function)
3. **`internal/store/export_csv.go`** — add the column name to `csvHeaders` and the value to `sceneToRow()`
4. **`MANUAL.md`** — add a row to the CSV column table and the data model table
5. If using SQLite — add the column to the `CREATE TABLE` statement and the insert/select queries in `internal/store/sqlite.go`

### Adding a new URL pattern to an existing scraper

Update `Patterns()` to return the new pattern and update `MatchesURL()` to recognise it. Add a test case to `TestMatchesURL`. No other files change.

---

## Resume and update behaviour

FSS supports three modes, selected by flags on `fss scrape`.

### Default — incremental (no flag)

The fastest option. Suitable for routine daily runs.

1. Load all scene IDs already stored for this studio.
2. Paginate the site's scene list (newest-first). As soon as an already-known ID appears, stop — everything behind it is already in the store.
3. Fetch full metadata only for the newly discovered scenes.
4. Merge new scenes in front of the existing set and save.

**Trade-off:** if the site re-orders or back-fills scenes you may miss them. Use `--refresh` periodically to catch that.

### `--full`

Ignores all existing data. Fetches every page, fetches every scene. Overwrites the store.

**Price history is not preserved** — all accumulated price snapshots are discarded and replaced with a single snapshot from this run. Use `--refresh` if you want to keep price history while re-fetching metadata.

Use when you want a clean slate or after a schema change.

### `--refresh`

Full traversal (every page, every scene) but preserves history:

- **Price history** from prior scrapes is carried forward — each re-fetched scene gets the new price snapshot appended to its existing history.
- **Soft-delete** — any scene that was in the store but is no longer returned by the site has its `deletedAt` timestamp set. It is never removed from the store.

Use periodically (e.g. weekly) to catch deletions and accumulate accurate price history.

---

## SQLite

### Enabling

Pass `--db <path>` to any scrape command, or set `db` in your config file:

```bash
fss scrape --db ./fss.db <studio-url>
```

When `--db` is set, SQLite is the source of truth. JSON/CSV files are exported from it if `--output` requests them.

### Schema

Two tables. Inspect with any SQLite client (`sqlite3`, DBeaver, TablePlus, etc.).

**`scenes`** — one row per scene:

| Column | Type | Notes |
|--------|------|-------|
| `id` | TEXT | Site-specific ID (primary key with `site_id`) |
| `site_id` | TEXT | e.g. `manyvids` |
| `studio_url` | TEXT | Indexed |
| `title` | TEXT | |
| `url` | TEXT | |
| `date` | TEXT | RFC3339 |
| `description` | TEXT | |
| `thumbnail` | TEXT | |
| `preview` | TEXT | |
| `performers` | TEXT | JSON array |
| `director` | TEXT | |
| `studio` | TEXT | |
| `tags` | TEXT | JSON array |
| `categories` | TEXT | JSON array |
| `series` | TEXT | |
| `series_part` | INTEGER | |
| `duration` | INTEGER | Seconds |
| `resolution` | TEXT | |
| `width` | INTEGER | |
| `height` | INTEGER | |
| `format` | TEXT | |
| `views` | INTEGER | |
| `likes` | INTEGER | |
| `comments` | INTEGER | |
| `lowest_price` | REAL | |
| `lowest_price_date` | TEXT | RFC3339, nullable |
| `scraped_at` | TEXT | RFC3339 |
| `deleted_at` | TEXT | RFC3339, nullable — NULL means active |

**`price_history`** — one row per price snapshot per scene:

| Column | Type |
|--------|------|
| `id` | INTEGER (autoincrement) |
| `scene_id` | TEXT |
| `site_id` | TEXT |
| `date` | TEXT |
| `regular` | REAL |
| `discounted` | REAL |
| `is_free` | INTEGER (0/1) |
| `is_on_sale` | INTEGER (0/1) |
| `discount_percent` | INTEGER |

**`studios`** — one row per studio URL:

| Column | Type | Notes |
|--------|------|-------|
| `url` | TEXT | Primary key |
| `site_id` | TEXT | e.g. `manyvids` |
| `name` | TEXT | User-supplied label via `--name`; never cleared by a scrape that omits `--name` |
| `added_at` | TEXT | RFC3339 — when first scraped |
| `last_scraped_at` | TEXT | RFC3339, nullable |

### Listing studios

```bash
fss list-studios --db ./fss.db
```

### Example queries

```sql
-- All active scenes for a studio
SELECT title, date, duration, lowest_price
FROM scenes
WHERE studio_url = 'https://www.manyvids.com/Profile/590705/bettie-bondage/Store/Videos'
  AND deleted_at IS NULL
ORDER BY date DESC;

-- Scenes that have ever been on sale
SELECT s.title, ph.regular, ph.discounted, ph.date
FROM scenes s
JOIN price_history ph ON ph.scene_id = s.id AND ph.site_id = s.site_id
WHERE ph.is_on_sale = 1
ORDER BY ph.date DESC;

-- Price history for one scene
SELECT date, regular, discounted, is_on_sale, discount_percent
FROM price_history
WHERE scene_id = '7342578' AND site_id = 'manyvids'
ORDER BY date ASC;

-- All studios and their scene counts
SELECT st.name, st.site_id, st.last_scraped_at, COUNT(sc.id) AS scenes
FROM studios st
LEFT JOIN scenes sc ON sc.studio_url = st.url AND sc.deleted_at IS NULL
GROUP BY st.url;

-- Scenes with a specific tag (SQLite JSON extension)
SELECT title FROM scenes, json_each(tags)
WHERE json_each.value = 'MILF'
  AND deleted_at IS NULL;
```
