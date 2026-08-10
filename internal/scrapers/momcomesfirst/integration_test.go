//go:build integration

package momcomesfirst

import (
	"testing"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
)

// liveStudioURL — site root; WordPress sitemap drives discovery.
const liveStudioURL = "https://momcomesfirst.com"

func TestLiveMomComesFirst(t *testing.T) {
	testutil.SkipIfPlaceholder(t, liveStudioURL)
	testutil.RunLiveScrape(t, New(), liveStudioURL, 2)
}
