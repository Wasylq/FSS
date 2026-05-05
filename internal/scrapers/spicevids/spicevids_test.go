package spicevids

import (
	"testing"

	"github.com/Wasylq/FSS/scraper"
)

func TestScraperInterface(t *testing.T) {
	var _ scraper.StudioScraper = &spicevidsScraper{}
}

func TestMatchesURL(t *testing.T) {
	s := &spicevidsScraper{}

	cases := []struct {
		name string
		url  string
		want bool
	}{
		{"root", "https://www.spicevids.com", true},
		{"scenes page", "https://www.spicevids.com/scenes", true},
		{"model URL", "https://www.spicevids.com/model/123/name", true},
		{"collection URL", "https://www.spicevids.com/collection/62061/adamandevevod", true},
		{"category URL", "https://www.spicevids.com/category/5/anal", true},
		{"no www", "https://spicevids.com/scenes", true},
		{"other domain", "https://www.example.com", false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := s.MatchesURL(c.url); got != c.want {
				t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.want)
			}
		})
	}
}
