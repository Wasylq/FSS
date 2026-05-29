//go:build integration

package wtfpass

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

// One live smoke for the parent (covers the whole network in one pass via
// per-card Series labels), one for a sister domain (own-catalogue path),
// and one for the alternate-domain config (hardfucktales.com paired with
// hardfuckgirls.com).

func TestLiveWTFPass(t *testing.T) {
	cfg := findSite(t, "wtfpass")
	testutil.RunLiveScrape(t, New(cfg), cfg.SiteBase+"/", 4)
}

func TestLiveTheArtPorn(t *testing.T) {
	cfg := findSite(t, "theartporn")
	testutil.RunLiveScrape(t, New(cfg), cfg.SiteBase+"/", 4)
}

func TestLiveHardFuckTales(t *testing.T) {
	cfg := findSite(t, "hardfucktales")
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
