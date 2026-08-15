//go:build integration

package pantheon

import (
	"testing"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
)

// siteByID looks the config up by name rather than by table index, which moves
// whenever a site is added.
func siteByID(t *testing.T, id string) *Scraper {
	t.Helper()
	for _, cfg := range sites {
		if cfg.SiteID == id {
			return New(cfg)
		}
	}
	t.Fatalf("site %q not registered", id)
	return nil
}

func TestLiveHotOlderMale(t *testing.T) {
	testutil.RunLiveScrape(t, siteByID(t, "hotoldermale"), "https://www.hotoldermale.com/", 2)
}

// A category and a performer page render the same cards, so both walk the same
// way over a different path.
func TestLiveHotOlderMaleCategory(t *testing.T) {
	testutil.RunLiveScrape(t, siteByID(t, "hotoldermale"), "https://www.hotoldermale.com/scenes/category/3-bears", 2)
}

func TestLiveHotOlderMaleProfile(t *testing.T) {
	testutil.RunLiveScrape(t, siteByID(t, "hotoldermale"), "https://www.hotoldermale.com/profile/609-mack-austin", 2)
}

// A single scene URL skips the walk entirely.
func TestLiveHotOlderMaleScene(t *testing.T) {
	testutil.RunLiveScrape(t, siteByID(t, "hotoldermale"), "https://www.hotoldermale.com/scene/910-daddy-mack-austin-fucks-muscle-stud-davin", 1)
}

// The two sibling tours run the same CMS; a live check on each is what pins
// that they really are card-for-card identical.
func TestLiveBlackBoyAddictionz(t *testing.T) {
	testutil.RunLiveScrape(t, siteByID(t, "blackboyaddictionz"), "https://www.blackboyaddictionz.com/", 2)
}

func TestLiveMonsterCub(t *testing.T) {
	testutil.RunLiveScrape(t, siteByID(t, "monstercub"), "https://www.monstercub.com/", 2)
}
