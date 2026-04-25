//go:build integration

package puretaboo

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

// liveStudioURL — videos listing endpoint (always present).
const liveStudioURL = "https://www.puretaboo.com/en/videos"

func TestLivePureTaboo(t *testing.T) {
	testutil.SkipIfPlaceholder(t, liveStudioURL)
	testutil.RunLiveScrape(t, New(), liveStudioURL, 2)
}
