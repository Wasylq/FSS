# Using FSS as a Go Library

FSS can be imported as a Go module. The scraper registry, scene types, and streaming interface are all exported — you get the same engine the CLI uses, without the CLI.

## Install

```bash
go get github.com/Wasylq/FSS@latest # Or use tag for stable release
```

## Quick Start

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/Wasylq/FSS/scraper"

	// Blank-import each scraper you want available.
	// Each import registers itself via init().
	_ "github.com/Wasylq/FSS/internal/scrapers/manyvids"
	_ "github.com/Wasylq/FSS/internal/scrapers/clips4sale"
)

func main() {
	ctx := context.Background()

	// Look up by URL (returns the first scraper whose MatchesURL matches)...
	s, err := scraper.ForURL("https://www.manyvids.com/Profile/590705/bettie-bondage/Store/Videos")
	if err != nil {
		log.Fatal(err)
	}

	// ...or by scraper ID directly.
	s, err = scraper.ForID("manyvids")
	if err != nil {
		log.Fatal(err)
	}

	ch, err := s.ListScenes(ctx, "https://www.manyvids.com/Profile/590705/bettie-bondage/Store/Videos", scraper.ListOpts{})
	if err != nil {
		log.Fatal(err)
	}

	for r := range ch {
		if r.Total > 0 {
			fmt.Printf("Estimated total: %d scenes\n", r.Total)
			continue
		}
		if r.StoppedEarly {
			fmt.Println("Stopped early (hit known ID)")
			continue
		}
		if r.Err != nil {
			log.Printf("error: %v", r.Err)
			continue
		}

		fmt.Printf("%-40s %s\n", r.Scene.Title, r.Scene.URL)
	}
}
```

## Registering Scrapers

Scrapers use Go's `init()` mechanism — a blank import is all it takes:

```go
// Register a single scraper:
_ "github.com/Wasylq/FSS/internal/scrapers/manyvids"

// Or register all of them (same imports as main.go):
_ "github.com/Wasylq/FSS/internal/scrapers/brazzers"
_ "github.com/Wasylq/FSS/internal/scrapers/clips4sale"
_ "github.com/Wasylq/FSS/internal/scrapers/manyvids"
// ... etc.
```

Only import what you need — each import adds to the global registry and slightly increases binary size.

## Registry API

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

`ListScenes` returns a channel of `SceneResult`. Each result is one of four things — check in this order:

```go
for r := range ch {
    switch {
    case r.Total > 0:
        // Progress hint (sent once). Use for display, then skip.
    case r.StoppedEarly:
        // Incremental mode hit a known ID. No more scenes coming.
    case r.Err != nil:
        // Non-fatal error. Log and continue.
    default:
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
