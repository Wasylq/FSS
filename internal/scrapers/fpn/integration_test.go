//go:build integration

package fpn

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/fpnutil"
	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

// findSite looks a site up by ID rather than slice index — inserting a config
// row must not silently repoint these tests at a different site.
func findSite(t *testing.T, id string) fpnutil.SiteConfig {
	t.Helper()
	for _, c := range sites {
		if c.SiteID == id {
			return c
		}
	}
	t.Fatalf("site not found: %s", id)
	return fpnutil.SiteConfig{}
}

func live(t *testing.T, id, url string) {
	t.Helper()
	testutil.RunLiveScrape(t, newSiteScraper(findSite(t, id)), url, 2)
}

func TestLiveAnalized(t *testing.T) {
	live(t, "analized", "https://analized.com/")
}

func TestLiveBadDaddyPOV(t *testing.T) {
	live(t, "baddaddypov", "https://baddaddypov.com/")
}

func TestLiveFullPornNetwork(t *testing.T) {
	live(t, "fullpornnetwork", "https://fullpornnetwork.com/")
}

func TestLiveArchAngelVideo(t *testing.T) {
	live(t, "archangelvideo", "https://archangelvideo.com/")
}
