//go:build integration

package feetondemand

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

// One live smoke per site is overkill — all 4 share the same AJAX
// template + parser. One smoke against the biggest catalogue (Goddess
// Foot Domination, ~1280 scenes) validates the shared wiring.
func TestLiveGoddessFootDomination(t *testing.T) {
	cfg := findSite(t, "goddessfootdomination")
	testutil.RunLiveScrape(t, New(cfg), cfg.BaseURL+"/", 4)
}

func findSite(t *testing.T, id string) SiteConfig {
	t.Helper()
	for _, cfg := range sites {
		if cfg.ID == id {
			return cfg
		}
	}
	t.Fatalf("no site config with ID %q", id)
	return SiteConfig{}
}
