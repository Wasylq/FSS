//go:build integration

package realitylovers

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func live(t *testing.T, id string) {
	t.Helper()
	for _, cfg := range sites {
		if cfg.SiteID != id {
			continue
		}
		testutil.RunLiveScrape(t, newScraper(cfg), "https://"+cfg.Domain, 3)
		return
	}
	t.Fatalf("unknown site %q", id)
}

func TestLiveRealityLovers(t *testing.T)   { live(t, "realitylovers") }
func TestLiveTSVirtualLovers(t *testing.T) { live(t, "tsvirtuallovers") }
func TestLivePlayGirlStories(t *testing.T) { live(t, "playgirlstories") }
func TestLiveWeAreCrazy(t *testing.T)      { live(t, "wearecrazy") }
