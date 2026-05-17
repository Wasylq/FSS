//go:build integration

package data18

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveScrapeStudio(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.data18.com/studios/mylf", 5)
}

func TestLiveScrapePerformer(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.data18.com/name/annie-king", 5)
}

func TestLiveScrapeTag(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.data18.com/tags/milf-hot-moms", 5)
}

func TestLiveScrapeSeries(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.data18.com/studios/elegant-angel/movie-series-milf-dreams", 5)
}
