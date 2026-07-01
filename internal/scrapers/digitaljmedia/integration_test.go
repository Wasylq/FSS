//go:build integration

package digitaljmedia

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/digitaljmediautil"
	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

// scraperFor returns a live scraper for the given network site id.
func scraperFor(t *testing.T, id string) *digitaljmediautil.Scraper {
	t.Helper()
	for _, cfg := range digitaljmediautil.Configs() {
		if cfg.SiteID == id {
			return digitaljmediautil.New(cfg)
		}
	}
	t.Fatalf("no config for %s", id)
	return nil
}

func TestLiveFellatioJapan(t *testing.T) {
	testutil.RunLiveScrape(t, scraperFor(t, "fellatiojapan"), "https://fellatiojapan.com/en/samples", 3)
}

func TestLiveCospuri(t *testing.T) {
	testutil.RunLiveScrape(t, scraperFor(t, "cospuri"), "https://cospuri.com/samples", 3)
}

func TestLiveCumBuffet(t *testing.T) {
	testutil.RunLiveScrape(t, scraperFor(t, "cumbuffet"), "https://cumbuffet.com/samples", 3)
}
