//go:build integration

package interracialpass

import (
	"regexp"
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/darkreachmodernutil"
	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func newScraper() *darkreachmodernutil.Scraper {
	return darkreachmodernutil.New(darkreachmodernutil.SiteConfig{
		ID:         "interracialpass",
		SiteBase:   "https://www.interracialpass.com",
		Studio:     "Interracial Pass",
		TourPrefix: "/t1",
		MatchRe:    regexp.MustCompile(`^https?://(?:www\.)?interracialpass\.com`),
	})
}

func TestLiveInterracialPass(t *testing.T) {
	testutil.RunLiveScrape(t, newScraper(), "https://www.interracialpass.com/t1/", 2)
}
