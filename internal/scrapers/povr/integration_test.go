//go:build integration

package povr

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/povrutil"
	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func newTestScraper(cfg siteConfig) *povrutil.Scraper {
	escaped := strings.ReplaceAll(cfg.Domain, ".", `\.`)
	re := regexp.MustCompile(fmt.Sprintf(`^https?://(?:www\.)?%s(/|$)`, escaped))

	return povrutil.New(povrutil.SiteConfig{
		ID:       cfg.SiteID,
		Studio:   cfg.StudioName,
		SiteBase: "https://www." + cfg.Domain,
		Patterns: []string{cfg.Domain},
		MatchRe:  re,
	})
}

func TestLiveWankzVR(t *testing.T) {
	testutil.RunLiveScrape(t, newTestScraper(sites[3]), "https://www.wankzvr.com/", 2)
}
