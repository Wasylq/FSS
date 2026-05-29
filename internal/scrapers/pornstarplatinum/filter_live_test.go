//go:build integration

package pornstarplatinum

import (
	"context"
	"testing"
	"time"

	"github.com/Wasylq/FSS/scraper"
)

// TestLivePornstarPlatinumFilteredToDeeWilliams verifies the per-URL
// performer filter the user explicitly tested: scraping
// `tour.deewilliams.xxx` should yield only Dee Williams scenes, not
// the whole 4720-scene network catalogue. We bound the walk to a few
// pages with a context cancellation so the test stays under a minute,
// then assert every emitted scene is hers.
func TestLivePornstarPlatinumFilteredToDeeWilliams(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	out, err := New().ListScenes(ctx, "https://tour.deewilliams.xxx/index.php", scraper.ListOpts{
		Workers: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	scenes := 0
	misattributed := 0
	for r := range out {
		if r.Kind == scraper.KindScene {
			scenes++
			if r.Scene.Series != "Dee Williams" {
				misattributed++
				t.Errorf("scene %s attributed to %q, want Dee Williams", r.Scene.ID, r.Scene.Series)
			}
			// Stop after a handful to keep the test bounded — we just
			// need to confirm the filter is correctly applied, not walk
			// the whole catalogue.
			if scenes >= 3 {
				cancel()
			}
		}
	}
	if scenes == 0 {
		t.Error("filter returned no Dee Williams scenes — check filter logic or catalogue contents")
	}
	if misattributed > 0 {
		t.Errorf("%d/%d scenes had wrong performer attribution", misattributed, scenes)
	}
}
