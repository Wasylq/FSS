//go:build integration

package iwantclips

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

// liveStudioURL — Diane Andrews store (stable, long-running).
// If gone, swap for any other active IWantClips store.
const liveStudioURL = "https://iwantclips.com/store/327/Diane-Andrews"

func TestLiveIWantClips(t *testing.T) {
	testutil.SkipIfPlaceholder(t, liveStudioURL)
	testutil.RunLiveScrape(t, New(), liveStudioURL, 2)
}
