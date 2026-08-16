//go:build integration

package housewifekelly

import (
	"testing"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
)

func TestLiveHousewifeKelly(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.housewifekelly.com/tour/", 2)
}

func TestLiveHousewifeKellyCategory(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.housewifekelly.com/tour/categories/3some.html", 2)
}
