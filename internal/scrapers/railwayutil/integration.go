package railwayutil

import (
	"context"
	"testing"
	"time"

	"github.com/Wasylq/FSS/scraper"
)

func RunLiveTest(t *testing.T, s scraper.StudioScraper, studioURL string, limit int) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
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
		if r.Kind != scraper.KindScene {
			continue
		}
		count++
		sc := r.Scene
		if count == 1 {
			t.Logf("first scene: %+v", sc)
		}
		if sc.ID == "" {
			t.Errorf("scene has empty ID")
		}
		if sc.Title == "" {
			t.Errorf("scene %q has empty Title", sc.ID)
		}
		if sc.Duration <= 0 {
			t.Errorf("scene %q has no Duration", sc.ID)
		}
		if len(sc.Performers) == 0 {
			t.Errorf("scene %q has no Performers", sc.ID)
		}
		if sc.Thumbnail == "" {
			t.Errorf("scene %q has empty Thumbnail", sc.ID)
		}
		if count >= limit {
			cancel()
		}
	}
	for range ch {
	}
	if count == 0 {
		t.Fatal("got 0 scenes")
	}
	t.Logf("%s: validated %d scenes", s.ID(), count)
}
