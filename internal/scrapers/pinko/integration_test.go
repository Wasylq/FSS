//go:build integration

package pinko

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
	"github.com/Wasylq/FSS/scraper"
)

func TestLivePinkoTGirls(t *testing.T) {
	url := "https://www.pinkotgirls.com/new-video.php"
	s, err := scraper.ForURL(url)
	if err != nil {
		t.Fatalf("no scraper matched %s: %v", url, err)
	}
	if s.ID() != "pinkotgirls" {
		t.Fatalf("expected pinkotgirls, got %s", s.ID())
	}
	testutil.RunLiveScrape(t, s, url, 3)
}

func TestLivePinkoClub(t *testing.T) {
	url := "https://www.pinkoclub.com/new-video.php"
	s, err := scraper.ForURL(url)
	if err != nil {
		t.Fatalf("no scraper matched %s: %v", url, err)
	}
	if s.ID() != "pinkoclub" {
		t.Fatalf("expected pinkoclub, got %s", s.ID())
	}
	testutil.RunLiveScrape(t, s, url, 3)
}
