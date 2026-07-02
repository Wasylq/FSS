//go:build integration

package realgirlsgonebad

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

const liveStudioURL = "https://www.realgirlsgonebad.com/tour/categories/videos_1_d.html"

func TestLiveRealGirlsGoneBad(t *testing.T) {
	testutil.SkipIfPlaceholder(t, liveStudioURL)
	testutil.RunLiveScrape(t, New(), liveStudioURL, 2)
}
