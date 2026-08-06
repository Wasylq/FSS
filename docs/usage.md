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
For the metadata model and store contract, see [metadata.md](metadata.md).
For choosing a store, inspecting a database, and moving between the two, see [storage.md](storage.md).

---

## CLI flags

### Global flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--debug`, `-d` | count | 0 | Increase debug verbosity. Stackable: `-d` = high-level ops, `-dd` = HTTP requests, `-ddd` = parsing details |

### `fss scrape <studio-url>`

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--workers`, `-w` | int | 3 | Max parallel metadata fetchers |
| `--full` | bool | false | Full traversal (no early-stop); preserves price history; drops scenes no longer on the site |
| `--refresh` | bool | false | Re-fetch metadata for all known scenes; soft-delete missing ones |
| `--force` | bool | false | Allow a `--full`/`--refresh` that returns 0 scenes to wipe a previously-populated studio (otherwise the destructive save is refused) |
| `--no-preserve` | bool | false | Let a re-scrape blank fields it no longer returns. By default a fresh scene inherits any field it left empty from the stored one, so a broken parser cannot wipe metadata — see [metadata.md](metadata.md) |
| `--output`, `-o` | string | `json` | Export format(s): `json`, `csv`, or `json,csv` |
| `--out-dir` | string | `.` | Output directory |
| `--db` | string | _(disabled)_ | Enable SQLite store (`--db` alone uses `~/.local/share/fss/fss.db`; `--db=/path` uses a custom path — note the `=`, a space-separated value is not parsed) |
| `--delay` | int | `500` | Milliseconds to sleep between page requests (default from config; `--delay 0` disables) |
| `--site-delay` | []string | _(none)_ | Per-scraper delay overrides as `name=ms` pairs, e.g. `--site-delay manyvids=0,pornhub=2000` |
| `--name` | string | _(none)_ | Human-readable label for this studio (stored when `--db` is set) |

`--full` and `--refresh` are mutually exclusive.

**Per-site delay precedence:** `--site-delay <id>=N` (CLI) > `site_delays.<id>: N` (config) > `--delay`/`delay` (global). A site explicitly set to `0` disables delay even when the global default is non-zero. `--full` re-fetches every scene (carrying price history forward) and drops scenes no longer on the site. `--refresh` traverses the full scene list but re-uses existing IDs to update metadata in place and detect deletions.

### `fss list-scrapers`

Prints all registered scrapers and the URL patterns each one handles.

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--markdown` | bool | false | Output as a markdown table (useful for docs) |

### `fss list-studios`

Lists all studios in the SQLite database with scene counts and last-scraped timestamps. Needs a database: pass `--db`, or set `db:` in the config file and omit the flag.

### `fss import <file-or-dir> ...`

Loads studio JSON files into the SQLite database. Directories contribute their `*.json` entries (not recursive). Each file's own `studioUrl` decides which studio it belongs to — filenames are never parsed, since `Slugify` is lossy.

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--db` | string | _(from config)_ | Target database (`--db` alone uses the default path; `--db=/path` for a custom one) |
| `--replace` | bool | false | Make each file authoritative: delete stored scenes it does not contain |
| `--dry-run` | bool | false | Report what would be imported without writing |

The database schema is a superset of the JSON layout, so nothing is lost. By default the file is *merged* into the database: scenes present in both take the file's values, price history is carried forward, fields the file omits keep their stored values, and scenes only in the database are left alone.

```bash
fss import --db ./out/                      # every studio file in ./out
fss import --db=/custom/path.db studio.json # one file into a custom database
```

### `fss export [studio-url ...]`

Writes studio JSON/CSV files out of the SQLite database, named exactly as the flat store names them. With no arguments, every tracked studio is exported.

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--db` | string | _(from config)_ | Source database |
| `--output`, `-o` | string | `json` | Export format(s): `json`, `csv`, or `json,csv` |
| `--out-dir` | string | `.` | Output directory |

```bash
fss export --db --out-dir ./out -o json,csv
```

A JSON → database → JSON round trip is lossless except for sub-second timestamp precision, which SQLite has never stored (times are written as RFC 3339 to the second).

### Scene sources (`fss stash import`, `fss identify`)

