//go:build integration

package brazzers

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

// liveStudioURL — Reagan Foxx profile (long-running performer).
// If gone, swap for any other active brazzers performer.
const liveStudioURL = "https://www.brazzers.com/pornstar/2719/reagan-foxx"

func TestLiveBrazzers(t *testing.T) {
	testutil.SkipIfPlaceholder(t, liveStudioURL)
	testutil.RunLiveScrape(t, New(), liveStudioURL, 2)
}
