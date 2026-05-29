//go:build integration

package kocompany

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

// One live smoke per filter form: label (KO BEAST) and maker (KO EAST).
// All 14 sub-labels share the same parser + pagination shape so the two
// representative smokes validate the entire wiring without hammering
// the upstream EC-CUBE.
func TestLiveKOBeast(t *testing.T) {
	cfg := findSite(t, "kobeast")
	testutil.RunLiveScrape(t, New(cfg), "https://ko-video.com/products/list.php?label=3", 4)
}

func TestLiveKOEast(t *testing.T) {
	cfg := findSite(t, "koeast")
	testutil.RunLiveScrape(t, New(cfg), "https://ko-video.com/products/list.php?maker=10", 4)
}

func TestLiveKODopyuNonke(t *testing.T) {
	// CJK-named label, exercises label=56 plus the ko-tube alt-URL match.
	cfg := findSite(t, "kodopyunonke")
	testutil.RunLiveScrape(t, New(cfg), "https://ko-video.com/products/list.php?label=56", 4)
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
