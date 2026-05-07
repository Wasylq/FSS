# Identify — NFO Sidecar Files

FSS can match a directory of video files against scraped metadata and write `.nfo` sidecar files alongside each matched video. The `.nfo` files can be picked up by Stash via the [community NFO scraper](https://github.com/stashapp/CommunityScrapers/tree/master/scrapers/nfo), or by any media manager that reads Kodi-style NFO metadata.

## Workflow

1. Scrape studios as usual: `fss scrape <url>` — produces JSON files
2. Point `fss identify` at a directory of video files and the JSON metadata
3. Review the dry-run output
4. Run again with `--apply` to write `.nfo` files
5. In Stash: run the **Identify** task with the NFO scraper as a source to bulk-import the metadata

## Quick start

```bash
# Dry-run — see what would match
fss identify /path/to/videos --json studio.json

# Write .nfo files
fss identify /path/to/videos --json studio.json --apply

# Cross-site merge: load multiple JSON files for richer metadata
fss identify /path/to/videos --json manyvids.json --json clips4sale.json --apply

# Load all JSON files from a directory
fss identify /path/to/videos --dir ./data --apply

# Overwrite existing .nfo files
fss identify /path/to/videos --json studio.json --apply --force

# Suppress the report file
fss identify /path/to/videos --json studio.json --apply --no-report
```

## Flags

### `fss identify <video-dir>`

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--json` | []string | _(none)_ | FSS JSON files to load |
| `--dir` | string | config `out_dir` | Directory containing FSS JSON files (loads all `*.json`) |
| `--apply` | bool | `false` | Actually write `.nfo` files (default is dry-run) |
| `--force` | bool | `false` | Overwrite existing `.nfo` files |
| `--no-report` | bool | `false` | Do not write `fss-report.txt` |

The positional argument `<video-dir>` is required — the directory of video files to scan.

`--json` and `--dir` work the same way as in `fss stash import`: `--json` loads specific files, `--dir` loads every `*.json` file in a directory. If neither is specified, the configured `out_dir` is used.

## Video discovery

The video directory is scanned **recursively**. Files are identified by extension:

`.mp4`, `.mkv`, `.avi`, `.wmv`, `.mov`, `.flv`, `.webm`, `.m4v`, `.mpg`, `.mpeg`, `.ts`

No magic-byte sniffing — extension-based detection is fast, works on network shares, and is consistent across platforms.

## Duration filtering

If `ffprobe` (part of FFmpeg) is installed and on `PATH`, `fss identify` probes each video file for its duration and uses it to disambiguate same-title scenes. This is optional — matching works without it, but duration filtering reduces false positives when multiple scenes share similar titles.

Install FFmpeg to enable this:

```bash
# Debian/Ubuntu
sudo apt install ffmpeg

# macOS
brew install ffmpeg

# Windows (winget)
winget install FFmpeg
```

## Matching

The same three-pass matching engine used by `fss stash import` matches each video filename against FSS scene titles. See [stash.md — Matching strategy](stash.md#matching-strategy) for details.

When multiple JSON files are loaded, **cross-site merging** applies: if the same scene title appears in both ManyVids and Clips4Sale JSON files, the metadata is merged — URLs are unioned, the earliest date is picked, performers and tags are combined.

## NFO format

Each `.nfo` file is written next to the video with the same basename (`scene.mp4` → `scene.nfo`):

```xml
<?xml version="1.0" encoding="UTF-8"?>
<movie>
  <title>Scene Title</title>
  <url>https://manyvids.com/Video/123/...</url>
  <url>https://clips4sale.com/studio/456/...</url>
  <premiered>2024-01-15</premiered>
  <plot>Scene description if available.</plot>
  <studio>Studio Name</studio>
  <thumb aspect="poster">https://example.com/cover.jpg</thumb>
  <actor>
    <name>Performer One</name>
  </actor>
  <actor>
    <name>Performer Two</name>
  </actor>
  <tag>Tag One</tag>
  <tag>Tag Two</tag>
</movie>
```

**Field mapping:**

| FSS field | NFO element | Notes |
|-----------|-------------|-------|
| `Title` | `<title>` | |
| `URL` | `<url>` | Multiple if cross-site merged |
| `Date` | `<premiered>` | |
| `Description` | `<plot>` | Empty if the scraper didn't produce one |
| `Studio` | `<studio>` | |
| `Thumbnail` | `<thumb aspect="poster">` | URL — may expire, but gives Stash a chance to fetch it |
| `Performers` | `<actor><name>` | One `<actor>` block per performer |
| `Tags` | `<tag>` | One element per tag |

## Existing `.nfo` files

If an `.nfo` file already exists for a video, the default behavior is to **warn and skip** it. The skipped file is logged in the console output and in the report. Pass `--force` to overwrite.

## Report file

By default, `fss identify` writes `fss-report.txt` in the video directory listing:
- **Unmatched** — video files with no FSS match
- **Skipped** — video files where an `.nfo` already existed

The report is not written if all files matched successfully. Pass `--no-report` to disable.

## Stash setup

To use the `.nfo` files in Stash:

1. **Install the NFO scraper**: In Stash, go to **Settings > Metadata Providers** and install "NFO Metadata Reader" from the community scrapers.
2. **Run the Identify task**: Go to **Settings > Tasks > Identify**, add "NFO Metadata Reader" as a source, and run it. Stash processes each scene in bulk — it finds the `.nfo` file by matching the filename and imports the metadata.

The NFO scraper requires Python on the Stash machine. The community scrapers package manager handles the `py_common` dependency automatically.
