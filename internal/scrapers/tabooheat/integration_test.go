//go:build integration

package tabooheat

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

// liveStudioURL — videos listing endpoint (always present).
const liveStudioURL = "https://www.tabooheat.com/en/videos"

func TestLiveTabooHeat(t *testing.T) {
	testutil.SkipIfPlaceholder(t, liveStudioURL)
	testutil.RunLiveScrape(t, New(), liveStudioURL, 2)
}
