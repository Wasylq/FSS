package spicevids

import (
	"testing"

	"github.com/Wasylq/FSS/scraper"
)

func TestCollectionCount(t *testing.T) {
	if len(collections) != 1000 {
		t.Errorf("expected 1000 collections, got %d", len(collections))
	}
}

func TestScraperInterface(t *testing.T) {
	for _, cfg := range collections {
		s := &collectionScraper{config: cfg}
		var _ scraper.StudioScraper = s
	}
}

func TestMatchesURL(t *testing.T) {
	lookup := make(map[int]*collectionScraper)
	for _, cfg := range collections {
		s := &collectionScraper{config: cfg}
		lookup[cfg.CollectionID] = s
	}

	cases := []struct {
		name   string
		url    string
		siteID string
		want   bool
	}{
		{"adamandeve match", "https://www.spicevids.com/collection/62061/adamandevevod", "sv-adamandevevod", true},
		{"wrong ID", "https://www.spicevids.com/collection/99999/unknown", "sv-adamandevevod", false},
		{"not a collection URL", "https://www.spicevids.com/scenes", "sv-adamandevevod", false},
		{"different domain", "https://www.example.com/collection/62061", "sv-adamandevevod", false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := lookup[62061]
			if s == nil {
				t.Fatal("adamandevevod collection not found")
			}
			if got := s.MatchesURL(c.url); got != c.want {
				t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.want)
			}
		})
	}
}

func TestGenericFallback(t *testing.T) {
	lookup := make(map[int]*collectionScraper)
	for _, cfg := range collections {
		lookup[cfg.CollectionID] = &collectionScraper{config: cfg}
	}
	g := &genericScraper{lookup: lookup}

	cases := []struct {
		name string
		url  string
		want bool
	}{
		{"model URL", "https://www.spicevids.com/model/123/name", true},
		{"scenes page", "https://www.spicevids.com/scenes", true},
		{"known collection", "https://www.spicevids.com/collection/62061/adamandevevod", false},
		{"unknown collection", "https://www.spicevids.com/collection/1/unknown", true},
		{"other domain", "https://www.example.com", false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := g.MatchesURL(c.url); got != c.want {
				t.Errorf("genericScraper.MatchesURL(%q) = %v, want %v", c.url, got, c.want)
			}
		})
	}
}

func TestUniqueSiteIDs(t *testing.T) {
	seen := map[string]bool{}
	for _, cfg := range collections {
		if seen[cfg.SiteID] {
			t.Errorf("duplicate SiteID: %s", cfg.SiteID)
		}
		seen[cfg.SiteID] = true
	}
}
