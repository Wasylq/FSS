//go:build integration

package missax

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

// liveStudioURL — site root; MissaX scraper handles the listing.
const liveStudioURL = "https://www.missax.com"

func TestLiveMissaX(t *testing.T) {
	testutil.SkipIfPlaceholder(t, liveStudioURL)
	testutil.RunLiveScrape(t, New(), liveStudioURL, 2)
}
