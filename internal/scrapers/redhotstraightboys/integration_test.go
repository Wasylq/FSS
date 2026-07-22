//go:build integration

package redhotstraightboys

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
		testutil.RunLiveScrape(t, newScraper(cfg), "https://www."+cfg.Domain, 3)
		return
	}
	t.Fatalf("unknown site %q", id)
}

func TestLiveRedHotStraightBoys(t *testing.T)   { live(t, "redhotstraightboys") }
func TestLiveSpankingStraightBoys(t *testing.T) { live(t, "spankingstraightboys") }
