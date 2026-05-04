package uptimely

import (
	"testing"
)

func TestSiteCount(t *testing.T) {
	if len(sites) != 5 {
		t.Errorf("expected 5 sites, got %d", len(sites))
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
