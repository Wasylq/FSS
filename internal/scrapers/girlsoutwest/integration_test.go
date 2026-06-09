//go:build integration

package girlsoutwest

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveGirlsOutWest(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://tour.girlsoutwest.com/", 2)
}

func TestLiveGirlsOutWestModel(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://tour.girlsoutwest.com/models/Sage-Cherie.html", 2)
}
