//go:build integration

package xxcel

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
	"github.com/Wasylq/FSS/scraper"
)

func TestLiveXXCel(t *testing.T) {
	url := "https://xx-cel.com/movies/page-1/?sort=recent"
	s, err := scraper.ForURL(url)
	if err != nil {
		t.Fatalf("no scraper matched %s: %v", url, err)
	}
	testutil.RunLiveScrape(t, s, url, 3)
}

func TestLiveHeavyOnHotties(t *testing.T) {
	url := "https://www.heavyonhotties.com/movies/page-1/?sort=recent"
	s, err := scraper.ForURL(url)
	if err != nil {
		t.Fatalf("no scraper matched %s: %v", url, err)
	}
	if s.ID() != "heavyonhotties" {
		t.Fatalf("expected heavyonhotties, got %s", s.ID())
	}
	testutil.RunLiveScrape(t, s, url, 3)
}
