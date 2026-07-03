//go:build integration

package underwatershow

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveUnderwaterShow(t *testing.T) {
	testutil.RunLiveScrape(t, newScraper(sites[0]), "https://underwatershow.com", 3)
}

func TestLiveAnalCoach(t *testing.T) {
	testutil.RunLiveScrape(t, newScraper(sites[1]), "https://anal-coach.com", 3)
}
