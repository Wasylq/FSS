package uptimely

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/uptimelyutil"
)

func TestSiteCount(t *testing.T) {
	if len(sites) != 13 {
		t.Errorf("expected 13 sites, got %d", len(sites))
	}
}

// buildScraper mirrors init()'s per-site construction so URL matching can be
// asserted offline.
func buildScraper(cfg siteConfig) *uptimelyutil.Scraper {
	escaped := strings.ReplaceAll(cfg.Domain, ".", `\.`)
	re := regexp.MustCompile(fmt.Sprintf(`^https?://(?:www\.)?%s/(?:works/list/|actress/detail/)`, escaped))
	return uptimelyutil.New(uptimelyutil.SiteConfig{
		ID:      cfg.SiteID,
		Studio:  cfg.StudioName,
		Domain:  cfg.Domain,
		MatchRe: re,
	})
}

func TestNewSitesMatchURLs(t *testing.T) {
	cases := map[string][]string{
		"oppai": {
			"https://oppai-av.com/works/list/release",
			"https://oppai-av.com/actress/detail/123",
		},
		"ebody": {
			"https://av-e-body.com/works/list/release",
			"https://www.av-e-body.com/actress/detail/45",
		},
	}
	for id, urls := range cases {
		s := buildScraper(findSiteCfg(id))
		for _, u := range urls {
			if !s.MatchesURL(u) {
				t.Errorf("%s: expected MatchesURL(%q) = true", id, u)
			}
		}
		if s.MatchesURL("https://example.com/works/list/release") {
			t.Errorf("%s: should not match unrelated domain", id)
		}
	}
}

func findSiteCfg(id string) siteConfig {
	for _, c := range sites {
		if c.SiteID == id {
			return c
		}
	}
	panic("site not found: " + id)
}

func TestUniqueSiteIDs(t *testing.T) {
	seen := map[string]bool{}
	for _, cfg := range sites {
		if seen[cfg.SiteID] {
			t.Errorf("duplicate SiteID: %s", cfg.SiteID)
		}
		seen[cfg.SiteID] = true
	}
}
