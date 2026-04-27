//go:build integration

package seemomsuck

import (
	"context"
	"testing"
	"time"

	"github.com/Wasylq/FSS/scraper"
)

func runLive(t *testing.T, studioURL string, limit int) {
	t.Helper()
	s := New()
	if !s.MatchesURL(studioURL) {
		t.Fatalf("scraper does not match URL %s", studioURL)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	ch, err := s.ListScenes(ctx, studioURL, scraper.ListOpts{})
	if err != nil {
		t.Fatalf("ListScenes: %v", err)
	}

	count := 0
	for r := range ch {
		if r.Err != nil {
			t.Errorf("error: %v", r.Err)
			continue
		}
		if r.Total > 0 || r.StoppedEarly {
			continue
		}
		count++
		if count == 1 {
			t.Logf("first scene: %+v", r.Scene)
		}
		sc := r.Scene
		if sc.ID == "" {
			t.Errorf("scene has empty ID")
		}
		if sc.Title == "" {
			t.Errorf("scene %q has empty Title", sc.ID)
		}
		if sc.URL == "" {
			t.Errorf("scene %q has empty URL", sc.ID)
		}
		if len(sc.Performers) == 0 {
			t.Errorf("scene %q has no performers", sc.ID)
		}
		if sc.Thumbnail == "" {
			t.Errorf("scene %q has empty Thumbnail", sc.ID)
		}
		if count >= limit {
			cancel()
			break
		}
	}
	for range ch {
	}
	t.Logf("validated %d scenes (limit %d)", count, limit)
	if count == 0 {
		t.Fatal("no scenes returned")
	}
}

func TestLiveScrape(t *testing.T) {
	runLive(t, "https://www.seemomsuck.com", 5)
}

func TestLiveModelScrape(t *testing.T) {
	runLive(t, "https://www.seemomsuck.com/models/stacie-starr.html", 3)
}
