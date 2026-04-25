//go:build integration

package clips4sale

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

// liveStudioURL — pick a real, stable C4S studio.
// Pattern: https://www.clips4sale.com/studio/<id>/<slug>
const liveStudioURL = "https://www.clips4sale.com/studio/21571/tara-tainton"

func TestLiveClips4Sale(t *testing.T) {
	testutil.SkipIfPlaceholder(t, liveStudioURL)
	testutil.RunLiveScrape(t, New(), liveStudioURL, 2)
}
