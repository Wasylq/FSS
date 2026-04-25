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
| `--cover` | bool | `false` | Download and set cover image from FSS thumbnail (also implicitly enabled when `cover` is in `--fields`) |
| `--cover-allow-private` | bool | `false` | Allow cover URLs that resolve to private/loopback IPs (for personal media servers) |
| `--fields` | []string | _(all)_ | Only update these fields: `title`,`details`,`date`,`urls`,`tags`,`performers`,`studio`,`cover` |
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

# Import with cover images from FSS thumbnails
fss stash import --dir ./data --cover --apply

# Filter by performer and add resolution tags
fss stash import --dir ./data --performer "Bettie Bondage" --resolution-tags --apply

# Only update tags, URLs, and date (only if earlier)
fss stash import --dir ./data --fields tags,urls,date --apply

# Only add tags and URLs, leave everything else untouched
fss stash import --dir ./data --fields tags,urls --apply

# Only process the first 50 Stash scenes (useful for testing)
fss stash import --dir ./data --top 50
```

**`--dir` vs `--json`:** By default, `--dir` loads every `*.json` file in the directory — all studios get pooled into one index. This is what you want when you've scraped a performer from multiple sites (e.g. ManyVids + Clips4Sale) and want cross-site merging. Use `--json` when you only want to import from specific files, for example a single studio.

**`--fields`:** By default, all detected changes are applied. Pass `--fields` with a comma-separated list to restrict which fields are updated. Unselected fields are left untouched in Stash, and changes to unselected fields are hidden from dry-run output. For example, `--fields date,tags,urls` will only update the release date (which already uses earliest-date logic, so it only changes when a earlier date is found), add new tags, and add new URLs — title, details, performers, studio, and cover are left as-is.

**Dry-run preview of new entities:** at the end of a dry-run, FSS prints a deduplicated, alphabetically-sorted list of every tag, performer, and studio that would be *created* in Stash on `--apply` (i.e. doesn't yet exist by name or alias). This catches the "fresh Stash silently grows 80 new tags" surprise. Existence checks are cached across scenes — the same tag is never queried twice. Example tail of a dry-run:

```
Would create on apply:
  + tag       "4K Available"
  + tag       "Female Domination"
  + performer "New Performer Name"
  + studio    "Some Studio"

Dry-run: 12 would match, 38 already up-to-date, 5 skipped, 0 ambiguous
```

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
| Cover image | First available thumbnail URL; downloaded and pushed as base64 (only when `--cover` is passed) |
| Performers | Union (deduplicated) |
| Tags | Union (deduplicated) |
| Duration | Maximum |
| Resolution | Highest available |

Format suffix stripping means that Clips4Sale scenes listed as separate formats (e.g. `"Title (FULL HD)"`, `"Title (mp4)"`, `"Title (mov)"`) are merged together, combining tags from all versions.

## Cover image fetching

Cover updates are opt-in (they're expensive and replace existing covers). Two equivalent ways to enable them:

- `--cover` — the original toggle.
- `--fields cover` — listing `cover` explicitly in the field allowlist also implicitly enables it. So `--fields tags,urls,cover --apply` works without needing `--cover` too.

The reverse (`--cover --fields title`) still skips cover, because `--fields` is a hard allowlist.

When enabled, FSS downloads each scene's `thumbnail` URL from the FSS JSON and pushes it to Stash as base64. To prevent SSRF when importing third-party JSON dumps, the URL is validated:

- Scheme must be `http` or `https`. `file://`, `gopher://`, `data:`, etc. are rejected.
- Host must not resolve to a private (RFC1918), loopback, link-local, or unspecified address. This stops a malicious JSON from coercing FSS to fetch from `localhost`, cloud metadata services (`169.254.169.254`), or your internal network.
- Response body is capped at 10 MiB.

If you legitimately host cover images on your own LAN (e.g. a media server at `192.168.1.50`), pass `--cover-allow-private` to bypass the IP check. The scheme and size cap still apply.

> Note: your local Stash URL (`--url http://localhost:9999`) is *not* affected by these checks — they apply only to the cover image fetch, not to the GraphQL endpoint.

## Tags

Every matched scene receives:

1. **Import marker tag** — `fss_import` by default (configurable via `--tag`). Applied to all imported scenes.
2. **FSS scene tags** — all tags and categories from the merged FSS scene, created in Stash if they don't exist.
3. **Resolution tag** (when `--resolution-tags` is set) — only the single highest applicable tag is added:
   - `4K Available` if width >= 3840
   - `Full HD Available` if width >= 1920 (and < 3840)
   - `HD Available` if width >= 1280 (and < 1920)

All tags are **additive** — existing Stash tags are never removed. Tag names are resolved against Stash aliases — e.g., if "Female Domination" is an alias for "Femdom" in your Stash instance, the existing "Femdom" tag is used instead of creating a duplicate.

## Failure handling in apply mode

When `--apply` runs, per-scene operations can fail in two distinct ways:

- **Update failed:** the underlying `sceneUpdate` GraphQL mutation errored — nothing was written for that scene. Counted as `failed` in the summary.
- **Partial:** the scene was updated, but one or more `EnsureTag` / `EnsurePerformer` / `EnsureStudio` calls or the cover image download failed mid-way. The scene has the fields that did succeed; missing pieces are reported. Counted as `partial`.

After the loop a grouped failure summary is written to **stderr** (so it stays out of the way of pipes). Each scene appears once with a list of which operations failed and why:

```
Failures (3 operations across 2 scenes):
  scene 42 (mom-fucks-daughters-ex.mp4):
    - tag "Female Domination": stash api: timeout reading from upstream
    - performer "Bettie Bondage": alias collision
  scene 87 (joi-countdown.mp4):
    - cover "https://cdn.example/cover.jpg": rejecting cover URL: host "cdn.example" is a private/loopback IP
```

The final stats line includes both new counters:

```
Done: 102 matched, 100 updated, 2 partial, 0 failed, 38 already up-to-date, 5 skipped, 0 ambiguous
```

Re-running the import after fixing the underlying issue (e.g. transient Stash hiccup) will reach those scenes again because `buildChanges` still detects the missing fields.

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
