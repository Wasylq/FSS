//go:build integration

package uptimely

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
	"github.com/Wasylq/FSS/internal/scrapers/uptimelyutil"
)

func newTestScraper(cfg siteConfig) *uptimelyutil.Scraper {
	escaped := strings.ReplaceAll(cfg.Domain, ".", `\.`)
	re := regexp.MustCompile(fmt.Sprintf(`^https?://(?:www\.)?%s/(?:works/list/|actress/detail/)`, escaped))

	return uptimelyutil.New(uptimelyutil.SiteConfig{
		ID:     cfg.SiteID,
		Studio: cfg.StudioName,
		Domain: cfg.Domain,
		Patterns: []string{
			cfg.Domain + "/works/list/series/{id}",
			cfg.Domain + "/works/list/release",
			cfg.Domain + "/works/list/date/{date}",
			cfg.Domain + "/works/list/genre/{id}",
			cfg.Domain + "/works/list/label/{id}",
			cfg.Domain + "/actress/detail/{id}",
		},
		MatchRe: re,
	})
}

func TestLiveHHHGroup(t *testing.T) {
	testutil.RunLiveScrape(t, newTestScraper(sites[2]), "https://hhh-av.com/works/list/release", 2)
}

func TestLiveIdeaPocket(t *testing.T) {
	testutil.RunLiveScrape(t, newTestScraper(sites[3]), "https://ideapocket.com/works/list/release", 2)
}
