//go:build integration

package girlsoutwest

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

// 18 > pageSize 16, so the main listing crosses a page boundary.
func TestLiveGirlsOutWest(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://tour.girlsoutwest.com/", 18)
}

func TestLiveGirlsOutWestModel(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://tour.girlsoutwest.com/models/Sage-Cherie.html", 2)
}
