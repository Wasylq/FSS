//go:build integration

package vixen

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
	"github.com/Wasylq/FSS/internal/scrapers/vixenutil"
)

func TestLiveVixen(t *testing.T) {
	testutil.RunLiveScrape(t, vixenutil.New(sites[0]), "https://www.vixen.com/", 3)
}

func TestLiveBlacked(t *testing.T) {
	testutil.RunLiveScrape(t, vixenutil.New(sites[1]), "https://www.blacked.com/", 2)
}
