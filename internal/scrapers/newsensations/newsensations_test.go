package newsensations

import (
	"testing"

	"github.com/Wasylq/FSS/scraper"
)

func TestSiteCount(t *testing.T) {
	if len(sites) != 8 {
		t.Errorf("expected 8 sites, got %d", len(sites))
	}
}

func TestScraperInterface(t *testing.T) {
	for _, cfg := range sites {
		s, err := scraper.ForID(cfg.SiteID)
		if err != nil {
			t.Errorf("ForID(%q): %v", cfg.SiteID, err)
			continue
		}
		var _ scraper.StudioScraper = s
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

func TestUniqueDomains(t *testing.T) {
	seen := map[string]bool{}
	for _, cfg := range sites {
		if seen[cfg.Domain] {
			t.Errorf("duplicate domain: %s", cfg.Domain)
		}
		seen[cfg.Domain] = true
	}
}
