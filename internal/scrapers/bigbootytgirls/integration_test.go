//go:build integration

package bigbootytgirls

import (
	"testing"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
)

func TestLiveBigBootyTGirls(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://bigbootytgirls.com/", 2)
}

// The listing is reachable under two aliases; both must walk the same tour.
func TestLiveBigBootyTGirlsUpdatesListing(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://bigbootytgirls.com/categories/updates_1_d.html", 2)
}
