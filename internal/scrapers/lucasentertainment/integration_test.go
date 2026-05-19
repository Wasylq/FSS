//go:build integration

package lucasentertainment

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveLucasEntertainment(t *testing.T) {
	testutil.RunLiveScrape(t, newTestScraper(sites[0]), "https://www.lucasentertainment.com", 3)
}

func TestLiveLucasRaunch(t *testing.T) {
	testutil.RunLiveScrape(t, newTestScraper(sites[1]), "https://www.lucasraunch.com", 2)
}
