# FSS — FullStudioScraper

Scrapes all scenes and metadata from a studio URL. Currently supports ManyVids; designed to be easily extended to other sites.

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
```

## Output

Without `--db`, each run produces one file per studio in the output directory:

- `<studio-slug>.json` — all scenes with full metadata
- `<studio-slug>.csv`  — same data in tabular form (if `--output csv`)

With `--db`, SQLite is the source of truth. JSON/CSV are exported from it on request.

See [MANUAL.md](MANUAL.md) for the full field reference, output format details, and how to add a new scraper.
