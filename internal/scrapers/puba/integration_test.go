//go:build integration

package puba

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

// All 14 scrapers share one JSON API + one parser; two live smokes (the
// parent network + a single per-pornstar group) validate the full
// wiring without hammering puba.com.
func TestLivePubaNetwork(t *testing.T) {
	cfg := findSite(t, "puba")
	testutil.RunLiveScrape(t, New(cfg), "https://www.puba.com/pornstarnetwork/index.php?section=538", 4)
}

func TestLivePubaSamanthaSaint(t *testing.T) {
	cfg := findSite(t, "pubasamanthasaint")
	testutil.RunLiveScrape(t, New(cfg), "https://www.puba.com/pornstarnetwork/index.php?section=538&group=46", 4)
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
