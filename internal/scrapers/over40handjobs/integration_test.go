//go:build integration

package over40handjobs

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

const liveURL = "https://www.over40handjobs.com/updates.htm"

func TestLiveOver40Handjobs(t *testing.T) {
	testutil.SkipIfPlaceholder(t, liveURL)
	testutil.RunLiveScrape(t, New(), liveURL, 2)
}
