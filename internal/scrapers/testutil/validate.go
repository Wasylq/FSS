// Package testutil provides helpers for scraper integration tests.
//
// These helpers are only useful from tests built with `-tags integration`,
// but the file itself has no build tag so static analysis (vet, lint) can
// reach it.
package testutil

import (
	"context"
	"fmt"
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

	for _, problem := range sceneProblems(s) {
		t.Error(problem)
	}
	for _, note := range sceneNotes(s) {
		t.Log(note)
	}
}

// sceneProblems returns one message per shape check the scene fails, or nil if
// it passes. Split out from ValidateScene so the rules are unit-testable
// without a failing *testing.T.
func sceneProblems(s models.Scene) []string {
	var out []string
	if s.ID == "" {
		out = append(out, "scene has empty ID")
	}
	if s.SiteID == "" {
		out = append(out, fmt.Sprintf("scene %q has empty SiteID", s.ID))
	}
	if s.Title == "" {
		out = append(out, fmt.Sprintf("scene %q has empty Title", s.ID))
	}
	if s.URL == "" {
		out = append(out, fmt.Sprintf("scene %q has empty URL", s.ID))
	} else if u, err := url.Parse(s.URL); err != nil || u.Scheme == "" || u.Host == "" {
		out = append(out, fmt.Sprintf("scene %q has malformed URL %q", s.ID, s.URL))
	}
	// Duration is sometimes unavailable from list endpoints; warn but don't fail.
	// Cap is 7 days — generous but catches overflow/unit bugs. JAV compilations can exceed 40h.
	if s.Duration < 0 || s.Duration > 7*24*60*60 {
		out = append(out, fmt.Sprintf("scene %q has implausible Duration %d (expected 0..604800)", s.ID, s.Duration))
	}
	if len(s.Performers) == 0 && s.Studio == "" {
		out = append(out, fmt.Sprintf("scene %q has neither Performers nor Studio", s.ID))
	}
	if s.ScrapedAt.IsZero() {
		out = append(out, fmt.Sprintf("scene %q has zero ScrapedAt", s.ID))
	}
	return out
}

// sceneNotes returns advisory messages that must not fail a test.
func sceneNotes(s models.Scene) []string {
	var out []string
	// Date is unavailable on some sites (e.g. AlternaDudes); warn but don't fail.
	if s.Date.IsZero() {
		out = append(out, fmt.Sprintf("scene %q has zero Date", s.ID))
	}
	return out
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
	scenes, stoppedEarly, errs := drain(ch)
	for _, err := range errs {
		t.Errorf("unexpected error: %v", err)
	}
	return scenes, stoppedEarly
}

// drain reads a SceneResult channel to completion, separating scenes, the
// StoppedEarly signal, and errors. Split out from collectAll so the channel
// bookkeeping is unit-testable without a failing *testing.T.
func drain(ch <-chan scraper.SceneResult) (scenes []models.Scene, stoppedEarly bool, errs []error) {
	for r := range ch {
		switch r.Kind {
		case scraper.KindTotal:
			continue
		case scraper.KindStoppedEarly:
			stoppedEarly = true
			continue
		case scraper.KindError:
			errs = append(errs, r.Err)
			continue
		case scraper.KindScene:
			scenes = append(scenes, r.Scene)
		}
	}
	return scenes, stoppedEarly, errs
}

// SkipIfPlaceholder skips the test if the URL still looks like a placeholder
// (contains "REPLACE-ME"). Use this for scrapers where the maintainer hasn't
// yet picked a verified live URL.
func SkipIfPlaceholder(t *testing.T, studioURL string) {
	t.Helper()
	if isPlaceholder(studioURL) {
		t.Skipf("placeholder URL — edit liveStudioURL in this file with a verified studio URL")
	}
}

func isPlaceholder(studioURL string) bool {
	return strings.Contains(studioURL, "REPLACE-ME")
}
