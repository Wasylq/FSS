//go:build integration

package manyvids

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Wasylq/FSS/scraper"
)

// TestLiveManyVids hits the real ManyVids API.
// Run with: go test -tags integration -v -timeout 120s ./internal/scrapers/manyvids/
const liveStudioURL = "https://www.manyvids.com/Profile/590705/bettie-bondage/Store/Videos"

func TestLiveManyVids(t *testing.T) {
	s := New()

	if !s.MatchesURL(liveStudioURL) {
		t.Fatalf("MatchesURL returned false for %s", liveStudioURL)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, err := s.ListScenes(ctx, liveStudioURL, scraper.ListOpts{Workers: 3})
	if err != nil {
		t.Fatalf("ListScenes: %v", err)
	}

	const limit = 5
	count := 0
	for result := range ch {
		if result.Err != nil {
			t.Errorf("scene error: %v", result.Err)
			continue
		}

		sc := result.Scene
		count++

		// Print the first scene as pretty JSON so we can eyeball the mapping.
		if count == 1 {
			data, _ := json.MarshalIndent(sc, "", "  ")
			t.Logf("First scene:\n%s", data)
		}

		// Spot-check required fields on every scene we receive.
		if sc.ID == "" {
			t.Errorf("scene %d: empty ID", count)
		}
		if sc.Title == "" {
			t.Errorf("scene %s: empty Title", sc.ID)
		}
		if sc.URL == "" {
			t.Errorf("scene %s: empty URL", sc.ID)
		}
		if sc.Date.IsZero() {
			t.Errorf("scene %s: zero Date", sc.ID)
		}
		if sc.Duration == 0 {
			t.Errorf("scene %s: zero Duration", sc.ID)
		}
		if len(sc.Performers) == 0 {
			t.Errorf("scene %s: no Performers", sc.ID)
		}
		if len(sc.PriceHistory) == 0 {
			t.Errorf("scene %s: no PriceHistory", sc.ID)
		}

		if count >= limit {
			cancel() // stop after limit scenes
			break
		}
	}

	// Drain remaining results after cancel so the goroutine can exit cleanly.
	for range ch {
	}

	t.Logf("Received %d scenes (limit %d)", count, limit)
	if count == 0 {
		t.Fatal("no scenes returned")
	}
}
