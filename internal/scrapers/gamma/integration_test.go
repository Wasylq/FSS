//go:build integration

package gamma

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/gammautil"
	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func newTestScraper(cfg siteConfig) *siteScraper {
	var re *regexp.Regexp
	if cfg.MatchRe != "" {
		re = regexp.MustCompile(cfg.MatchRe)
	} else {
		escaped := strings.ReplaceAll(cfg.Domain, ".", `\.`)
		re = regexp.MustCompile(fmt.Sprintf(`^https?://(?:www\.)?%s`, escaped))
	}

	siteName := cfg.SiteName
	if siteName == "" && cfg.StudioName != "" {
		siteName = cfg.SiteID
	}

	gammaCfg := gammautil.SiteConfig{
		SiteID:      cfg.SiteID,
		SiteBase:    "https://www." + cfg.Domain,
		StudioName:  cfg.StudioName,
		SiteName:    siteName,
		RefererBase: cfg.RefererBase,
	}

	return &siteScraper{
		gamma:   gammautil.NewScraper(gammaCfg),
		config:  cfg,
		matchRe: re,
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

func TestLiveBurningAngel(t *testing.T) {
	testutil.RunLiveScrape(t, newTestScraper(findSite("burningangel")), "https://www.burningangel.com/", 2)
}

func TestLivePureTaboo(t *testing.T) {
	testutil.RunLiveScrape(t, newTestScraper(findSite("puretaboo")), "https://www.puretaboo.com/", 2)
}
