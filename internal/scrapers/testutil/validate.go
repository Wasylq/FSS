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
	// Date is unavailable on some sites (e.g. AlternaDudes); warn but don't fail.
	if s.Date.IsZero() {
		t.Logf("scene %q has zero Date", s.ID)
	}
	// Duration is sometimes unavailable from list endpoints; warn but don't fail.
	// Cap is 7 days — generous but catches overflow/unit bugs. JAV compilations can exceed 40h.
	if s.Duration < 0 || s.Duration > 7*24*60*60 {
		t.Errorf("scene %q has implausible Duration %d (expected 0..604800)", s.ID, s.Duration)
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
// On transient network failures (0 scenes returned), retries once after a
// short pause before failing the test.
//
// Fails the test if no scenes are returned or any scene fails ValidateScene.
func RunLiveScrape(t *testing.T, s scraper.StudioScraper, studioURL string, limit int) {
	t.Helper()

	if !s.MatchesURL(studioURL) {
		t.Fatalf("scraper %s does not match URL %s", s.ID(), studioURL)
	}

	const maxAttempts = 2
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		count := runOnce(t, s, studioURL, limit, attempt < maxAttempts)
		if count > 0 {
			return
		}
		if attempt < maxAttempts {
			t.Logf("%s: 0 scenes on attempt %d, retrying after 3s", s.ID(), attempt)
			time.Sleep(3 * time.Second)
		}
	}
	t.Fatalf("%s: no scenes returned from %s after %d attempts", s.ID(), studioURL, maxAttempts)
}

func runOnce(t *testing.T, s scraper.StudioScraper, studioURL string, limit int, tolerateErrors bool) int {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	ch, err := s.ListScenes(ctx, studioURL, scraper.ListOpts{Workers: 3})
	if err != nil {
		if tolerateErrors {
			t.Logf("ListScenes(%s): %v (will retry)", studioURL, err)
			return 0
		}
		t.Fatalf("ListScenes(%s): %v", studioURL, err)
	}

	count := 0
	for result := range ch {
		switch result.Kind {
		case scraper.KindError:
			t.Logf("scene error: %v", result.Err)
			continue
		case scraper.KindTotal, scraper.KindStoppedEarly:
			continue
		case scraper.KindScene:
		}

		count++
		if count == 1 {
			t.Logf("first scene from %s: %+v", s.ID(), result.Scene)
		}

		ValidateScene(t, result.Scene)

		if count >= limit {
			cancel()
			break
		}
	}

	for range ch {
	}

	t.Logf("%s: validated %d scenes (limit %d)", s.ID(), count, limit)
	return count
}

// CollectScenes drains a SceneResult channel, returning all scenes.
// Progress and StoppedEarly signals are silently skipped.
// Errors fail the test via t.Errorf so the remaining scenes are still collected.
func CollectScenes(t *testing.T, ch <-chan scraper.SceneResult) []models.Scene {
	t.Helper()
	scenes, _ := collectAll(t, ch)
	return scenes
}

// CollectScenesWithStop drains a SceneResult channel, returning all scenes
// and whether a StoppedEarly signal was received.
func CollectScenesWithStop(t *testing.T, ch <-chan scraper.SceneResult) ([]models.Scene, bool) {
	t.Helper()
	return collectAll(t, ch)
}

func collectAll(t *testing.T, ch <-chan scraper.SceneResult) ([]models.Scene, bool) {
	t.Helper()
	var scenes []models.Scene
	stoppedEarly := false
	for r := range ch {
		switch r.Kind {
		case scraper.KindTotal:
			continue
		case scraper.KindStoppedEarly:
			stoppedEarly = true
			continue
		case scraper.KindError:
			t.Errorf("unexpected error: %v", r.Err)
			continue
		case scraper.KindScene:
			scenes = append(scenes, r.Scene)
		}
	}
	return scenes, stoppedEarly
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
