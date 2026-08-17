//go:build integration

package tripforfuck

import (
	"testing"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
)

func TestLiveTripForFuck(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.tripforfuck.com/", 2)
}

func TestLiveTripForFuckMovie(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.tripforfuck.com/member/movie/368-1/index.html", 1)
}
