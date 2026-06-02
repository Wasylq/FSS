//go:build integration

package innofsin

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveMyDeepDarkSecret(t *testing.T) {
	testutil.RunLiveScrape(t, newScraper(sites[0]), "https://mydeepdarksecret.com", 3)
}

func TestLiveRichardMannsWorld(t *testing.T) {
	testutil.RunLiveScrape(t, newScraper(sites[1]), "https://richardmannsworld.com", 3)
}

func TestLiveBBCTitans(t *testing.T) {
	testutil.RunLiveScrape(t, newScraper(sites[2]), "https://bbctitans.com", 3)
}

func TestLiveRichardMannEvents(t *testing.T) {
	testutil.RunLiveScrape(t, newScraper(sites[3]), "https://richardmannevents.com", 3)
}
