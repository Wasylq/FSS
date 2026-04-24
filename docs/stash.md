# Stash Integration

FSS can push scraped metadata into a local [Stash](https://stashapp.cc/) instance, matching scenes by filename against FSS JSON output.

## Workflow

1. Scrape studios as usual: `fss scrape <url>` — produces JSON files
2. Download videos and add them to Stash (outside FSS)
3. List unmatched scenes: `fss stash unmatched`
4. Import metadata: `fss stash import --dir ./data` (dry-run first, then `--apply`)

## Connecting to Stash

By default, FSS connects to `http://localhost:9999`. Override with `--url` or the `stash.url` config key.

If your Stash instance requires authentication, provide an API key via:
- `--api-key` flag
- `FSS_STASH_API_KEY` environment variable
- `stash.api_key` config key

Precedence: flag > env var > config.

## CLI flags

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

## Listing unmatched scenes

```bash
fss stash unmatched
fss stash unmatched --performer "Bettie Bondage"
fss stash unmatched --studio "Some Studio"
```

Lists scenes in Stash with `stash_id_count == 0` (no StashDB metadata). Output is a table with ID, filename, title, and performers.

## Importing metadata

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

## Matching strategy

FSS matches Stash scenes to FSS scenes by comparing each Stash scene's filename (minus extension) against FSS scene titles. Both sides are normalized: camelCase boundaries split into words (e.g. `SunnyDayAtTheBeach` → `sunny day at the beach`), format suffixes stripped (e.g. `(FULL HD)`, `(mp4)`, `(mov)`), lowercased, non-alphanumeric characters replaced with spaces, trimmed.

**Two-pass matching:**

1. **Primary index** — match normalized filename against normalized titles:
   - Exact match (filename == title)
   - Substring match: all title words present in filename as whole words, and title covers >=50% of filename words
2. **Sanitized index** — strip noise words (e.g. "step") from both filename and titles, then retry exact + substring. This handles cases where studios add "step-" prefixes that aren't in the filename.

**Duration filtering:** When the file's duration is known, candidates where the FSS scene duration differs by more than `max(10% of file duration, 30 seconds)` are rejected. This reduces false positives when multiple scenes have similar titles.

**Disambiguation:** When multiple substring matches tie on title length, the match is flagged as ambiguous and skipped.

Match confidence levels:

| Level | Meaning |
|-------|---------|
| **EXACT** | Normalized filename equals normalized title |
| **SUBSTR** | All title words are present in the filename (whole-word, >=50% overlap). When multiple titles match, the longest (most specific) wins |
| **AMBIGUOUS** | Multiple distinct titles match with equal specificity — skipped |
| **SKIP** | No match found |

Dry-run output shows the confidence level for each match so you can verify before applying.

## Cross-site merging

When the same scene title appears in multiple FSS JSON files (e.g. scraped from both ManyVids and Clips4Sale), FSS merges them:

| Field | Strategy |
|-------|----------|
| URLs | Union of all site URLs |
| Date | Earliest non-zero date across all FSS sources AND the existing Stash date |
| Title | First non-empty |
| Description | Longest non-empty; runs of 3+ spaces are converted to newlines |
| Cover image | First available thumbnail URL; downloaded and pushed as base64 |
| Performers | Union (deduplicated) |
| Tags | Union (deduplicated) |
| Duration | Maximum |
| Resolution | Highest available |

Format suffix stripping means that Clips4Sale scenes listed as separate formats (e.g. `"Title (FULL HD)"`, `"Title (mp4)"`, `"Title (mov)"`) are merged together, combining tags from all versions.

## Tags

Every matched scene receives:

1. **Import marker tag** — `fss_import` by default (configurable via `--tag`). Applied to all imported scenes.
2. **FSS scene tags** — all tags and categories from the merged FSS scene, created in Stash if they don't exist.
3. **Resolution tag** (when `--resolution-tags` is set) — only the single highest applicable tag is added:
   - `4K Available` if width >= 3840
   - `Full HD Available` if width >= 1920 (and < 3840)
   - `HD Available` if width >= 1280 (and < 1920)

All tags are **additive** — existing Stash tags are never removed. Tag names are resolved against Stash aliases — e.g., if "Female Domination" is an alias for "Femdom" in your Stash instance, the existing "Femdom" tag is used instead of creating a duplicate.

## StashDB override tracking

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

## Optional Stash scraper

Pass `--scrape` to invoke Stash's built-in `scrapeSceneURL` on the first URL of each matched scene after import. This can pull additional metadata (performer images, etc.) from Stash's community scrapers. Off by default.
