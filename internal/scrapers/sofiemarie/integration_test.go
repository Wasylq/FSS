//go:build integration

package sofiemarie

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveScrapeAll(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://sofiemariexxx.com/", 5)
}

func TestLiveScrapeModel(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://sofiemariexxx.com/models/sofie-marie.html", 5)
}

func TestLiveScrapeDVD(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://sofiemariexxx.com/dvds/Dirt-Road-Warriors.html", 3)
}
