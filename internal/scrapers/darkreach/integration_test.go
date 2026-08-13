//go:build integration

package darkreach

import (
	"testing"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
	"github.com/Anastylosis/FSS/scraper"
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

func TestLiveEvolvedFightsLez(t *testing.T) {
	url := "https://evolvedfightslez.com/categories/updates_1_d.html"
	s, err := scraper.ForURL(url)
	if err != nil {
		t.Fatalf("no scraper matched %s: %v", url, err)
	}
	if s.ID() != "evolvedfightslez" {
		t.Fatalf("expected evolvedfightslez, got %s", s.ID())
	}
	testutil.RunLiveScrape(t, s, url, 3)
}

// thelisaann is the marketing listing on a host that also serves a separate VOD
// tour (`thelisaannvod`). Its match pattern was narrowed to the two paths it
// actually serves so the tour's URLs stop resolving here; this pins that the
// narrowing did not also cost it its own catalogue.
func TestLiveTheLisaAnn(t *testing.T) {
	url := "https://www.thelisaann.com/"
	s, err := scraper.ForURL(url)
	if err != nil {
		t.Fatalf("no scraper matched %s: %v", url, err)
	}
	if s.ID() != "thelisaann" {
		t.Fatalf("expected thelisaann, got %s", s.ID())
	}
	testutil.RunLiveScrape(t, s, url, 3)
}
