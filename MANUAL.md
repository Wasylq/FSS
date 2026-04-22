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
sort: date        # str   — date | featured
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

_Documented in Phase 2._

---

## Adding a scraper

_Documented in Phase 3 — will use ManyVids as the fully worked example._

**Short version:**

1. Create `internal/scrapers/<site>/<site>.go`
2. Import `github.com/Wasylq/FSS/scraper` and `github.com/Wasylq/FSS/models` (both public)
3. Implement the `scraper.StudioScraper` interface (4 methods: `ID`, `Patterns`, `MatchesURL`, `ListScenes`)
4. Register in an `init()`: `scraper.Register(&Scraper{})`
5. Add a blank import in `main.go` so `init()` runs: `_ "github.com/Wasylq/FSS/internal/scrapers/<site>"`

Nothing else changes. External projects can also implement `scraper.StudioScraper` and register their own scrapers by importing the public `scraper` package.

---

## Modifying a scraper

_Documented in Phase 3._

---

## Resume and update behaviour

_Documented in Phase 5._

---

## SQLite

_Documented in Phase 4._
