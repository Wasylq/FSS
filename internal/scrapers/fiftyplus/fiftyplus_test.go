package fiftyplus

import (
	"testing"

	"github.com/Wasylq/FSS/scraper"
)

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url  string
		want bool
	}{
		{"https://www.50plusmilfs.com", true},
		{"https://50plusmilfs.com/xxx-milf-videos/?page=1", true},
		{"https://www.50plusmilfs.com/xxx-milf-videos/Zena-Rey/80992/", true},
		{"https://www.example.com", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.want {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.want)
		}
	}
}

func TestScraperInterface(t *testing.T) {
	var _ scraper.StudioScraper = New()
}