Both commands read previously-scraped scenes. They share one set of flags for choosing where those scenes come from:

| `--json` | []string | _(none)_ | Specific JSON files to load |
| `--dir` | string | _(config `out_dir`)_ | Directory of FSS JSON files |
| `--db` | string | _(from config)_ | Load scenes from the SQLite store instead of JSON (`--db` alone uses the default path; `--db=/path` for a custom one). Cannot be combined with `--json`/`--dir` |
| `--from-studio` | []string | _(none)_ | Only use scenes from these studios. Accepts a studio URL, a studio display name, or a per-scene studio/sub-brand name. Repeatable — any match |
| `--from-performer` | []string | _(none)_ | Only use scenes featuring these performers. Repeatable — any match |

**Precedence:** `--json` → `--db` → `--dir` → the config's `db:` if set → the config's `out_dir`. Passing `--db` together with `--json` or `--dir` is an error rather than a silent winner. Each run prints the source it resolved to.

If you have `db:` in your config, these commands read the database by default — no need to pass `--db` everywhere. Export JSON only if you want it.

**How `--from-studio` resolves**, most specific first:

1. A value containing `://` matches the scene's studio URL exactly.
2. A value naming a per-scene studio matches **that sub-brand only**. Scrapers record a per-scene studio name, which for a network is the sub-brand — one scrape of `sexlikereal.com` carries 705 distinct values (`SLR Originals`, `perVRt`, …). This gives you one level of grouping even though FSS records no studio hierarchy.
3. Otherwise it is matched against the studios table's display names and selects every scene under those URLs.

Step 2 comes first deliberately: a studio's display name is often *derived* from its first scene, so it is frequently a sub-brand itself. Matching display names first would make `--from-studio "SLR Originals"` select the entire network.

Names are matched canonically — case-folded with whitespace collapsed — so `"ABC "`, `"abc"` and `"ABC"` are the same studio. This is the same rule `MergeScenes` uses to deduplicate names, so filtering and merging always agree.

A filter matching nothing is an **error listing the available names**, not an empty run.

**Combining filters:** repeated values of one flag are OR; different flags are AND. `--from-studio X --from-performer A` means scenes from X that also feature A.

Note `--from-studio`/`--from-performer` filter the **FSS metadata**, while `fss stash import`'s `--studio`/`--performer` filter which **Stash scenes** are queried. They apply to opposite sides of the match and combine as AND.

### `fss check <url>`

Checks whether a URL is supported by any registered scraper. Prints the scraper ID and its URL patterns if matched. If unsupported, prints a pre-filled link to open a new-scraper request issue on GitHub.

```bash
$ fss check https://www.brazzers.com/videos
Scraper:  brazzers
Patterns: brazzers.com, brazzers.com/pornstar/{id}/{slug}, ...

$ fss check https://example.com/unknown
Not supported: https://example.com/unknown

Request support: https://github.com/Wasylq/FSS/issues/new?template=new_scraper.yml&url=...
```

### `fss detect <url>`

Fetches a URL once and checks the response for known platform signals (Aylo `instance_token`, Algolia API, `psmcdn.net`, ModelCentro, etc.). Reports the detected platform and corresponding util package. Useful when deciding whether a new site needs a standalone scraper or belongs to an existing shared package.

```bash
$ fss detect https://www.teenmegaworld.net
Platform: Teen Mega World
Package:  tmwutil
```

If the URL is already supported by a registered scraper, it reports that instead of fetching.

### `fss doctor`

Checks environment health: config file, scraper registry, database writability, Stash connectivity, `ffprobe` availability, and network egress.

### `fss completion <bash|zsh|fish|powershell>`

Generates shell completion scripts. Usage:

```bash
source <(fss completion bash)           # bash
fss completion zsh > "${fpath[1]}/_fss" # zsh
fss completion fish | source            # fish
```

### `fss config init`

Creates a default config file at the XDG config path (e.g. `~/.config/fss/config.yaml`). Fails if the file already exists.

### `fss config path`

Prints the config file path for the current platform.

### `fss version`

Prints the build version, commit hash, and build date. Checks for newer releases on GitHub.

