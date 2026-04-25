//go:build integration

package manyvids

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

// liveStudioURL — Bettie Bondage profile (long-running, large catalog).
// If this 404s, swap for any other active ManyVids profile.
const liveStudioURL = "https://www.manyvids.com/Profile/590705/bettie-bondage/Store/Videos"

func TestLiveManyVids(t *testing.T) {
	testutil.SkipIfPlaceholder(t, liveStudioURL)
	testutil.RunLiveScrape(t, New(), liveStudioURL, 3)
}
