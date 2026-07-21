//go:build integration

package jakecruise

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

func TestLiveCocksureMen(t *testing.T) { live(t, "cocksuremen") }
func TestLiveJakeCruise(t *testing.T)  { live(t, "jakecruise") }
func TestLiveStraightGuysForGayEyes(t *testing.T) {
	live(t, "straightguysforgayeyes")
}
func TestLiveHotDadsHotLads(t *testing.T) { live(t, "hotdadshotlads") }
