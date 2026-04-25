//go:build integration

package taratainton

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

// liveStudioURL — site home; WordPress sitemap drives discovery.
const liveStudioURL = "https://taratainton.com/home.html"

func TestLiveTaraTainton(t *testing.T) {
	testutil.SkipIfPlaceholder(t, liveStudioURL)
	testutil.RunLiveScrape(t, New(), liveStudioURL, 2)
}
