//go:build integration

package feetondemand

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

// One live smoke per site is overkill — all 5 share the same AJAX
// template + parser. Two representative smokes (the biggest catalogue
// — Goddess Foot Domination with ~1280 scenes; and a smaller one) are
// enough to validate the wiring.
func TestLiveGoddessFootDomination(t *testing.T) {
	cfg := findSite(t, "goddessfootdomination")
	testutil.RunLiveScrape(t, New(cfg), cfg.BaseURL+"/", 4)
}

func TestLiveJerkToMyFeet(t *testing.T) {
	cfg := findSite(t, "jerktomyfeet")
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
