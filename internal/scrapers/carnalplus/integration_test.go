//go:build integration

package carnalplus

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

// One live smoke per parser variant the network uses.
//
//   - VariantNATS: funsizeboys.com is the representative sister site —
//     same template as the other 12 NATS domains.
//   - VariantGrid: carnalplus.com (parent portal aggregating every brand)
//     + baptistboys (sub-path).
//   - VariantWordPress: growlboys.com.

func TestLiveFunsizeBoys(t *testing.T) {
	cfg := findSite(t, "funsizeboys")
	testutil.RunLiveScrape(t, New(cfg), cfg.SiteBase+"/", 4)
}

func TestLiveCarnalPlus(t *testing.T) {
	cfg := findSite(t, "carnalplus")
	testutil.RunLiveScrape(t, New(cfg), cfg.SiteBase+"/", 4)
}

func TestLiveBaptistBoys(t *testing.T) {
	cfg := findSite(t, "baptistboys")
	testutil.RunLiveScrape(t, New(cfg), "https://carnalplus.com/baptistboys/", 4)
}

func TestLiveGrowlBoys(t *testing.T) {
	cfg := findSite(t, "growlboys")
	testutil.RunLiveScrape(t, New(cfg), cfg.SiteBase+"/", 4)
}

func findSite(t *testing.T, id string) SiteConfig {
	t.Helper()
	for _, cfg := range sites {
		if cfg.ID == id {
			return cfg
		}
	}
	t.Fatalf("no site config registered with ID %q", id)
	return SiteConfig{}
}
