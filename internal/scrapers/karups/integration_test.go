//go:build integration

package karups

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLive_KarupsOW(t *testing.T) {
	testutil.RunLiveScrape(t, newScraper(sites[0]), "https://www.karupsow.com/videos/", 5)
}

func TestLive_KarupsPC(t *testing.T) {
	testutil.RunLiveScrape(t, newScraper(sites[1]), "https://www.karupspc.com/videos/", 5)
}

func TestLive_KarupsHA(t *testing.T) {
	testutil.RunLiveScrape(t, newScraper(sites[2]), "https://www.karupsha.com/videos/", 5)
}
