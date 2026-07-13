//go:build integration

package marsmedia

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/natscmsutil"
	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

// All 12 sites share the same NATS CMS API; one live smoke per CMS area
// validates the full wiring. Pick the largest catalogue (bearfilms,
// ~1473 sets) and one smaller site to verify per-site UUIDs work.
func TestLiveBearFilms(t *testing.T) {
	cfg := findSite(t, "bearfilms")
	testutil.RunLiveScrape(t, natscmsutil.New(cfg), cfg.SiteBase+"/", 4)
}

func TestLiveHardBritLads(t *testing.T) {
	cfg := findSite(t, "hardbritlads")
	testutil.RunLiveScrape(t, natscmsutil.New(cfg), cfg.SiteBase+"/", 4)
}

func findSite(t *testing.T, id string) natscmsutil.SiteConfig {
	t.Helper()
	for _, cfg := range sites {
		if cfg.ID == id {
			return withDefaults(cfg)
		}
	}
	t.Fatalf("no site config with ID %q", id)
	return natscmsutil.SiteConfig{}
}
