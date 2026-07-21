//go:build integration

package teendreams

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

func TestLiveTeenDreams(t *testing.T) { live(t, "teendreams") }
func TestLiveLesArchive(t *testing.T) { live(t, "lesarchive") }
