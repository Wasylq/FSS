package fpn

import (
	"testing"

	"github.com/Wasylq/FSS/scraper"
)

func TestSiteCount(t *testing.T) {
	if len(sites) != 23 {
		t.Errorf("expected 23 sites, got %d", len(sites))
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

// TestArchAngelVideoSite pins the ArchAngel Video entry. ArchAngel is part of
// Full Porn Network and runs the same Elevated X TourFoundation skin as
// analized.com, so it belongs here as a config row rather than in a scraper of
// its own.
func TestArchAngelVideoSite(t *testing.T) {
	var found bool
	for _, c := range sites {
		if c.SiteID == "archangelvideo" {
			found = true
			if c.Domain != "archangelvideo.com" {
				t.Errorf("Domain = %q", c.Domain)
			}
			if c.SiteBase != "https://archangelvideo.com" {
				t.Errorf("SiteBase = %q", c.SiteBase)
			}
			if c.StudioName != "ArchAngel Video" {
				t.Errorf("StudioName = %q", c.StudioName)
			}
		}
	}
	if !found {
		t.Fatal("archangelvideo is not configured")
	}

	s := newSiteScraper(sites[0])
	_ = s
	for _, c := range sites {
		sc := newSiteScraper(c)
		if !sc.MatchesURL("https://" + c.Domain + "/") {
			t.Errorf("%s does not match its own domain", c.SiteID)
		}
	}
}
