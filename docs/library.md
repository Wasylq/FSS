# Using FSS as a Go Library

FSS can be imported as a Go module. The scraper engine, matching, merging, Stash integration, and NFO generation are all available to external code.

## Install

```bash
go get github.com/Wasylq/FSS@latest # Or use tag for stable release
```

## Public Packages

| Package | Import path | Purpose |
|---------|------------|---------|
| `scrapers/all` | `github.com/Wasylq/FSS/scrapers/all` | Blank-import to register all scrapers |
| `scraper` | `github.com/Wasylq/FSS/scraper` | Registry API, `StudioScraper` interface, `SceneResult` channel protocol |
| `models` | `github.com/Wasylq/FSS/models` | `Scene`, `PriceSnapshot` — the core data model |
| `match` | `github.com/Wasylq/FSS/match` | Filename→title matching, cross-site merging, JSON loading |
| `stash` | `github.com/Wasylq/FSS/stash` | GraphQL client for Stash |
| `nfo` | `github.com/Wasylq/FSS/nfo` | Kodi-style NFO XML generation |
| `identify` | `github.com/Wasylq/FSS/identify` | Video directory scan + match + NFO write |

**Registering scrapers:** The individual scraper implementations live under `internal/scrapers/`, but a public aggregator package re-exports them all:

```go
import _ "github.com/Wasylq/FSS/scrapers/all"  // registers all 102 scrapers
```

This is all you need to populate the registry for scraping from external code.

## Quick Start — Scraping

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/Wasylq/FSS/scraper"
	_ "github.com/Wasylq/FSS/scrapers/all"
)

func main() {
	ctx := context.Background()

	s, err := scraper.ForURL("https://www.manyvids.com/Profile/590705/bettie-bondage/Store/Videos")
	if err != nil {
		log.Fatal(err)
	}

	ch, err := s.ListScenes(ctx, "https://www.manyvids.com/Profile/590705/bettie-bondage/Store/Videos", scraper.ListOpts{})
	if err != nil {
		log.Fatal(err)
	}

	for r := range ch {
		switch r.Kind {
		case scraper.KindScene:
			fmt.Printf("%-40s %s\n", r.Scene.Title, r.Scene.URL)
		case scraper.KindError:
			log.Printf("error: %v", r.Err)
		}
	}
}
```

## Quick Start — Matching & Merging

Load FSS JSON output files and match filenames against them (no scraper registration needed).

```go
package main

import (
	"fmt"
	"time"

	"github.com/Wasylq/FSS/match"
)

func main() {
	// Load scenes from FSS JSON files (produced by `fss scrape`).
	scenes, err := match.LoadJSONDir("./data")
	if err != nil {
		panic(err)
	}

	// Build a title index.
	idx := match.BuildIndex(scenes)

	// Match a filename (duration in seconds, 0 = unknown).
	result := idx.Match("Fostering the Bully.mp4", 605.0)

	switch result.Confidence {
	case match.MatchExact:
		fmt.Println("Exact match:", result.Scenes[0].Title)
	case match.MatchSubstring:
		fmt.Println("Substring match:", result.Scenes[0].Title)
	case match.MatchAmbiguous:
		fmt.Printf("Ambiguous: %d candidates\n", result.Candidates)
	case match.MatchNone:
		fmt.Println("No match")
	}

	// Merge cross-site scenes into a single metadata record.
	if result.Confidence == match.MatchExact || result.Confidence == match.MatchSubstring {
		merged := match.MergeScenes(result.Scenes, time.Time{})
		fmt.Println(merged.Title, merged.URLs, merged.Performers)
	}
}
```

## Selective Scraper Registration

If you only need a few scrapers (to reduce binary size), you can blank-import individual packages from within a fork or custom build inside this repo:

```go
// Only works within the FSS module (forks, custom builds).
_ "github.com/Wasylq/FSS/internal/scrapers/manyvids"
_ "github.com/Wasylq/FSS/internal/scrapers/clips4sale"
```

From external modules, use `_ "github.com/Wasylq/FSS/scrapers/all"` to register all scrapers at once.

## Registry API

After importing `scrapers/all`, the registry is populated and these functions work:

```go
// Find a scraper by URL.
s, err := scraper.ForURL("https://www.manyvids.com/...")

// Find a scraper by its stable ID (e.g. "manyvids", "clips4sale").
s, err := scraper.ForID("clips4sale")

