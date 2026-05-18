//go:build integration

package sexmexpro

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/sexmexutil"
	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func newTestScraper(cfg siteConfig) *sexmexutil.Scraper {
	escaped := strings.ReplaceAll(cfg.Domain, ".", `\.`)
	re := regexp.MustCompile(fmt.Sprintf(`^https?://(?:www\.)?%s(/|$)`, escaped))

	return sexmexutil.New(sexmexutil.SiteConfig{
		ID:       cfg.SiteID,
		Studio:   cfg.StudioName,
		SiteBase: "https://" + cfg.Domain,
		Patterns: []string{
			cfg.Domain,
			cfg.Domain + "/tour/updates",
			cfg.Domain + "/tour/models/{slug}.html",
			cfg.Domain + "/tour/categories/{slug}.html",
		},
		MatchRe: re,
	})
}

func TestLiveSexMex(t *testing.T) {
	testutil.RunLiveScrape(t, newTestScraper(sites[1]), "https://sexmex.xxx/", 2)
}
