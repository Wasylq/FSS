//go:build integration

package bronetwork

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/bronetworkutil"
	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

// site looks a config up by ID rather than by position in `sites`. The slice
// order is load-bearing for URL routing (see the comment on `sites`), so
// indexing into it silently pointed these tests at the wrong scraper the first
// time that order changed.
func site(t *testing.T, id string) bronetworkutil.SiteConfig {
	t.Helper()
	for _, cfg := range sites {
		if cfg.ID == id {
			return cfg
		}
	}
	t.Fatalf("no site config with ID %q", id)
	return bronetworkutil.SiteConfig{}
}

func TestLiveMenAtPlay(t *testing.T) {
	testutil.RunLiveScrape(t, bronetworkutil.New(site(t, "menatplay")), "https://menatplay.com/", 3)
}

func TestLiveBroNetwork(t *testing.T) {
	testutil.RunLiveScrape(t, bronetworkutil.New(site(t, "thebronetwork")), "https://thebronetwork.com/", 3)
}

func TestLiveMasqulin(t *testing.T) {
	testutil.RunLiveScrape(t, bronetworkutil.New(site(t, "masqulin")), "https://masqulin.com/", 3)
}
