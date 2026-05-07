# FSS — FullStudioScraper

[![CI](https://github.com/Wasylq/FSS/actions/workflows/ci.yml/badge.svg)](https://github.com/Wasylq/FSS/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/Wasylq/FSS/graph/badge.svg)](https://codecov.io/gh/Wasylq/FSS)
[![Go Report Card](https://goreportcard.com/badge/github.com/Wasylq/FSS)](https://goreportcard.com/report/github.com/Wasylq/FSS)
[![Go Reference](https://pkg.go.dev/badge/github.com/Wasylq/FSS.svg)](https://pkg.go.dev/github.com/Wasylq/FSS)

Scrapes all scenes and metadata from a studio URL. Designed to be easily extended to new sites. Can push scraped metadata into a local [Stash](https://stashapp.cc/) instance.

## Supported sites

**453 sites** across 15+ platforms — [full list with URL patterns →](docs/scrapers.md)

| Platform | Count | Examples |
|----------|------:|---------|
| Score Group | 93 | Scoreland, XL Girls, 18Eighteen, Leg Sex, Naughty Mag, 50 Plus MILFs, … |
| Gamma/Algolia | 14 | Burning Angel, Dogfart Network, Evil Angel, OpenLife, Pure Taboo, Wicked, … |
| WP video-elements | 12 | DaughterSwap, MommysBoy, PervMom, SisLovesMe, … |
| SpiceVids collections | 1001 | AdamAndEveVOD, Crunchboy, 1111Customs, AdultTime, NewSensations, … (Aylo marketplace) |
| Aylo/Juan | 21 | Babes, BangBros, BigStr, Brazzers, Erito, HentaiPros, Killergram, LetsDoeIt, Metro, MileHigh, Reality Kings, Sean Cody, SpiceVids, WhyNotBi, … |
| WordPress | 5 | Anal Therapy, Family Therapy, Tara Tainton, … |
| POVR/WankzVR | 4 | BrasilVR, MilfVR, TranzVR, WankzVR |
| Up-Timely CMS | 5 | DAS!, Idea Pocket, Madonna, MOODYZ, S1 NO.1 STYLE |
| SexMex Pro CMS | 3 | Exposed Latinas, SexMex, Trans Queens |
| ModelCentro | 26 | Penny Barber, The Jerky Girls, Mugur Porn, Thicc Vision, Cum Trainer, Monster Males, … |
| Railway/Express | 3 | Smoking Erotica, Smoking Models, Spanking Glamour |
| Stashbox | 1+ | StashDB, any stashbox instance (config-driven) |
| Standalone | 47 | APClips, Clips4Sale, Crunchboy, DEEP'S, Fakings, Glory Quest, GrandparentsX, Jules Jordan, Kink, KM Produce, Lucas Entertainment, ManyVids, Naked News, Pornhub, Pure CFNM, r18.dev, Takara TV, VENUS, … |

## Install

Pick one — pre-built binary is easiest, system packages for auto-updates, Docker if you prefer containers, source if you want to hack on it.

### Option 1 — pre-built binary (recommended)

Download the archive for your platform from the [latest release](https://github.com/Wasylq/FSS/releases/latest), extract, and put the binary on your `PATH`. All binaries are static (no runtime dependencies).

Asset naming: `fss-<version>-<os>-<arch>.tar.gz` (or `.zip` for Windows). Available platforms: `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`, `windows/amd64`.

#### Linux

```bash
# Resolves to the latest tag (e.g. v1.11.0) by following the GitHub redirect.
VERSION=$(curl -sIL -o /dev/null -w '%{url_effective}' https://github.com/Wasylq/FSS/releases/latest | sed 's|.*/||')
ARCH=amd64    # or arm64 for Raspberry Pi 4/5, ARM cloud instances, etc.

curl -LO https://github.com/Wasylq/FSS/releases/download/${VERSION}/fss-${VERSION}-linux-${ARCH}.tar.gz
tar xzf fss-${VERSION}-linux-${ARCH}.tar.gz
sudo install -m 0755 fss /usr/local/bin/fss
fss version
```

If you don't have sudo, drop `fss` into `~/.local/bin/` (already on your `PATH` on most distros) instead.

#### macOS

```bash
VERSION=$(curl -sIL -o /dev/null -w '%{url_effective}' https://github.com/Wasylq/FSS/releases/latest | sed 's|.*/||')
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
$Version = (Invoke-RestMethod -Uri "https://api.github.com/repos/Wasylq/FSS/releases/latest").tag_name

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

### Option 2 — system package (.deb / .rpm / AUR)

Each release also publishes `.deb` and `.rpm` packages. Download them from the [latest release](https://github.com/Wasylq/FSS/releases/latest).

```bash
TAG=$(curl -sIL -o /dev/null -w '%{url_effective}' https://github.com/Wasylq/FSS/releases/latest | sed 's|.*/||')
VERSION=${TAG#v}    # asset filenames drop the leading "v"
ARCH=amd64          # or arm64

# Debian / Ubuntu
curl -LO "https://github.com/Wasylq/FSS/releases/download/${TAG}/fss_${VERSION}_${ARCH}.deb"
sudo dpkg -i "fss_${VERSION}_${ARCH}.deb"

# Fedora / RHEL
curl -LO "https://github.com/Wasylq/FSS/releases/download/${TAG}/fss-${VERSION}-1.x86_64.rpm"
sudo rpm -i "fss-${VERSION}-1.x86_64.rpm"
```

#### Arch Linux (AUR)

```bash
yay -S fss
```

### Option 3 — Docker (multi-arch image on GHCR)

```bash
docker pull ghcr.io/wasylq/fss:latest
docker run --rm ghcr.io/wasylq/fss:latest list-scrapers
```

See [docs/docker.md](docs/docker.md) for volume conventions, the bind-mount UID gotcha, and a `docker compose` example with Stash.

### Option 4 — build from source

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

### NFO sidecar files

```bash
# Dry-run: match video files against FSS metadata
fss identify /path/to/videos --dir ./data

# Write .nfo files next to matched videos
fss identify /path/to/videos --dir ./data --apply

# Optional: install ffprobe (FFmpeg) for duration-based disambiguation
sudo apt install ffmpeg  # or: brew install ffmpeg
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
| [docs/identify.md](docs/identify.md) | NFO sidecar file generation: matching videos, writing .nfo files |
| [docs/stash.md](docs/stash.md) | Stash integration: matching, merging, import workflow |
| [docs/docker.md](docs/docker.md) | Running FSS in Docker — image tags, volumes, compose examples |
| [docs/library.md](docs/library.md) | Using FSS as a Go library — registry API, streaming results, Scene model |
| [docs/architecture.md](docs/architecture.md) | System design, plugin registry, streaming model, store abstraction |
| [CONTRIBUTING.md](CONTRIBUTING.md) | How to add a new scraper, reference implementations |
| [SECURITY.md](SECURITY.md) | Credential handling, network policy, vulnerability reporting |
