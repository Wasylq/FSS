package gamma

import (
	"testing"

	"github.com/Wasylq/FSS/scraper"
)

func TestSiteCount(t *testing.T) {
	if len(sites) != 14 {
		t.Errorf("expected 14 sites, got %d", len(sites))
	}
}

func TestScraperInterface(t *testing.T) {
	for _, cfg := range sites {
		_ = cfg
		var _ scraper.StudioScraper = &siteScraper{}
	}
}

func TestUniqueSiteIDs(t *testing.T) {
	seen := map[string]bool{}
	for _, cfg := range sites {
		if seen[cfg.SiteID] {
			t.Errorf("duplicate SiteID: %s", cfg.SiteID)
		}
		seen[cfg.SiteID] = true
	}
}
