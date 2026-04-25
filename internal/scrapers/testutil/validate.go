// Package testutil provides helpers for scraper integration tests.
//
// These helpers are only useful from tests built with `-tags integration`,
// but the file itself has no build tag so static analysis (vet, lint) can
// reach it.
package testutil

import (
	"context"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/scraper"
)

// ValidateScene runs cheap shape checks on a scraped scene. It catches the
// common signs that a scraper is broken without making assumptions about the
// site's specific data. Anything that is *site-specific* (e.g. expected
// performer name, expected duration range) belongs in that scraper's own
// integration test, not here.
func ValidateScene(t *testing.T, s models.Scene) {
	t.Helper()

	if s.ID == "" {
		t.Errorf("scene has empty ID")
	}
	if s.SiteID == "" {
		t.Errorf("scene %q has empty SiteID", s.ID)
	}
	if s.Title == "" {
		t.Errorf("scene %q has empty Title", s.ID)
	}
	if s.URL == "" {
		t.Errorf("scene %q has empty URL", s.ID)
	} else if u, err := url.Parse(s.URL); err != nil || u.Scheme == "" || u.Host == "" {
		t.Errorf("scene %q has malformed URL %q", s.ID, s.URL)
	}
	if s.Date.IsZero() {
		t.Errorf("scene %q has zero Date", s.ID)
	}
	// Duration is sometimes unavailable from list endpoints; warn but don't fail.
	if s.Duration < 0 || s.Duration > 24*60*60 {
		t.Errorf("scene %q has implausible Duration %d (expected 0..86400)", s.ID, s.Duration)
	}
	if len(s.Performers) == 0 && s.Studio == "" {
		t.Errorf("scene %q has neither Performers nor Studio", s.ID)
	}
	if s.ScrapedAt.IsZero() {
		t.Errorf("scene %q has zero ScrapedAt", s.ID)
	}
}

// RunLiveScrape exercises a scraper against a live URL and validates the
// first `limit` scenes. It cancels the context after `limit` is reached so
// the scraper goroutine exits cleanly, then drains the channel.
//
// The first scene is logged in full (via t.Logf) so a developer running
// `go test -v` can eyeball the field mapping after a scraper change.
//
// Fails the test if no scenes are returned or any scene fails ValidateScene.
func RunLiveScrape(t *testing.T, s scraper.StudioScraper, studioURL string, limit int) {
	t.Helper()

	if !s.MatchesURL(studioURL) {
		t.Fatalf("scraper %s does not match URL %s", s.ID(), studioURL)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	ch, err := s.ListScenes(ctx, studioURL, scraper.ListOpts{Workers: 3})
	if err != nil {
		t.Fatalf("ListScenes(%s): %v", studioURL, err)
	}

	count := 0
	for result := range ch {
		if result.Err != nil {
			t.Errorf("scene error: %v", result.Err)
			continue
		}
		if result.Total > 0 || result.StoppedEarly {
			continue
		}

		count++
		if count == 1 {
			t.Logf("first scene from %s: %+v", s.ID(), result.Scene)
		}

		ValidateScene(t, result.Scene)

		if count >= limit {
			cancel() // stop the scraper goroutine
			break
		}
	}

	// Drain remaining results so the goroutine can exit cleanly.
	for range ch {
	}

	t.Logf("%s: validated %d scenes (limit %d)", s.ID(), count, limit)
	if count == 0 {
		t.Fatalf("%s: no scenes returned from %s", s.ID(), studioURL)
	}
}

// SkipIfPlaceholder skips the test if the URL still looks like a placeholder
// (contains "REPLACE-ME"). Use this for scrapers where the maintainer hasn't
// yet picked a verified live URL.
func SkipIfPlaceholder(t *testing.T, studioURL string) {
	t.Helper()
	if strings.Contains(studioURL, "REPLACE-ME") {
		t.Skipf("placeholder URL — edit liveStudioURL in this file with a verified studio URL")
	}
}
