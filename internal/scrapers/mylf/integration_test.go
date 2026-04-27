//go:build integration

package mylf

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveScrapeAll(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.mylf.com/", 5)
}

func TestLiveScrapeModel(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.mylf.com/models/penny-barber", 5)
}

func TestLiveScrapeSeries(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.mylf.com/series/features-ts", 5)
}
