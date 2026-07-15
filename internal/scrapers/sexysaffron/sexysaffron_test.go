package sexysaffron

import (
	"regexp"
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/veutil"
	"github.com/Wasylq/FSS/scraper"
)

func newTestScraper() *veutil.Scraper {
	return veutil.New(veutil.SiteConfig{
		ID:             "sexysaffron",
		Studio:         "Saffron Bacchus",
		SiteBase:       "https://sexysaffron.com",
		MainCategoryID: videosCategoryID,
		Patterns:       []string{"sexysaffron.com"},
		MatchRe:        regexp.MustCompile(`^https?://(?:www\.)?sexysaffron\.com(?:/|$)`),
	})
}

func TestRegistered(t *testing.T) {
	s, err := scraper.ForURL("https://sexysaffron.com/videos/some-scene/")
	if err != nil {
		t.Fatalf("ForURL: %v", err)
	}
	if s.ID() != "sexysaffron" {
		t.Errorf("ForURL matched %q, want sexysaffron", s.ID())
	}
}

func TestMatchesURL(t *testing.T) {
	s := newTestScraper()
	cases := []struct {
		url   string
		match bool
	}{
		{"https://sexysaffron.com", true},
		{"https://sexysaffron.com/", true},
		{"https://www.sexysaffron.com/videos/meditation-joi-34/", true},
		{"http://sexysaffron.com/tag/blowjob/", true},
		{"https://saffronbacchus.com/", false},
		{"", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.match {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.match)
		}
	}
}

// Category 3 ("videos") is the parent of clips/free/shows and covers the whole
// ~695-scene catalogue. Pointing at 4 ("clips") would drop roughly half.
func TestVideosCategoryID(t *testing.T) {
	if videosCategoryID != 3 {
		t.Errorf("videosCategoryID = %d, want 3 (the parent videos category)", videosCategoryID)
	}
}

func TestID(t *testing.T) {
	if got := newTestScraper().ID(); got != "sexysaffron" {
		t.Errorf("ID() = %q", got)
	}
}
