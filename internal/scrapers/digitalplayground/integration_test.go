//go:build integration

package digitalplayground

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

// liveStudioURL — pick a real performer with a stable catalog.
// Pattern: https://www.digitalplayground.com/pornstar/<id>/<slug>
const liveStudioURL = "https://www.digitalplayground.com/modelprofile/153/anya-olsen"

func TestLiveDigitalPlayground(t *testing.T) {
	testutil.SkipIfPlaceholder(t, liveStudioURL)
	testutil.RunLiveScrape(t, New(), liveStudioURL, 2)
}
