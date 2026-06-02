//go:build integration

package wankitnow

import (
	"regexp"
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
	"github.com/Wasylq/FSS/internal/scrapers/wankitnowutil"
)

func newLiveScraper() *wankitnowutil.Scraper {
	return wankitnowutil.New(wankitnowutil.SiteConfig{
		ID:       "wankitnow",
		Domain:   "wankitnow.com",
		Studio:   "Wank It Now",
		Patterns: []string{"wankitnow.com"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?wankitnow\.com`),
	})
}

func TestLiveWankItNow(t *testing.T) {
	testutil.RunLiveScrape(t, newLiveScraper(), "https://www.wankitnow.com/", 2)
}

func TestLiveWankItNowModelPage(t *testing.T) {
	testutil.RunLiveScrape(t, newLiveScraper(), "https://www.wankitnow.com/models/chloe-toy", 2)
}
