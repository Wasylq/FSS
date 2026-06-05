//go:build integration

package williamhiggins

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
	"github.com/Wasylq/FSS/internal/scrapers/whutil"
)

func newTestScraper(cfg siteConfig) *siteScraper {
	apiBase := fmt.Sprintf("https://backend.williamhiggins.com/%s/api/v1/", cfg.BackendSlug)
	if cfg.BackendSlug == "" {
		apiBase = fmt.Sprintf("https://www.%s/api/", cfg.Domain)
	}
	escaped := strings.ReplaceAll(cfg.Domain, ".", `\.`)
	re := regexp.MustCompile(fmt.Sprintf(`^https?://(?:www\.)?%s(?:/|$)`, escaped))

	return &siteScraper{
		wh: whutil.New(whutil.SiteConfig{
			SiteID:     cfg.SiteID,
			Domain:     cfg.Domain,
			StudioName: cfg.StudioName,
			APIBase:    apiBase,
			DetailPath: cfg.DetailPath,
		}),
		config: cfg,
		re:     re,
	}
}

func findSite(id string) siteConfig {
	for _, cfg := range sites {
		if cfg.SiteID == id {
			return cfg
		}
	}
	panic("site not found: " + id)
}

func TestLiveStr8Hell(t *testing.T) {
	testutil.RunLiveScrape(t, newTestScraper(findSite("str8hell")), "https://www.str8hell.com", 3)
}

func TestLiveWilliamHiggins(t *testing.T) {
	testutil.RunLiveScrape(t, newTestScraper(findSite("williamhiggins")), "https://www.williamhiggins.com", 3)
}
