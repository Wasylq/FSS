# FSS — FullStudioScraper

[![CI](https://github.com/Wasylq/FSS/actions/workflows/ci.yml/badge.svg)](https://github.com/Wasylq/FSS/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/Wasylq/FSS/graph/badge.svg)](https://codecov.io/gh/Wasylq/FSS)

Scrapes all scenes and metadata from a studio URL. Supports A POV Story, ManyVids, Clips4Sale, IWantClips, MyDirtyHobby, MissaX, Pornhub, Pure Taboo, Rachel Steele, Taboo Heat, Tara Tainton, and Mom Comes First. Designed to be easily extended to other sites. Can push scraped metadata into a local [Stash](https://stashapp.cc/) instance.

## Install

```bash
git clone https://github.com/Wasylq/FSS
cd FSS
go build -o fss .
```

## Quick start

```bash
# Scrape a ManyVids studio, output JSON (default)
fss scrape https://www.manyvids.com/Profile/590705/bettie-bondage/Store/Videos

# Scrape a Clips4Sale studio
fss scrape https://www.clips4sale.com/studio/27897/bettie-bondage

# Default behaviour: only fetch scenes not already in the output file (incremental)
fss scrape <url>

# Re-scrape everything from scratch
fss scrape --full <url>

# Re-fetch metadata for all known scenes; soft-delete any that have disappeared
fss scrape --refresh <url>

# Output both JSON and CSV into a specific directory, with 10 parallel workers
fss scrape --output json,csv --out ./data --workers 10 <url>

# Use SQLite as the store — enables price history tracking across scrapes and SQL queries
fss scrape --db ./fss.db --name "Bettie Bondage" <url>

# List all studios tracked in the database
fss list-studios --db ./fss.db

# See which sites are supported and their URL patterns
fss list-scrapers

# --- Stash integration ---

# List scenes in Stash that have no StashDB metadata
fss stash unmatched

# Dry-run: show what would be imported from FSS JSONs into Stash
fss stash import --dir ./data

# Actually apply the import
fss stash import --dir ./data --apply

# Filter by performer, connect to a remote Stash instance
fss stash import --url http://192.168.1.50:9999 --performer "Bettie Bondage" --apply
```

## Config file

An optional YAML config file sets defaults for all flags. Its location follows platform conventions:

| Platform | Path |
|----------|------|
| Linux    | `~/.config/fss/config.yaml` |
| macOS    | `~/Library/Application Support/fss/config.yaml` |
| Windows  | `%APPDATA%\fss\config.yaml` |

CLI flags always override config values.

```yaml
workers: 3        # parallel metadata fetchers
output: json      # json | csv | json,csv
out_dir: .        # output directory
db: ""            # SQLite path — empty disables SQLite

stash:
  url: "http://localhost:9999"
  api_key: ""     # or set FSS_STASH_API_KEY env var
  tag: "fss_import"
```

## Output

Without `--db`, each run produces one file per studio in the output directory:

- `<studio-slug>.json` — all scenes with full metadata
- `<studio-slug>.csv`  — same data in tabular form (if `--output csv`)

With `--db`, SQLite is the source of truth. JSON/CSV are exported from it on request.

See [MANUAL.md](MANUAL.md) for the full field reference, output format details, and advanced usage.
See [CONTRIBUTING.md](CONTRIBUTING.md) for how to add a new scraper.
