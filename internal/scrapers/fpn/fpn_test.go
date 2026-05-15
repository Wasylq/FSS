package fpn

import (
	"testing"

	"github.com/Wasylq/FSS/scraper"
)

func TestSiteCount(t *testing.T) {
	if len(sites) != 22 {
		t.Errorf("expected 22 sites, got %d", len(sites))
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

func TestUniqueDomains(t *testing.T) {
	seen := map[string]bool{}
	for _, cfg := range sites {
		if seen[cfg.Domain] {
			t.Errorf("duplicate Domain: %s", cfg.Domain)
		}
		seen[cfg.Domain] = true
	}
}

func TestMatchesURL(t *testing.T) {
	s := newSiteScraper(sites[4]) // analized
	if !s.MatchesURL("https://analized.com/porn-categories/movies/") {
		t.Error("should match analized.com")
	}
	if !s.MatchesURL("https://www.analized.com/models/Naomi.html") {
		t.Error("should match www.analized.com")
	}
	if s.MatchesURL("https://baddaddypov.com/") {
		t.Error("should not match different domain")
	}
}
