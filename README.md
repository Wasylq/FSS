# FSS — FullStudioScraper

[![CI](https://github.com/Wasylq/FSS/actions/workflows/ci.yml/badge.svg)](https://github.com/Wasylq/FSS/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/Wasylq/FSS/graph/badge.svg)](https://codecov.io/gh/Wasylq/FSS)
[![Go Report Card](https://goreportcard.com/badge/github.com/Wasylq/FSS)](https://goreportcard.com/report/github.com/Wasylq/FSS)

Scrapes all scenes and metadata from a studio URL. Designed to be easily extended to new sites. Can push scraped metadata into a local [Stash](https://stashapp.cc/) instance.

## Supported sites

| Site | Platform |
|------|----------|
| A POV Story | PHP tour site |
| Babes | Aylo/Juan |
| BrasilVR | POVR/WankzVR |
| Brazzers | Aylo/Juan |
| Clips4Sale | Clips4Sale |
| Anal Therapy | WordPress |
| Digital Playground | Aylo/Juan |
| Evil Angel | Gamma/Algolia |
| Fakings | Next.js RSC |
| Family Therapy | WordPress |
| Gloryhole Secrets | Gamma/Algolia |
| IWantClips | IWantClips |
| Kink | Kink |
| ManyVids | ManyVids |
| Mature.nl | Custom |
| MilfVR | POVR/WankzVR |
| MYLF | TeamSkeet/PSM |
| Pure Mature | AMA Multimedia |
| MissaX | Custom |
| Mofos | Aylo/Juan |
| Mom Comes First | WordPress |
| Mommy Blows Best | Gamma/Algolia |
| Burning Angel | Gamma/Algolia |
| Filthy Kings | Gamma/Algolia |
| Gangbang Creampie | Gamma/Algolia |
| Girlfriends Films | Gamma/Algolia |
| Lethal Hardcore | Gamma/Algolia |
| Naughty America | Naughty America |
| OopsFamily | FapHouse/Custom |
| Nubiles Network | EdgeCms |
| Perfect Girlfriend | WordPress |
| MyDirtyHobby | MyDirtyHobby |
| Pornhub | Pornhub |
| Pure Taboo | Gamma/Algolia |
| PropertySex | Aylo/Juan |
| Queensnake | Custom |
| Rachel Steele | MyMember.site |
| Reality Kings | Aylo/Juan |
| Rocco Siffredi | Gamma/Algolia |
| SexMex | SexMex Pro CMS |
| Sofie Marie | ELXComplete/Andomark |
| Exposed Latinas | SexMex Pro CMS |
| Trans Angels | Aylo/Juan |
| Trans Queens | SexMex Pro CMS |
| TranzVR | POVR/WankzVR |
| Taboo Heat | Gamma/Algolia |
| Tara Tainton | WordPress |
| Twistys | Aylo/Juan |
| WankzVR | POVR/WankzVR |
| Wicked | Gamma/Algolia |
| YourVids | YourVids |

See [docs/scrapers.md](docs/scrapers.md) for URL patterns and details.

## Install

Pick one — pre-built binary is easiest, Docker if you prefer containers, source if you want to hack on it.

### Option 1 — pre-built binary (recommended)

Download the archive for your platform from the [latest release](https://github.com/Wasylq/FSS/releases/latest), extract, and put the binary on your `PATH`. All binaries are static (no runtime dependencies).

Asset naming: `fss-<version>-<os>-<arch>.tar.gz` (or `.zip` for Windows). Available platforms: `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`, `windows/amd64`.

#### Linux

```bash
# Pick a version from https://github.com/Wasylq/FSS/releases/latest
VERSION=v1.6.0
ARCH=amd64    # or arm64 for Raspberry Pi 4/5, ARM cloud instances, etc.

curl -LO https://github.com/Wasylq/FSS/releases/download/${VERSION}/fss-${VERSION}-linux-${ARCH}.tar.gz
tar xzf fss-${VERSION}-linux-${ARCH}.tar.gz
sudo install -m 0755 fss /usr/local/bin/fss
fss version
```

If you don't have sudo, drop `fss` into `~/.local/bin/` (already on your `PATH` on most distros) instead.

#### macOS

```bash
VERSION=v1.6.0
ARCH=arm64    # Apple Silicon (M1+); use amd64 for Intel Macs

curl -LO https://github.com/Wasylq/FSS/releases/download/${VERSION}/fss-${VERSION}-darwin-${ARCH}.tar.gz
tar xzf fss-${VERSION}-darwin-${ARCH}.tar.gz
sudo install -m 0755 fss /usr/local/bin/fss

# Gatekeeper will block unsigned binaries the first time. Either:
xattr -d com.apple.quarantine /usr/local/bin/fss
# or: System Settings → Privacy & Security → "Open anyway" after the first failed run.

fss version
```

#### Windows (PowerShell)

```powershell
$Version = "v1.6.0"

Invoke-WebRequest -Uri "https://github.com/Wasylq/FSS/releases/download/$Version/fss-$Version-windows-amd64.zip" -OutFile fss.zip
Expand-Archive -Path fss.zip -DestinationPath .

# Move into a folder that's on your PATH, e.g.:
New-Item -ItemType Directory -Force -Path "$env:USERPROFILE\bin" | Out-Null
Move-Item -Force fss.exe "$env:USERPROFILE\bin\fss.exe"

# Add %USERPROFILE%\bin to your PATH (one-time):
[Environment]::SetEnvironmentVariable("Path", "$env:Path;$env:USERPROFILE\bin", "User")
# Restart your shell, then:
fss version
```

### Option 2 — Docker (multi-arch image on GHCR)

```bash
docker pull ghcr.io/wasylq/fss:latest
docker run --rm ghcr.io/wasylq/fss:latest list-scrapers
```

See [docs/docker.md](docs/docker.md) for volume conventions, the bind-mount UID gotcha, and a `docker compose` example with Stash.

### Option 3 — build from source

Requires Go 1.25+ (matches the `go` directive in `go.mod`).

```bash
git clone https://github.com/Wasylq/FSS
cd FSS
go build -o fss .
./fss version
```

Or via `go install` (binary lands in `$GOBIN`, typically `~/go/bin/`):

```bash
go install github.com/Wasylq/FSS@latest
```

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

See [`config.example.yaml`](config.example.yaml) for all available options with descriptions. To get started, grab it directly and drop it into place:

```bash
# Linux
mkdir -p ~/.config/fss
curl -fsSL https://raw.githubusercontent.com/Wasylq/FSS/master/config.example.yaml \
  -o ~/.config/fss/config.yaml

# macOS
mkdir -p ~/Library/Application\ Support/fss
curl -fsSL https://raw.githubusercontent.com/Wasylq/FSS/master/config.example.yaml \
  -o ~/Library/Application\ Support/fss/config.yaml
```

Or if you already have the repo cloned: `cp config.example.yaml ~/.config/fss/config.yaml`

CLI flags always override config values. See [docs/usage.md](docs/usage.md#config-file) for the full reference.

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
