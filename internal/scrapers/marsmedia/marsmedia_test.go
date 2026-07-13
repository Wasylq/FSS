package marsmedia

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/natscmsutil"
)

func TestSitesTable(t *testing.T) {
	seen := map[string]bool{}
	for _, cfg := range sites {
		if cfg.ID == "" {
			t.Errorf("empty ID in sites table")
		}
		if seen[cfg.ID] {
			t.Errorf("duplicate ID: %q", cfg.ID)
		}
		seen[cfg.ID] = true
		if cfg.CMSAreaID == "" {
			t.Errorf("site %q missing CMSAreaID", cfg.ID)
		}
		if cfg.SiteBase == "" {
			t.Errorf("site %q missing SiteBase", cfg.ID)
		}
		if cfg.MatchRe == nil {
			t.Errorf("site %q missing MatchRe", cfg.ID)
		}
	}
	// 12 of the 14 stashdb children share the My Gay Cash NATS CMS;
	// tgirlplaytime.com and twotgirls.com run Nebula CMS and need a
	// separate package.
	if len(sites) != 12 {
		t.Errorf("expected 12 sites, got %d", len(sites))
	}
}

func TestMatchesURL(t *testing.T) {
	get := func(id string) *natscmsutil.Scraper {
		for _, cfg := range sites {
			if cfg.ID == id {
				return natscmsutil.New(cfg)
			}
		}
		return nil
	}
	cases := []struct {
		id, url string
		want    bool
	}{
		{"bearfilms", "https://www.bearfilms.com/", true},
		{"bearfilms", "https://tour.bearfilms.com/", true},
		{"bearfilms", "http://bearfilms.com/anything", true},
		{"bearfilms", "https://www.barebackcumpigs.com/", false},
		{"barebackcumpigs", "https://www.barebackcumpigs.com/", true},
		{"barebackcumpigs", "https://www.barebackthathole.com/", false},
		{"hardbritlads", "https://www.hardbritlads.com/tour/", true},
		// Substring/prefix traps — "bearfilms.com" must not match a host
		// like "bearfilmsfake.com".
		{"bearfilms", "https://bearfilmsfake.com/", false},
	}
	for _, c := range cases {
		s := get(c.id)
		if s == nil {
			t.Fatalf("unknown ID %q", c.id)
		}
		if got := s.MatchesURL(c.url); got != c.want {
			t.Errorf("MatchesURL[%s](%q) = %v, want %v", c.id, c.url, got, c.want)
		}
	}
}
