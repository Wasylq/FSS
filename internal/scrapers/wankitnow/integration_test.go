//go:build integration

package wankitnow

import (
	"regexp"
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
	"github.com/Wasylq/FSS/internal/scrapers/wankitnowutil"
)

func TestLiveWankItNow(t *testing.T) {
	s := wankitnowutil.New(wankitnowutil.SiteConfig{
		ID:       "wankitnow",
		Domain:   "wankitnow.com",
		Studio:   "Wank It Now",
		Patterns: []string{"wankitnow.com"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?wankitnow\.com`),
	})
	testutil.RunLiveScrape(t, s, "https://www.wankitnow.com/", 2)
}
