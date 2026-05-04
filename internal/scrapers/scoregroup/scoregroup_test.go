package scoregroup

import (
	"testing"

	"github.com/Wasylq/FSS/scraper"
)

func TestAllSitesRegister(t *testing.T) {
	if len(sites) != 93 {
		t.Errorf("expected 93 sites, got %d", len(sites))
	}
}

func TestScraperInterface(t *testing.T) {
	for _, cfg := range sites {
		s := newScraper(cfg)
		var _ scraper.StudioScraper = s
	}
}

func TestMatchesURL(t *testing.T) {
	cases := []struct {
		siteID string
		url    string
		want   bool
	}{
		{"50plusmilfs", "https://www.50plusmilfs.com", true},
		{"50plusmilfs", "https://50plusmilfs.com/xxx-milf-videos/?page=1", true},
		{"50plusmilfs", "https://www.50plusmilfs.com/xxx-milf-videos/Zena-Rey/80992/", true},
		{"scoreland", "https://www.scoreland.com/big-boob-videos/", true},
		{"scoreland", "https://scoreland.com", true},
		{"18eighteen", "https://www.18eighteen.com/xxx-teen-videos/", true},
		{"xlgirls", "https://www.xlgirls.com/bbw-videos/", true},
		{"legsex", "https://www.legsex.com/foot-fetish-videos/", true},
		{"50plusmilfs", "https://www.example.com", false},
		{"scoreland", "https://www.scoreland2.com", false},
	}

	scraperMap := map[string]*siteScraper{}
	for _, cfg := range sites {
		scraperMap[cfg.SiteID] = newScraper(cfg)
	}

	for _, c := range cases {
		s, ok := scraperMap[c.siteID]
		if !ok {
			t.Fatalf("no scraper for %q", c.siteID)
		}
		if got := s.MatchesURL(c.url); got != c.want {
			t.Errorf("%s.MatchesURL(%q) = %v, want %v", c.siteID, c.url, got, c.want)
		}
	}
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

func TestAllSitesHaveVideosPath(t *testing.T) {
	for _, cfg := range sites {
		if cfg.VideosPath == "" {
			t.Errorf("site %s has empty VideosPath", cfg.SiteID)
		}
	}
}
