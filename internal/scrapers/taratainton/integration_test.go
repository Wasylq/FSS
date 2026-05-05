//go:build integration

package taratainton

import (
	"context"
	"testing"
	"time"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
	"github.com/Wasylq/FSS/scraper"
)

// liveStudioURL — site home; WordPress sitemap drives discovery.
const liveStudioURL = "https://taratainton.com/home.html"

// TestLiveTaraTainton — disabled: many detail pages on taratainton.com return
// HTTP 500, which causes RunLiveScrape to fail. Re-enable when the site fixes
// the broken pages.
//
// func TestLiveTaraTainton(t *testing.T) {
// 	testutil.SkipIfPlaceholder(t, liveStudioURL)
// 	testutil.RunLiveScrape(t, New(), liveStudioURL, 2)
// }

func TestLiveTaraTaintonTolerant(t *testing.T) {
	s := New()
	if !s.MatchesURL(liveStudioURL) {
		t.Fatalf("scraper does not match URL %s", liveStudioURL)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	ch, err := s.ListScenes(ctx, liveStudioURL, scraper.ListOpts{Workers: 3})
	if err != nil {
		t.Fatalf("ListScenes: %v", err)
	}

	const limit = 2
	count := 0
	errors := 0
	for result := range ch {
		switch result.Kind {
		case scraper.KindError:
			errors++
			t.Logf("page error (expected — some pages return 500): %v", result.Err)
			continue
		case scraper.KindTotal, scraper.KindStoppedEarly:
			continue
		case scraper.KindScene:
		}

		count++
		if count == 1 {
			t.Logf("first scene: %+v", result.Scene)
		}
		testutil.ValidateScene(t, result.Scene)

		if count >= limit {
			cancel()
			break
		}
	}
	for range ch {
	}

	t.Logf("validated %d scenes, %d page errors", count, errors)
	if count == 0 {
		t.Fatalf("no scenes returned from %s", liveStudioURL)
	}
}
