//go:build integration

package puremature

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveScrapeAll(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://puremature.com/", 5)
}

func TestLiveScrapeModel(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://puremature.com/models/laura-bentley", 3)
}
