# Architecture

This document describes the high-level design of FSS for contributors. For usage, see [usage.md](usage.md). For adding a scraper, see [CONTRIBUTING.md](../CONTRIBUTING.md).

## Overview

```
main.go                      ← blank-imports each scraper to trigger init()
  │
  ├── cmd/scrape.go           ← CLI entry point, orchestrates scraping
  │     │
  │     ├── scraper.ForURL()  ← registry lookup: URL → scraper
  │     │
  │     ├── scraper.ListScenes() → chan SceneResult
  │     │                          ↑ goroutine streams results
  │     │
  │     ├── collectScenes()   ← drains channel, displays progress
  │     │
  │     └── store.Save()      ← persists to JSON or SQLite
  │
  ├── cmd/stash_import.go     ← matches FSS scenes → Stash scenes
  │     ├── stash.BuildIndex()
  │     ├── stash.Match()
  │     └── stash.Client.UpdateScene()
  │
  └── cmd/stash_unmatched.go  ← queries Stash for unmatched scenes
```

## Plugin Registry

**Files:** `scraper/registry.go`, `scraper/interface.go`

Scrapers register themselves at import time — no config, no discovery:

```
init() → scraper.Register(New()) → appends to global slice
main.go blank-imports each scraper package → triggers init()
```

At runtime, `scraper.ForURL(url)` iterates registered scrapers and returns the first whose `MatchesURL()` matches. `scraper.ForID(id)` looks up by stable identifier. `scraper.All()` returns everything (used by `list-scrapers`). See [docs/library.md](library.md) for using these as a Go library.

The `StudioScraper` interface has four methods: `ID()`, `Patterns()`, `MatchesURL()`, and `ListScenes()`. All scrapers implement this directly — there's no base struct or embedding.

## Channel-Based Streaming

**Files:** `scraper/interface.go`, `cmd/scrape.go`

`ListScenes()` returns `<-chan SceneResult` immediately. A background goroutine paginates the site and sends results. The channel carries data and control signals in a single stream:

| Field | Meaning |
|-------|---------|
| `Scene` | A scraped scene — the normal case |
| `Err` | A non-fatal error (logged, scraping continues) |
| `Total` | Progress hint sent once after the first page |
| `StoppedEarly` | Incremental mode hit a known ID — stop signal |

The consumer (`collectScenes()` in `cmd/scrape.go`) checks these fields in priority order and drains until the channel closes.

**Critical invariant:** the goroutine must `defer close(out)` as its first line, and every send must be wrapped in `select` with `case <-ctx.Done()` to prevent goroutine leaks on cancellation.

## Store Abstraction

**Files:** `internal/store/interface.go`, `internal/store/flat.go`, `internal/store/sqlite.go`

The `Store` interface decouples scraping from persistence:

```go
type Store interface {
    Load(studioURL string) ([]models.Scene, error)
    Save(studioURL string, scenes []models.Scene) error
    MarkDeleted(studioURL string, ids []string, at time.Time) error
    Export(studioURL string, format string) error
    UpsertStudio(s Studio) error
    ListStudios() ([]Studio, error)
}
```

**Flat store** (default): one JSON file per studio on disk. The file _is_ the backing store — `Load()` reads it, `Save()` overwrites it atomically. CSV is an export format written alongside.

**SQLite store** (`--db`): auto-migrates schema on open via `schema_version` table. Scenes, price history, and studios live in relational tables. Performers, tags, and categories are normalized into lookup tables with junction tables (`scene_performers`, `scene_tags`, `scene_categories`) — the old JSON columns in `scenes` are kept but ignored on read. `Export()` regenerates JSON/CSV from the database. Uses `SetMaxOpenConns(1)` for serial writes.

Scrapers never know which store is active.

## Shared Scraper Packages

Two utility packages eliminate duplication for sites that share a platform:

### gammautil

**File:** `internal/scrapers/gammautil/gammautil.go`

For Gamma Entertainment sites (Pure Taboo, Taboo Heat) that use the same Algolia search backend. Provides:

- `Scraper` struct with `Run()` — handles API key extraction, Algolia pagination, and channel streaming
- `ToScene()` — converts Algolia hits to `models.Scene`
- `FetchAPIKey()` — scrapes the rotating API key from the site's HTML
- Resolution, thumbnail, and trailer helpers

Individual scrapers are ~50-line wrappers that supply a `SiteConfig` (site ID, base URL, studio name) and delegate to `gammautil.Scraper.Run()`.

### wputil

**File:** `internal/scrapers/wputil/wputil.go`

For WordPress-based sites (Tara Tainton, Mom Comes First). Provides:

- `FetchSitemap()` / `FetchAllSitemaps()` — XML sitemap parsing
- `ParseMeta()` — extracts OpenGraph tags, article metadata, JSON-LD VideoObject, shortlink post ID
- `RunWorkerPool()` — sitemap discovery + parallel page fetching with a `PageParser` callback
- `BrowserHeaders()` — WAF-bypassing headers

Individual scrapers implement a `parsePage` callback and registration; wputil handles discovery and concurrency.

## HTTP Layer

**File:** `internal/httpx/httpx.go`

All scrapers share a single HTTP transport (`MaxIdleConnsPerHost: 10`) for connection pooling across requests to the same host.

`httpx.Do()` wraps requests with:
- **Retry with backoff**: network errors, 429, 5xx are retried up to 3 times with `attempt * 2s` sleep
- **Fail-fast**: non-retryable 4xx errors return immediately as `*StatusError`
- **Centralized UA strings**: `UserAgentFirefox` and `UserAgentChrome` constants, versioned in one place

`httpx.NewClient(timeout)` creates a client using the shared transport.

## Config System

**File:** `internal/config/config.go`

YAML config at the XDG path (`~/.config/fss/config.yaml` on Linux). Loaded once at startup in `rootCmd.PersistentPreRunE`.

Resolution order: CLI flags > config file > hardcoded defaults.

No validation or dynamic reload. The Stash API key can also come from `FSS_STASH_API_KEY` env var (checked at flag parse time in `cmd/stash.go`).

## Stash Integration

**Files:** `internal/stash/client.go`, `internal/stash/match.go`, `internal/stash/merge.go`

Three-layer design:

1. **Client** (`client.go`): thin GraphQL wrapper over Stash's API. Methods for `FindScenes`, `UpdateScene`, `EnsureTag`, `EnsurePerformer`, `EnsureStudio`. Uses `httpx.Do()` for HTTP.

2. **Matcher** (`match.go`): builds a `SceneIndex` from FSS JSON files, indexed by normalized title. Matches Stash filenames against the index with two passes (primary + sanitized/noise-stripped). Returns confidence levels: Exact, Substring, Ambiguous, None. Duration filtering rejects false positives.

3. **Merger** (`merge.go`): when a title appears in multiple FSS JSONs (cross-site), combines metadata: union of URLs/performers/tags, earliest date, longest description, highest resolution.

The import command (`cmd/stash_import.go`) orchestrates: load index → query Stash → match → merge → dry-run or apply.
