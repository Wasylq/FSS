//go:build integration

package zone8

import (
	"testing"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
)

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

func TestLiveEnglishLads(t *testing.T) {
	testutil.RunLiveScrape(t, siteByID(t, "englishlads"), "https://www.englishlads.com/", 2)
}

// The sibling tour shares the sitemap-driven walk but has its own page
// template, so it needs its own live check.
func TestLiveFitYoungMen(t *testing.T) {
	testutil.RunLiveScrape(t, siteByID(t, "fityoungmen"), "https://www.fityoungmen.com/", 2)
}

// A single shoot URL skips the sitemap entirely.
func TestLiveEnglishLadsShoot(t *testing.T) {
	testutil.RunLiveScrape(t, siteByID(t, "englishlads"),
		"https://www.englishlads.com/video-2025-05-16-02979-N-young-lean-hiker-wanks-his-big-uncut-cock-in-the-ecuadorian-mountains-cums-buckets-loads", 1)
}
