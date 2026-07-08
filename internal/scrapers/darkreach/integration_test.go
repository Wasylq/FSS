//go:build integration

package darkreach

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
	"github.com/Wasylq/FSS/scraper"
)

func TestLiveHammerBoys(t *testing.T) {
	url := "https://hammerboys.tv/categories/updates_1_d.html"
	s, err := scraper.ForURL(url)
	if err != nil {
		t.Fatalf("no scraper matched %s: %v", url, err)
	}
	if s.ID() != "hammerboys" {
		t.Fatalf("expected hammerboys, got %s", s.ID())
	}
	testutil.RunLiveScrape(t, s, url, 3)
}