The update check is best-effort and never fails the command: a network error prints
`Could not check for updates: …`, and a response that carries no release tag prints
`Could not determine the latest release.` rather than advertising an update with nothing
after the arrow.

For `fss identify`, see [identify.md](identify.md).
For `fss stash` subcommands, see [stash.md](stash.md).

---

## Config file

Located at the XDG config path for your platform (see [README](../README.md)). All keys are optional — missing keys use the defaults shown below. A fully commented example is available at [`config.example.yaml`](../config.example.yaml) in the repo root.

```yaml
workers: 3        # int   — parallel metadata fetchers
output: json      # str   — json | csv | json,csv
out_dir: .        # str   — output directory path
db: ""            # str   — "" = flat store (explicit, survives the coming default flip);
                  #         "default" = ~/.local/share/fss/fss.db; or a path, absolute or
                  #         relative to the working directory (e.g. "./fss.db").
                  #         Omitting the key entirely is not the same as "" — see storage.md
delay: 500        # int   — ms between page requests; 0 disables
user_agent: ""    # str   — "firefox" (default), "chrome", or a custom UA string
notices: true     # bool  — advisory messages (e.g. an upcoming default change); false silences them

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

Two distinct studio URLs can sanitise to the same slug (e.g. `/foo-bar` and `/foo/bar`, or `/Foo` and `/foo`). The full `studioUrl` is stored inside the JSON, and the store refuses to load or overwrite a file whose stored URL doesn't match the one you're scraping — you'll see a `slug collision` error. Rename or move one of the studio files to resolve.

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

**Important:** all scenes are collected in memory first, then the entire JSON file is written at the end of the scrape. If you cancel mid-scrape (Ctrl+C), no output file is produced. For large sites (e.g. ~1750 pages), a scrape can take several minutes — use `--delay` to throttle requests and avoid being blocked. The progress line (`fetching: N / total scenes`) and the final `Done:` / `Partial save complete:` line both include elapsed wall-clock time. Note that `--delay` paces each worker individually, not the aggregate request rate — a high `--workers` count can still overwhelm a rate-limited site even with a non-zero delay, so lower `--workers` first if a site starts timing out mid-scrape.

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

Full traversal — no early-stop hint. Fetches every page and every scene.

**Price history is preserved** — existing price snapshots are carried forward and the new snapshot is appended.

Differs from `--refresh` in one way: scenes that no longer appear on the site are dropped from the store rather than soft-deleted. Use when you want a clean slate of "what's currently on the site" without losing pricing history.

### `--refresh`

Full traversal (every page, every scene) but preserves history:

- **Price history** from prior scrapes is carried forward — each re-fetched scene gets the new price snapshot appended to its existing history.
- **Soft-delete** — any scene that was in the store but is no longer returned by the site has its `deletedAt` timestamp set. It is never removed from the store.

Use periodically (e.g. weekly) to catch deletions and accumulate accurate price history.

---

## SQLite

### Enabling

Pass `--db` to any scrape command, or set `db` in your config file:

```bash
fss scrape --db <studio-url>                  # uses default path: ~/.local/share/fss/fss.db
fss scrape --db=/custom/path.db <studio-url>  # uses a custom path — the `=` is required
```

Or in `config.yaml`:

```yaml
db: "default"           # uses ~/.local/share/fss/fss.db
db: "/custom/path.db"   # uses a custom path
```

The data directory is created automatically if it doesn't exist. When `--db` is set, SQLite is the source of truth. JSON/CSV files are exported from it if `--output` requests them.

### Schema

Eleven tables (three core + six junction/lookup + one metadata + `schema_version`),
at schema version 8. Inspect with any SQLite client (`sqlite3`, DB Browser for
SQLite, DBeaver, Datasette, …). Migrations run automatically on open.

**`scenes`** — one row per scene. Primary key is **`(id, site_id, studio_url)`**:
the same scene reachable from two studio URLs is stored once per URL, so the
studio URL is part of the identity.

| Column | Type | Notes |
|--------|------|-------|
| `id` | TEXT | Site-specific ID (part of the primary key) |
| `site_id` | TEXT | e.g. `manyvids` (part of the primary key) |
| `studio_url` | TEXT | Part of the primary key; indexed |
| `title` | TEXT | |
| `url` | TEXT | |
| `date` | TEXT | RFC3339 |
| `description` | TEXT | |
| `thumbnail` | TEXT | Remote URL; often a short-lived signed CDN link |
| `preview` | TEXT | |
| `performers` | TEXT | **Legacy JSON array — no longer read or written.** Use `scene_performers` |
| `director` | TEXT | |
| `studio` | TEXT | |
| `tags` | TEXT | **Legacy JSON array — no longer read or written.** Use `scene_tags` |
| `categories` | TEXT | **Legacy JSON array — no longer read or written.** Use `scene_categories` |
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
| `first_seen_at` | TEXT | RFC3339 — when the scene entered the store; never moves once set |
| `scraped_at` | TEXT | RFC3339 — when it was last fetched |
| `deleted_at` | TEXT | RFC3339, nullable — NULL means active |
| `content_hash` | TEXT | Internal. Fingerprint of everything above except `scraped_at`/`first_seen_at`, so `Save` can skip unchanged rows. **Do not hand-edit** — a stale value makes a scene invisible to updates. Setting it to `''` forces a rewrite |

**`price_history`** — one row per price snapshot per scene:

| Column | Type | Notes |
|--------|------|-------|
| `id` | INTEGER | autoincrement |
| `scene_id` | TEXT | with `site_id`, `studio_url` identifies the scene |
| `site_id` | TEXT | |
| `studio_url` | TEXT | |
| `date` | TEXT | RFC3339 |
| `regular` | REAL | |
| `discounted` | REAL | |
| `is_free` | INTEGER | 0/1 |
| `is_on_sale` | INTEGER | 0/1 |
| `discount_percent` | INTEGER | |

**`studios`** — one row per studio URL:

| Column | Type | Notes |
|--------|------|-------|
| `url` | TEXT | Primary key |
| `site_id` | TEXT | e.g. `manyvids` |
| `name` | TEXT | Label via `--name`; never cleared by a scrape that omits `--name` |
| `added_at` | TEXT | RFC3339 — when first scraped |
| `last_scraped_at` | TEXT | RFC3339, nullable |

**Normalized lookup tables** — performers, tags and categories live in their own
tables with junction tables linking them to scenes, so the data is queryable
without JSON parsing. The lookup tables are **global**: one row per distinct
name, shared across every studio. That is deliberate deduplication.

**`performers`** / **`tags`** / **`categories`**:

| Column | Type |
|--------|------|
| `id` | INTEGER (autoincrement) |
| `name` | TEXT (unique) |

**`scene_performers`** / **`scene_tags`** / **`scene_categories`** — all three have
the same shape, including `position`:

| Column | Type | Notes |
|--------|------|-------|
| `scene_id` | TEXT | FK to scenes |
| `site_id` | TEXT | FK to scenes |
| `studio_url` | TEXT | FK to scenes — always join on all three |
| `<entity>_id` | INTEGER | FK to `performers` / `tags` / `categories` |
| `position` | INTEGER | Source order (0 = first). Scrapers emit meaningful order and it is preserved |

**`scene_external_ids`** — cross-site identity, e.g. a StashDB UUID:

| Column | Type | Notes |
|--------|------|-------|
| `scene_id` | TEXT | FK to scenes |
| `site_id` | TEXT | FK to scenes |
| `studio_url` | TEXT | FK to scenes |
| `source` | TEXT | `stashdb`, `tpdb`, `iafd`, `indexxx`, or a stashbox site ID |
| `external_id` | TEXT | The ID in that database |

Indexed on `(source, external_id)`, so "which stored scenes are this StashDB
UUID" is answerable across every site and studio.

**`schema_version`** — tracks migration state (single `version INTEGER` column).

> **Joining scene child rows:** always match on **all three** of `scene_id`,
> `site_id` *and* `studio_url`. Dropping `studio_url` silently merges rows from
> different studios that happen to share a scene ID.

### Listing studios

```bash
fss list-studios --db              # uses default db location
fss list-studios --db ./fss.db     # uses a custom path
fss list-studios                   # uses the config file's `db:` value
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
JOIN price_history ph
  ON ph.scene_id = s.id AND ph.site_id = s.site_id AND ph.studio_url = s.studio_url
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

