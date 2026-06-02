//go:build integration

package indiebucks

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveBoysSmokng(t *testing.T) {
	testutil.RunLiveScrape(t, newScraper(sites[0]), "https://boys-smoking.com/videos", 3)
}

func TestLiveBoysPissing(t *testing.T) {
	testutil.RunLiveScrape(t, newScraper(sites[1]), "https://www.boys-pissing.com/videos", 3)
}

func TestLiveBoundMuscleJocks(t *testing.T) {
	testutil.RunLiveScrape(t, newScraper(sites[2]), "https://www.boundmusclejocks.com/videos", 3)
}
