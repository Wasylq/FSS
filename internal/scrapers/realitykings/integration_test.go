//go:build integration

package realitykings

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

// liveStudioURL — pick a real performer with a stable catalog.
// Pattern: https://www.realitykings.com/pornstar/<id>/<slug>
const liveStudioURL = "https://www.realitykings.com/model/2439/ariella-ferrera"

func TestLiveRealityKings(t *testing.T) {
	testutil.SkipIfPlaceholder(t, liveStudioURL)
	testutil.RunLiveScrape(t, New(), liveStudioURL, 2)
}
