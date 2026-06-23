//go:build integration

package hussiepass

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/hussieutil"
	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveHussiePass(t *testing.T) {
	testutil.RunLiveScrape(t, hussieutil.New(sites[0]), "https://hussiepass.com/", 3)
}

func TestLivePOVPornstars(t *testing.T) {
	testutil.RunLiveScrape(t, hussieutil.New(sites[1]), "https://www.povpornstars.com/", 3)
}

func TestLiveInterracialPOVs(t *testing.T) {
	testutil.RunLiveScrape(t, hussieutil.New(sites[2]), "https://interracialpovs.com/", 3)
}
