//go:build integration

package newsensations

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

const liveStudioURL = "https://www.newsensations.com/tour_ns/categories/movies_1_d.html"

func TestLiveNewSensations(t *testing.T) {
	testutil.SkipIfPlaceholder(t, liveStudioURL)
	testutil.RunLiveScrape(t, New(), liveStudioURL, 2)
}
