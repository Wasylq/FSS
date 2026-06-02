//go:build integration

package kbproductions

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveMelinaMay(t *testing.T) {
	testutil.RunLiveScrape(t, newScraper(sites[0]), "https://melina-may.com/videos", 3)
}

func TestLivePassionPOV(t *testing.T) {
	testutil.RunLiveScrape(t, newScraper(sites[1]), "https://passionpov.com/videos", 3)
}

func TestLiveVRAllure(t *testing.T) {
	testutil.RunLiveScrape(t, newScraper(sites[3]), "https://vrallure.com/videos", 3)
}

func TestLiveManPuppy(t *testing.T) {
	testutil.RunLiveScrape(t, newScraper(sites[4]), "https://www.manpuppy.com/videos", 3)
}
