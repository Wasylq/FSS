package teamskeet

import (
	"testing"

	"github.com/Wasylq/FSS/scraper"
)

func TestSiteCount(t *testing.T) {
	if len(sites) != 47 {
		t.Errorf("expected 47 sites, got %d", len(sites))
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

func TestSayUncleScenePath(t *testing.T) {
	var found bool
	for _, cfg := range sites {
		if cfg.SiteID == "sayuncle" {
			found = true
			if cfg.Index != "sau_network" {
				t.Errorf("SayUncle index = %q, want sau_network", cfg.Index)
			}
			if cfg.ScenePath != "/movies/" {
				t.Errorf("SayUncle ScenePath = %q, want /movies/", cfg.ScenePath)
			}
		}
	}
	if !found {
		t.Error("sayuncle entry not found in sites")
	}
}
