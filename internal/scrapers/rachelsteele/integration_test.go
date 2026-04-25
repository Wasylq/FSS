//go:build integration

package rachelsteele

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

// liveStudioURL — site root; the scraper handles the listing.
const liveStudioURL = "https://rachel-steele.com"

func TestLiveRachelSteele(t *testing.T) {
	testutil.SkipIfPlaceholder(t, liveStudioURL)
	testutil.RunLiveScrape(t, New(), liveStudioURL, 2)
}
