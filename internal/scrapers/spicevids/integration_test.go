//go:build integration

package spicevids

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
	"github.com/Wasylq/FSS/scraper"
)

func TestLiveCollection(t *testing.T) {
	url := "https://www.spicevids.com/collection/62061/adamandevevod"
	s, err := scraper.ForURL(url)
	if err != nil {
		t.Fatalf("no scraper matched %s: %v", url, err)
	}
	if s.ID() != "spicevids" {
		t.Fatalf("expected spicevids, got %s", s.ID())
	}
	testutil.RunLiveScrape(t, s, url, 2)
}

func TestLiveGeneric(t *testing.T) {
	url := "https://www.spicevids.com/scenes"
	s, err := scraper.ForURL(url)
	if err != nil {
		t.Fatalf("no scraper matched %s: %v", url, err)
	}
	if s.ID() != "spicevids" {
		t.Fatalf("expected spicevids, got %s", s.ID())
	}
	testutil.RunLiveScrape(t, s, url, 2)
}
