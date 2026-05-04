# Usage Manual

Full technical reference. Use the section links to jump to what you need.

- [CLI flags](#cli-flags)
- [Config file](#config-file)
- [Data model](#data-model)
- [Output formats](#output-formats)
- [Modifying a scraper](#modifying-a-scraper)
- [Resume and update behaviour](#resume-and-update-behaviour)
- [SQLite](#sqlite)

For Stash integration, see [stash.md](stash.md).
For NFO sidecar file generation, see [identify.md](identify.md).

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
| `--delay` | int | `0` | Milliseconds to sleep between page requests (applies to sites without a `--site-delay` override) |
| `--site-delay` | []string | _(none)_ | Per-scraper delay overrides as `name=ms` pairs, e.g. `--site-delay manyvids=0,pornhub=2000` |
| `--name` | string | _(none)_ | Human-readable label for this studio (stored when `--db` is set) |

`--full` and `--refresh` are mutually exclusive.

**Per-site delay precedence:** `--site-delay <id>=N` (CLI) > `site_delays.<id>: N` (config) > `--delay`/`delay` (global). A site explicitly set to `0` disables delay even when the global default is non-zero. `--full` ignores all existing data. `--refresh` traverses the full scene list but re-uses existing IDs to update metadata in place and detect deletions.

### `fss list-scrapers`

Prints all registered scrapers and the URL patterns each one handles. No flags.

### `fss list-studios`

Lists all studios in the SQLite database with scene counts and last-scraped timestamps. Requires `--db`.

### `fss version`

Prints the build version, commit hash, and build date. Checks for newer releases on GitHub.

For `fss identify`, see [identify.md](identify.md).
For `fss stash` subcommands, see [stash.md](stash.md).

---

## Config file

Located at the XDG config path for your platform (see [README](../README.md)). All keys are optional — missing keys use the defaults shown below. A fully commented example is available at [`config.example.yaml`](../config.example.yaml) in the repo root.

```yaml
workers: 3        # int   — parallel metadata fetchers
output: json      # str   — json | csv | json,csv
out_dir: .        # str   — output directory path
db: ""            # str   — SQLite path; empty string disables SQLite
delay: 0          # int   — ms between page requests; 0 disables

site_delays:      # map[string]int — per-scraper delay overrides (overrides `delay` for matching sites)
  # manyvids: 0
  # pornhub: 2000
  # brazzers: 500

stashbox:         # list — stashbox instances for the stashbox scraper
  # - url: "https://stashdb.org/graphql"       # GraphQL endpoint URL
  #   api_key: "your-api-key-here"             # API key for this instance
  # - url: "https://pmvstash.org/graphql"
  #   api_key: "another-api-key"

stash:
  url: "http://localhost:9999"    # str   — Stash server URL
  api_key: ""                     # str   — API key (prefer FSS_STASH_API_KEY env var)
  tag: "fss_import"               # str   — import marker tag
  stashbox_tag: "fss_stashbox_override"  # str — tag for StashDB override tracking
  resolution_tags: true           # bool  — add 4K/FHD/HD Available tags
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

**Important:** all scenes are collected in memory first, then the entire JSON file is written at the end of the scrape. If you cancel mid-scrape (Ctrl+C), no output file is produced. For large sites (e.g. ~1750 pages), a scrape can take several minutes — use `--delay` to throttle requests and avoid being blocked.

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

## Modifying a scraper

### Adding a new field to `models.Scene`

1. **`models/scene.go`** — add the field with JSON tag
2. **The scraper** — populate the field in `toScene()` (or equivalent mapping function)
3. **`internal/store/export_csv.go`** — add the column name to `csvHeaders` and the value to `sceneToRow()`
4. **`docs/usage.md`** — add a row to the CSV column table and the data model table (this file)
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

Ten tables (three core + six junction/lookup + one metadata). Inspect with any SQLite client (`sqlite3`, DBeaver, TablePlus, etc.).

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

**Normalized lookup tables** — performers, tags, and categories are stored in dedicated tables with junction tables linking them to scenes. This makes the data fully queryable without JSON parsing. The old JSON columns (`performers`, `tags`, `categories`) in the `scenes` table are kept for compatibility but are no longer used for reads.

**`performers`** — deduplicated performer names:

| Column | Type |
|--------|------|
| `id` | INTEGER (autoincrement) |
| `name` | TEXT (unique) |

**`tags`** / **`categories`** — same structure as `performers`.

**`scene_performers`** — links scenes to performers:

| Column | Type | Notes |
|--------|------|-------|
| `scene_id` | TEXT | FK to scenes |
| `site_id` | TEXT | FK to scenes |
| `performer_id` | INTEGER | FK to performers |
| `position` | INTEGER | Listing order (0 = first billed) |

**`scene_tags`** / **`scene_categories`** — same structure without `position`.

**`schema_version`** — tracks migration state (single `version INTEGER` column).

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

-- Scenes with a specific tag (via junction table)
SELECT s.title
FROM scenes s
JOIN scene_tags st ON s.id = st.scene_id AND s.site_id = st.site_id
JOIN tags t ON st.tag_id = t.id
WHERE t.name = 'MILF'
  AND s.deleted_at IS NULL;

-- All performers for a scene (ordered by billing)
SELECT p.name
FROM scene_performers sp
JOIN performers p ON sp.performer_id = p.id
WHERE sp.scene_id = '7342578' AND sp.site_id = 'manyvids'
ORDER BY sp.position;

-- Scenes by performer (across all sites)
SELECT s.title, s.site_id, s.date
FROM scenes s
JOIN scene_performers sp ON s.id = sp.scene_id AND s.site_id = sp.site_id
JOIN performers p ON sp.performer_id = p.id
WHERE p.name = 'Rachel Steele'
  AND s.deleted_at IS NULL
ORDER BY s.date DESC;

-- Most common tags
SELECT t.name, COUNT(*) AS scene_count
FROM scene_tags st
JOIN tags t ON st.tag_id = t.id
GROUP BY t.name
ORDER BY scene_count DESC
LIMIT 20;
```

