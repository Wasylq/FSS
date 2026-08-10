package all

import (
	"context"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/Anastylosis/FSS/scraper"
)

// placeholderRe matches the {N}/{slug}/{id} tokens Patterns() uses for display.
var placeholderRe = regexp.MustCompile(`\{[^}]*\}`)

// sampleURL synthesises a URL the scraper claims, from its own Patterns().
//
// Patterns are display strings ("pornhub.com/model/{slug}"), so the tokens are
// filled with something innocuous and the result checked against MatchesURL —
// only a URL the scraper actually accepts is any use here. Returns false when no
// pattern can be turned into a match, which is common enough (patterns naming a
// query string, or a scraper matching only a very specific path shape) that the
// count is reported rather than ignored.
func sampleURL(s scraper.StudioScraper) (string, bool) {
	for _, p := range s.Patterns() {
		cand := placeholderRe.ReplaceAllString(p, "1")
		cand = strings.TrimSuffix(cand, "/")
		for _, u := range []string{cand, "https://" + cand, "https://www." + cand} {
			if !strings.HasPrefix(u, "http") {
				continue
			}
			if s.MatchesURL(u) {
				return u, true
			}
		}
	}
	return "", false
}

// TestScrapersExitOnCancelledContext checks that no scraper hangs when its
// context is already dead.
//
// Every scraper must select on ctx.Done() in its channel sends and check
// ctx.Err() in helper pagination loops (see CONTRIBUTING.md), but almost no package tests
// it — deleting the select breaks nothing else, and the cost is a goroutine that
// keeps fetching pages nobody will read, per cancelled scrape. testutil's
// AssertCancellable covers mid-flight cancellation but needs a per-scraper test
// server, so it can only ever be added a package at a time.
//
// This covers the whole registry in one go by cancelling *before* the call: a
// scraper that respects its context must close its channel almost immediately,
// and no request is made because the context is already dead — so this stays a
// fully offline test despite touching every site.
func TestScrapersExitOnCancelledContext(t *testing.T) {
	const perScraper = 15 * time.Second

	var tested, skipped int
	for _, s := range scraper.All() {
		u, ok := sampleURL(s)
		if !ok {
			skipped++
			continue
		}
		tested++

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		ch, err := s.ListScenes(ctx, u, scraper.ListOpts{Workers: 2})
		if err != nil {
			// Refusing outright is a valid response to a dead context.
			continue
		}

		done := make(chan struct{})
		go func() {
			defer close(done)
			for range ch {
			}
		}()

		select {
		case <-done:
		case <-time.After(perScraper):
			t.Errorf("%s: channel still open %v after ListScenes with an already-cancelled "+
				"context — the scraper is not respecting ctx and leaks a goroutine per "+
				"cancelled scrape (url %s)", s.ID(), perScraper, u)
		}
	}

	// Not a silent cap: report the coverage this test actually achieved so a
	// drop in it is visible rather than looking like a pass.
	t.Logf("cancellation checked on %d scrapers; %d skipped (no URL derivable from Patterns)", tested, skipped)
	if tested == 0 {
		t.Fatal("no scrapers were exercised — sampleURL is broken, not the scrapers")
	}
}
