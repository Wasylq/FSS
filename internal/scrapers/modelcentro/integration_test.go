//go:build integration

package modelcentro

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
	"github.com/Wasylq/FSS/scraper"
)

func TestLiveThejerkygirls(t *testing.T) {
	url := "https://thejerkygirls.com/videos"
	s, err := scraper.ForURL(url)
	if err != nil {
		t.Fatalf("no scraper matched %s: %v", url, err)
	}
	if s.ID() != "thejerkygirls" {
		t.Fatalf("expected thejerkygirls, got %s", s.ID())
	}
	testutil.RunLiveScrape(t, s, url, 2)
}

func TestLiveMugurporn(t *testing.T) {
	url := "https://mugurporn.com/videos"
	s, err := scraper.ForURL(url)
	if err != nil {
		t.Fatalf("no scraper matched %s: %v", url, err)
	}
	if s.ID() != "mugurporn" {
		t.Fatalf("expected mugurporn, got %s", s.ID())
	}
	testutil.RunLiveScrape(t, s, url, 2)
}

func TestLiveKinkyrubberworld(t *testing.T) {
	url := "https://www.kinkyrubberworld.com/videos"
	s, err := scraper.ForURL(url)
	if err != nil {
		t.Fatalf("no scraper matched %s: %v", url, err)
	}
	if s.ID() != "kinkyrubberworld" {
		t.Fatalf("expected kinkyrubberworld, got %s", s.ID())
	}
	testutil.RunLiveScrape(t, s, url, 2)
}
