//go:build integration

package coedproductions

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/coedutil"
	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveNebraskaCoeds(t *testing.T) {
	testutil.RunLiveScrape(t, coedutil.New(sites[0]), "https://tour.nebraskacoeds.com/", 3)
}

func TestLiveAfterHoursExposed(t *testing.T) {
	testutil.RunLiveScrape(t, coedutil.New(sites[2]), "https://afterhoursexposed.com/", 3)
}
