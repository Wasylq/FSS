# FSS — FullStudioScraper

[![CI](https://github.com/Wasylq/FSS/actions/workflows/ci.yml/badge.svg)](https://github.com/Wasylq/FSS/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/Wasylq/FSS/graph/badge.svg)](https://codecov.io/gh/Wasylq/FSS)

Scrapes all scenes and metadata from a studio URL. Designed to be easily extended to new sites. Can push scraped metadata into a local [Stash](https://stashapp.cc/) instance.

## Supported sites

| Site | Platform |
|------|----------|
| A POV Story | PHP tour site |
| Babes | Aylo/Juan |
| Brazzers | Aylo/Juan |
| Clips4Sale | Clips4Sale |
| Digital Playground | Aylo/Juan |
| IWantClips | IWantClips |
| ManyVids | ManyVids |
| MissaX | Custom |
| Mofos | Aylo/Juan |
| Mom Comes First | WordPress |
| Naughty America | Naughty America |
| Nubiles Network | EdgeCms |
| MyDirtyHobby | MyDirtyHobby |
| Pornhub | Pornhub |
| Pure Taboo | Gamma/Algolia |
| Rachel Steele | MyMember.site |
| Reality Kings | Aylo/Juan |
| Taboo Heat | Gamma/Algolia |
| Tara Tainton | WordPress |

See [docs/scrapers.md](docs/scrapers.md) for URL patterns and details.

## Install

```bash
git clone https://github.com/Wasylq/FSS
cd FSS
go build -o fss .
```

Or via Docker (multi-arch image on GHCR):

```bash
docker pull ghcr.io/wasylq/fss:latest
docker run --rm ghcr.io/wasylq/fss:latest list-scrapers
```

See [docs/docker.md](docs/docker.md) for volume conventions and a `docker compose` example.

## Quick start

```bash
# Scrape a studio — outputs JSON by default
fss scrape https://www.manyvids.com/Profile/590705/bettie-bondage/Store/Videos

# Incremental mode (default): only fetches new scenes
fss scrape <url>

# Full re-scrape from scratch
fss scrape --full <url>

# Re-fetch all metadata, soft-delete removed scenes
fss scrape --refresh <url>

# Output both JSON and CSV
fss scrape --output json,csv --out ./data <url>

# Use SQLite as the store
fss scrape --db ./fss.db --name "Bettie Bondage" <url>

# See supported sites
fss list-scrapers
```

### Stash integration

```bash
# List unmatched scenes in Stash
fss stash unmatched

# Dry-run: show what would be imported
fss stash import --dir ./data

# Apply changes
fss stash import --dir ./data --apply
```

## Config file

Optional YAML config at the platform-specific path:

| Platform | Path |
|----------|------|
| Linux    | `~/.config/fss/config.yaml` |
| macOS    | `~/Library/Application Support/fss/config.yaml` |
| Windows  | `%APPDATA%\fss\config.yaml` |

```yaml
workers: 3
output: json
out_dir: .
db: ""

stash:
  url: "http://localhost:9999"
  api_key: ""
  tag: "fss_import"
```

CLI flags always override config values.

## Documentation

| Document | Contents |
|----------|----------|
| [docs/scrapers.md](docs/scrapers.md) | Supported sites, URL patterns, shared packages |
| [docs/usage.md](docs/usage.md) | CLI reference, data model, output formats, SQLite |
| [docs/stash.md](docs/stash.md) | Stash integration: matching, merging, import workflow |
| [docs/docker.md](docs/docker.md) | Running FSS in Docker — image tags, volumes, compose examples |
| [docs/architecture.md](docs/architecture.md) | System design, plugin registry, streaming model, store abstraction |
| [CONTRIBUTING.md](CONTRIBUTING.md) | How to add a new scraper, reference implementations |
| [SECURITY.md](SECURITY.md) | Credential handling, network policy, vulnerability reporting |
