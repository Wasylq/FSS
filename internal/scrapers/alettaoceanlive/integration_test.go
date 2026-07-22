//go:build integration

package alettaoceanlive

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveAlettaOceanLive(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://alettaoceanlive.com", 3)
}
