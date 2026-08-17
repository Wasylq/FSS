//go:build integration

package shinybound

import (
	"testing"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
)

func siteByID(t *testing.T, id string) *Scraper {
	t.Helper()
	for _, cfg := range sites {
		if cfg.SiteID == id {
			return New(cfg)
		}
	}
	t.Fatalf("site %q not registered", id)
	return nil
}

func TestLiveShinyBound(t *testing.T) {
	testutil.RunLiveScrape(t, siteByID(t, "shinybound"), "https://shinybound.com/", 2)
}

func TestLiveShinysBoundSluts(t *testing.T) {
	testutil.RunLiveScrape(t, siteByID(t, "shinysboundsluts"), "https://shinysboundsluts.com/", 2)
}

// A single scene URL skips the listing walk.
func TestLiveShinyBoundScene(t *testing.T) {
	testutil.RunLiveScrape(t, siteByID(t, "shinybound"),
		"https://shinybound.com/updates/silky-spandex-and-heavy-leather", 1)
}
