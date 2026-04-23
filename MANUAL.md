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
- [Stash integration](#stash-integration)

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

### `fss stash unmatched`

Lists Stash scenes that have no StashDB metadata (`stash_id_count == 0`).

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--url` | string | `http://localhost:9999` | Stash server URL |
| `--api-key` | string | _(empty)_ | Stash API key (also: `FSS_STASH_API_KEY` env var) |
| `--performer` | string | _(none)_ | Filter by performer name |
| `--studio` | string | _(none)_ | Filter by studio name |
| `--top` | int | `10` | Limit number of results; 0 = all |

### `fss stash import`

Matches FSS JSON scenes against Stash scenes by filename and pushes metadata. **Dry-run by default** — pass `--apply` to write changes.

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--url` | string | `http://localhost:9999` | Stash server URL |
| `--api-key` | string | _(empty)_ | Stash API key (also: `FSS_STASH_API_KEY` env var) |
| `--dir` | string | config `out_dir` | Directory to scan — loads every `*.json` file in it |
| `--json` | []string | _(none)_ | Load only these specific JSON files (overrides `--dir`) |
| `--tag` | string | `fss_import` | Import marker tag applied to every matched scene |
| `--resolution-tags` | bool | from config | Add resolution tags (`4K Available`, `Full HD Available`, `HD Available`) |
| `--organized` | bool | `false` | Set the organized flag on imported scenes |
| `--scrape` | bool | from config | Call Stash's built-in scraper on the first URL after import |
| `--include-stashbox` | bool | `false` | Also process scenes that already have StashDB data |
| `--stashbox-tag` | string | `fss_stashbox_override` | Tag applied to modified StashDB scenes for tracking |
| `--apply` | bool | `false` | Actually write changes to Stash |
| `--performer` | string | _(none)_ | Filter Stash scenes by performer name |
| `--studio` | string | _(none)_ | Filter Stash scenes by studio name |
| `--top` | int | `0` | Limit number of Stash scenes to process; 0 = all |

---

## Config file

Located at the XDG config path for your platform (see README). All keys are optional — missing keys use the defaults shown below.

```yaml
workers: 3        # int   — parallel metadata fetchers
output: json      # str   — json | csv | json,csv
out_dir: .        # str   — output directory path
db: ""            # str   — SQLite path; empty string disables SQLite
delay: 0          # int   — ms between page requests; 0 disables

stash:
  url: "http://localhost:9999"    # str   — Stash server URL
  api_key: ""                     # str   — API key (prefer FSS_STASH_API_KEY env var)
  tag: "fss_import"               # str   — import marker tag
  stashbox_tag: "fss_stashbox_override"  # str — tag for StashDB override tracking
  resolution_tags: true           # bool  — add 4K/FHD/HD Available tags
  scrape: false                   # bool  — invoke Stash scraper after import
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

See [CONTRIBUTING.md](CONTRIBUTING.md) for the full step-by-step guide with code examples and a checklist.

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

Three tables. Inspect with any SQLite client (`sqlite3`, DBeaver, TablePlus, etc.).

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

---

## Stash integration

FSS can push scraped metadata into a local [Stash](https://stashapp.cc/) instance, matching scenes by filename against FSS JSON output.

### Workflow

1. Scrape studios as usual: `fss scrape <url>` — produces JSON files
2. Download videos and add them to Stash (outside FSS)
3. List unmatched scenes: `fss stash unmatched`
4. Import metadata: `fss stash import --dir ./data` (dry-run first, then `--apply`)

### Connecting to Stash

By default, FSS connects to `http://localhost:9999`. Override with `--url` or the `stash.url` config key.

If your Stash instance requires authentication, provide an API key via:
- `--api-key` flag
- `FSS_STASH_API_KEY` environment variable
- `stash.api_key` config key

Precedence: flag > env var > config.

### Listing unmatched scenes

```bash
fss stash unmatched
fss stash unmatched --performer "Bettie Bondage"
fss stash unmatched --studio "Some Studio"
```

Lists scenes in Stash with `stash_id_count == 0` (no StashDB metadata). Output is a table with ID, filename, title, and performers.

### Importing metadata

```bash
# Dry-run — shows what would change, writes nothing
fss stash import --dir ./data

# Apply changes
fss stash import --dir ./data --apply

# Load specific JSON files instead of a directory
fss stash import --json studio-a.json --json studio-b.json --apply

# Filter by performer and add resolution tags
fss stash import --dir ./data --performer "Bettie Bondage" --resolution-tags --apply

# Only process the first 50 Stash scenes (useful for testing)
fss stash import --dir ./data --top 50
```

**`--dir` vs `--json`:** By default, `--dir` loads every `*.json` file in the directory — all studios get pooled into one index. This is what you want when you've scraped a performer from multiple sites (e.g. ManyVids + Clips4Sale) and want cross-site merging. Use `--json` when you only want to import from specific files, for example a single studio.

### Matching strategy

FSS matches Stash scenes to FSS scenes by comparing each Stash scene's filename (minus extension) against FSS scene titles. Both sides are normalized: lowercased, non-alphanumeric characters replaced with spaces, trimmed.

**Two-pass matching:**

1. **Primary index** — match normalized filename against normalized titles:
   - Exact match (filename == title)
   - Substring match: all title words present in filename as whole words, and title covers ≥50% of filename words
2. **Sanitized index** — strip noise words (e.g. "step") from both filename and titles, then retry exact + substring. This handles cases where studios add "step-" prefixes that aren't in the filename.

**Duration filtering:** When the file's duration is known, candidates where the FSS scene duration differs by more than `max(10% of file duration, 30 seconds)` are rejected. This reduces false positives when multiple scenes have similar titles.

**Disambiguation:** When multiple substring matches tie on title length, the match is flagged as ambiguous and skipped.

Match confidence levels:

| Level | Meaning |
|-------|---------|
| **EXACT** | Normalized filename equals normalized title |
| **SUBSTR** | All title words are present in the filename (whole-word, ≥50% overlap). When multiple titles match, the longest (most specific) wins |
| **AMBIGUOUS** | Multiple distinct titles match with equal specificity — skipped |
| **SKIP** | No match found |

Dry-run output shows the confidence level for each match so you can verify before applying.

### Cross-site merging

When the same scene title appears in multiple FSS JSON files (e.g. scraped from both ManyVids and Clips4Sale), FSS merges them:

| Field | Strategy |
|-------|----------|
| URLs | Union of all site URLs |
| Date | Earliest non-zero date across all FSS sources AND the existing Stash date |
| Title | First non-empty |
| Description | Longest non-empty |
| Performers | Union (deduplicated) |
| Tags | Union (deduplicated) |
| Duration | Maximum |
| Resolution | Highest available |

### Tags

Every matched scene receives:

1. **Import marker tag** — `fss_import` by default (configurable via `--tag`). Applied to all imported scenes.
2. **FSS scene tags** — all tags and categories from the merged FSS scene, created in Stash if they don't exist.
3. **Resolution tag** (when `--resolution-tags` is set) — only the single highest applicable tag is added:
   - `4K Available` if width >= 3840
   - `Full HD Available` if width >= 1920 (and < 3840)
   - `HD Available` if width >= 1280 (and < 1920)

All tags are **additive** — existing Stash tags are never removed.

### StashDB override tracking

By default, scenes with existing StashDB metadata are skipped entirely. Pass `--include-stashbox` to also process them.

When `--include-stashbox` modifies a scene that has StashDB data:

1. The scene is tagged with `fss_stashbox_override` (configurable via `--stashbox-tag`) so you can filter and revert in Stash's UI
2. A JSON changelog entry is appended to `fss-stashbox-changelog.json` in the `--dir` directory (or `out_dir` from config, default `.`), recording:
   - Which Stash scene was modified
   - Which FSS scene it was matched to
   - What fields changed and their before/after values

The changelog is append-only — multiple import runs accumulate history. Example entry:

```json
{
  "stash_scene_id": "42",
  "timestamp": "2026-04-23T12:00:00Z",
  "filename": "scene.mp4",
  "matched_to": "Fostering the Bully",
  "changes": {
    "date": { "from": "2026-02-01", "to": "2026-01-01" },
    "urls": { "added": ["https://manyvids.com/..."] },
    "tags": { "added": ["JOI", "4K Available"] }
  }
}
```

### Optional Stash scraper

Pass `--scrape` to invoke Stash's built-in `scrapeSceneURL` on the first URL of each matched scene after import. This can pull additional metadata (performer images, etc.) from Stash's community scrapers. Off by default.
