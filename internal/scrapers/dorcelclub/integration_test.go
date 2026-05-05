//go:build integration

package dorcelclub

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

const liveStudioURL = "https://www.dorcelclub.com/en/news-videos-x-marc-dorcel"

func TestLiveDorcelClub(t *testing.T) {
	testutil.SkipIfPlaceholder(t, liveStudioURL)
	testutil.RunLiveScrape(t, New(), liveStudioURL, 2)
}
