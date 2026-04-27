//go:build integration

package houseofyre

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveScrape(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.houseofyre.com", 5)
}

func TestLiveModelScrape(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.houseofyre.com/models/AshlynPeaks.html", 3)
}