// List all registered scrapers.
for _, s := range scraper.All() {
    fmt.Printf("%-20s %s\n", s.ID(), s.Patterns())
}
```

Use `fss list-scrapers` (or iterate `scraper.All()`) to see all available IDs and URL patterns.

## Controlling Scrape Behaviour

`ListOpts` configures how the scraper paginates:

```go
opts := scraper.ListOpts{
    // Number of concurrent detail-page workers (for scrapers that fetch
    // detail pages). Zero uses the scraper's default (usually 4).
    Workers: 2,

    // Delay between page fetches. Useful for rate-limiting.
    Delay: 500 * time.Millisecond,

    // Incremental mode: stop as soon as any of these IDs are encountered.
    // Scrapers that sort newest-first will stop at the first known scene,
    // skipping older pages that are already in your store.
    KnownIDs: map[string]bool{
        "existing-scene-id": true,
    },
}
```

## Reading Results

`ListScenes` returns a channel of `SceneResult`. Each result carries a `Kind` field — switch on it:

```go
for r := range ch {
    switch r.Kind {
    case scraper.KindTotal:
        // Progress hint (sent once). Use for display, then skip.
    case scraper.KindStoppedEarly:
        // Incremental mode hit a known ID. No more scenes coming.
    case scraper.KindError:
        // Non-fatal error (r.Err). Log and continue.
    case scraper.KindScene:
        // r.Scene is a valid models.Scene.
    }
}
```

The channel is always closed when the scraper finishes (or is cancelled via context).

## The Scene Model

`models.Scene` has everything a scraper can extract. Fields vary by site — only `ID`, `SiteID`, `Title`, `URL`, and `ScrapedAt` are guaranteed.

| Group | Fields |
|-------|--------|
| Identity | `ID`, `SiteID`, `StudioURL` |
| Core | `Title`, `URL`, `Date`, `Description` |
| Media | `Thumbnail`, `Preview` |
| People | `Performers`, `Director`, `Studio` |
| Classification | `Tags`, `Categories` |
| Series | `Series`, `SeriesPart` |
| Technical | `Duration` (seconds), `Resolution`, `Width`, `Height`, `Format` |
| Engagement | `Views`, `Likes`, `Comments` |
| Pricing | `PriceHistory`, `LowestPrice`, `LowestPriceDate` |
| Housekeeping | `ScrapedAt`, `DeletedAt` |

Scenes serialize cleanly to JSON (all fields have `json` tags with `omitempty` where appropriate).

## Cancellation

Pass a cancellable context to stop scraping early:

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

ch, _ := s.ListScenes(ctx, url, scraper.ListOpts{})
for r := range ch {
    // ...
    if haveEnough {
        cancel() // scraper stops, channel drains and closes
    }
}
```

All scrapers respect `ctx.Done()` on every page fetch and channel send — cancellation is immediate with no goroutine leaks.

## Matching & Merging (`match`)

The `match` package provides filename-to-title matching and cross-site scene merging — the same engine used by `fss stash import` and `fss identify`.

```go
import (
    "github.com/Wasylq/FSS/match"
    _ "github.com/Wasylq/FSS/internal/scrapers/manyvids"
)

// Load scenes from FSS JSON files.
scenes, err := match.LoadJSONFiles([]string{"manyvids.json", "clips4sale.json"})
// or: scenes, err := match.LoadJSONDir("./data")

// Build a title index.
idx := match.BuildIndex(scenes)

// Match a filename (duration in seconds, 0 = unknown).
result := idx.Match("Fostering the Bully.mp4", 605.0)

switch result.Confidence {
case match.MatchExact:
    fmt.Println("Exact match:", result.Scenes[0].Title)
case match.MatchSubstring:
    fmt.Println("Substring match:", result.Scenes[0].Title)
case match.MatchAmbiguous:
    fmt.Printf("Ambiguous: %d candidates\n", result.Candidates)
case match.MatchNone:
    fmt.Println("No match")
}

// Merge cross-site scenes into a single metadata record.
merged := match.MergeScenes(result.Scenes, time.Time{})
fmt.Println(merged.Title, merged.URLs, merged.Performers)
```

**Key types:** `SceneIndex`, `MatchResult`, `MatchConfidence`, `MergedScene`.

## NFO Generation (`nfo`)

The `nfo` package generates Kodi-style `.nfo` XML files from merged scene metadata.

```go
import "github.com/Wasylq/FSS/nfo"

mov := nfo.FromMergedScene(merged) // merged is a match.MergedScene
data, err := nfo.Marshal(mov)
os.WriteFile("scene.nfo", data, 0o644)
```

**Key types:** `Movie`, `Thumb`, `Actor`.

## Identify (`identify`)

The `identify` package scans a directory of video files, matches them against an FSS scene index, and optionally writes `.nfo` sidecar files.

```go
import "github.com/Wasylq/FSS/identify"

videos, _ := identify.FindVideos("/path/to/videos")
results := identify.Run(videos, idx, identify.Options{
    Apply: true,  // write .nfo files (false = dry-run)
    Force: false, // don't overwrite existing .nfo
})
stats := identify.Summarize(results)
fmt.Printf("%d matched, %d unmatched\n", stats.Matched, stats.Unmatched)
```

**Key types:** `Result`, `Options`, `Stats`.

## Stash Client (`stash`)

The `stash` package provides a GraphQL client for interacting with a [Stash](https://stashapp.cc/) instance.

```go
import "github.com/Wasylq/FSS/stash"

client := stash.NewClient("http://localhost:9999", "optional-api-key")

// Ping to verify connectivity.
err := client.Ping(ctx)

// Query scenes.
scenes, total, err := client.FindScenes(ctx, stash.FindScenesFilter{
    PerformerName: "Bettie Bondage",
}, 1, 25)

// Update a scene.
title := "New Title"
err = client.UpdateScene(ctx, stash.SceneUpdateInput{
    ID:    "42",
    Title: &title,
})

// Ensure entities exist (create if missing).
tagID, _ := client.EnsureTag(ctx, "fss_import")
perfID, _ := client.EnsurePerformer(ctx, "Bettie Bondage")
studioID, _ := client.EnsureStudio(ctx, "Bettie Bondage")
```

**Key types:** `Client`, `StashScene`, `FindScenesFilter`, `SceneUpdateInput`.
