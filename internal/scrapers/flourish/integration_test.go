//go:build integration

package flourish

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/flourishutil"
	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveTheFlourishXXX(t *testing.T) {
	testutil.RunLiveScrape(t, flourishutil.New(sites[0]), "https://tour.theflourishxxx.com/", 3)
}

func TestLiveTheFlourishPOV(t *testing.T) {
	testutil.RunLiveScrape(t, flourishutil.New(sites[1]), "https://tour.theflourishpov.com/", 2)
}
