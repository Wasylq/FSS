//go:build integration

package gangbangcreampie

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveScrapeAll(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.gangbangcreampie.com/", 5)
}
