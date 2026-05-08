//go:build integration

package rawfuckclub

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveScrape(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.rawfuckclub.com/browse/new", 2)
}

func TestLiveScrapeChannel(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.rawfuckclub.com/RawFuckClub", 2)
}
