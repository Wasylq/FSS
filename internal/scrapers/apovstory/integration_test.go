//go:build integration

package apovstory

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

// liveStudioURL — the site root; the scraper handles the listing.
const liveStudioURL = "https://apovstory.com"

func TestLiveAPovStory(t *testing.T) {
	testutil.SkipIfPlaceholder(t, liveStudioURL)
	testutil.RunLiveScrape(t, New(), liveStudioURL, 2)
}
