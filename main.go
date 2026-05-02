// FSS (FullStudioScraper) scrapes scene metadata from studio URLs.
//
// # CLI
//
// Install the binary and run:
//
//	fss scrape <studio-url>
//	fss list-scrapers
//	fss stash import --dir ./data
//
// See https://github.com/Wasylq/FSS for full CLI documentation.
//
// # Library
//
// FSS can be imported as a Go module. The public API lives in two packages:
//
//   - [github.com/Wasylq/FSS/scraper] — scraper registry and streaming interface
//   - [github.com/Wasylq/FSS/models] — Scene and PriceSnapshot types
//
// Blank-import the scraper packages you need to register them, then look up
// by URL or ID:
//
//	import (
//	    "github.com/Wasylq/FSS/scraper"
//	    _ "github.com/Wasylq/FSS/internal/scrapers/manyvids"
//	)
//
//	s, err := scraper.ForURL("https://www.manyvids.com/Profile/590705/bettie-bondage/Store/Videos")
//	ch, err := s.ListScenes(ctx, url, scraper.ListOpts{})
//	for r := range ch {
//	    fmt.Println(r.Scene.Title)
//	}
//
// To register every bundled scraper at once, blank-import
// [github.com/Wasylq/FSS/internal/scrapers/all] instead. That is what the fss
// binary itself does.
//
// See [docs/library.md] in the repository for the full guide.
package main

import (
	"github.com/Wasylq/FSS/cmd"
	_ "github.com/Wasylq/FSS/internal/scrapers/all"
)

// Set by -ldflags at release build time.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	cmd.SetVersion(version, commit, date)
	cmd.Execute()
}
