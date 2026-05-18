//go:build integration

package railway

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/railwayutil"
	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func newTestScraper(cfg siteConfig) *railwayutil.Scraper {
	escaped := strings.ReplaceAll(cfg.Domain, ".", `\.`)
	re := regexp.MustCompile(fmt.Sprintf(`^https?://(?:www\.)?%s`, escaped))

	return railwayutil.New(railwayutil.SiteConfig{
		ID:       cfg.SiteID,
		SiteCode: cfg.SiteCode,
		Studio:   cfg.StudioName,
		SiteBase: "https://" + cfg.Domain,
		Patterns: []string{
			cfg.Domain + "/#/models",
			cfg.Domain + "/#/models/{name}",
		},
		MatchRe: re,
	})
}

func TestLiveSmokingErotica(t *testing.T) {
	testutil.RunLiveScrape(t, newTestScraper(sites[0]), "https://smokingerotica.com/", 2)
}
