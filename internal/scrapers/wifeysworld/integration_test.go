//go:build integration

package wifeysworld

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

const liveStudioURL = "https://wifeysworld.com/v3/tour/categories/updates_1_d.html"

func TestLiveWifeysWorld(t *testing.T) {
	testutil.SkipIfPlaceholder(t, liveStudioURL)
	testutil.RunLiveScrape(t, New(), liveStudioURL, 2)
}
