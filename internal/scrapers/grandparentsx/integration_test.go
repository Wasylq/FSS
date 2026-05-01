//go:build integration

package grandparentsx

import (
	"context"
	"testing"
	"time"

	"github.com/Wasylq/FSS/scraper"
)

func TestLiveGrandparentsX(t *testing.T) {
	s := New()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	ch, err := s.ListScenes(ctx, "https://grandparentsx.com/", scraper.ListOpts{})
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
		if sc.Thumbnail == "" {
			t.Errorf("scene %q has empty Thumbnail", sc.ID)
		}
		if count >= 5 {
			cancel()
		}
	}
	if count == 0 {
		t.Fatal("got 0 scenes")
	}
	t.Logf("grandparentsx: validated %d scenes", count)
}
