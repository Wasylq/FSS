//go:build integration

package pervcity

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
	"github.com/Wasylq/FSS/scraper"
)

func TestLivePervCity(t *testing.T) {
	url := "https://pervcity.com/categories/updates_1_d.html"
	s, err := scraper.ForURL(url)
	if err != nil {
		t.Fatalf("no scraper matched %s: %v", url, err)
	}
	if s.ID() != "pervcity" {
		t.Fatalf("expected pervcity, got %s", s.ID())
	}
	testutil.RunLiveScrape(t, s, url, 3)
}

func TestLiveAnalOverdose(t *testing.T) {
	url := "https://analoverdose.com/categories/movies_1_d.html"
	s, err := scraper.ForURL(url)
	if err != nil {
		t.Fatalf("no scraper matched %s: %v", url, err)
	}
	if s.ID() != "analoverdose" {
		t.Fatalf("expected analoverdose, got %s", s.ID())
	}
	testutil.RunLiveScrape(t, s, url, 3)
}

func TestLiveUpHerAsshole(t *testing.T) {
	url := "https://upherasshole.com/categories/movies_1_d.html"
	s, err := scraper.ForURL(url)
	if err != nil {
		t.Fatalf("no scraper matched %s: %v", url, err)
	}
	if s.ID() != "upherasshole" {
		t.Fatalf("expected upherasshole, got %s", s.ID())
	}
	testutil.RunLiveScrape(t, s, url, 3)
}
