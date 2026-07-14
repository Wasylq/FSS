package d2pass

import (
	"testing"
)

func TestSites(t *testing.T) {
	if len(sites) != 4 {
		t.Fatalf("got %d sites, want 4", len(sites))
	}
	ids := map[string]bool{}
	for _, c := range sites {
		if c.SiteID == "" || c.Domain == "" || c.StudioName == "" {
			t.Errorf("incomplete config: %+v", c)
		}
		if ids[c.SiteID] {
			t.Errorf("duplicate SiteID %q", c.SiteID)
		}
		ids[c.SiteID] = true
	}
	for _, want := range []string{"1pondo", "10musume", "pacopacomama", "muramura"} {
		if !ids[want] {
			t.Errorf("missing site %q", want)
		}
	}
}

func TestMatchesURL(t *testing.T) {
	byID := map[string]*Scraper{}
	for _, c := range sites {
		byID[c.SiteID] = newScraper(c)
	}

	cases := []struct {
		id    string
		url   string
		match bool
	}{
		// www, bare and the en. host all serve the same site.
		{"1pondo", "https://www.1pondo.tv/", true},
		{"1pondo", "https://1pondo.tv/", true},
		{"1pondo", "https://en.1pondo.tv/movies/072126_001/", true},
		{"1pondo", "https://www.10musume.com/", false},
		{"10musume", "https://www.10musume.com/movies/072126_01/", true},
		{"10musume", "https://10musume.com", true},
		{"pacopacomama", "https://www.pacopacomama.com/", true},
		{"muramura", "https://www.muramura.tv/", true},
		// Not on this platform — these must not be claimed.
		{"1pondo", "https://www.caribbeancom.com/", false},
		{"1pondo", "https://www.heyzo.com/", false},
		{"1pondo", "", false},
	}
	for _, c := range cases {
		s := byID[c.id]
		if s == nil {
			t.Fatalf("no scraper for %q", c.id)
		}
		if got := s.MatchesURL(c.url); got != c.match {
			t.Errorf("%s.MatchesURL(%q) = %v, want %v", c.id, c.url, got, c.match)
		}
	}
}

// No site may claim another's domain, or a bare-domain URL would be routed to
// whichever scraper happened to register first.
func TestNoCrossMatching(t *testing.T) {
	for _, a := range sites {
		s := newScraper(a)
		for _, b := range sites {
			if a.SiteID == b.SiteID {
				continue
			}
			u := "https://" + b.Domain + "/"
			if s.MatchesURL(u) {
				t.Errorf("%s wrongly matches %s", a.SiteID, u)
			}
		}
	}
}

func TestIDAndPatterns(t *testing.T) {
	for _, c := range sites {
		s := newScraper(c)
		if s.ID() != c.SiteID {
			t.Errorf("ID() = %q, want %q", s.ID(), c.SiteID)
		}
		if len(s.Patterns()) == 0 {
			t.Errorf("%s: no patterns", c.SiteID)
		}
	}
}
