//go:build integration

package mindcontroltheatre

import (
	"testing"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
)

func TestLiveMindControlTheatre(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://mindcontroltheatre.com/", 2)
}

// A single movie URL skips the sitemap; it also proves the age cookie clears
// the interstitial on a page fetched directly.
func TestLiveMindControlTheatreMovie(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://mindcontroltheatre.com/movie/ccvr-diagnostic-testing", 1)
}
