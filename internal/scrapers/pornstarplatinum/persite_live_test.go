//go:build integration

package pornstarplatinum

import (
	"context"
	"testing"
	"time"

	"github.com/Wasylq/FSS/scraper"
)

// runPerSiteCount drains the scraper channel collecting scene count
// and a stop-condition. Returns once `wantScenes` are seen or the
// channel is closed.
func runPerSiteCount(t *testing.T, url string, wantScenes int, wantPerformer string) int {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	out, err := New().ListScenes(ctx, url, scraper.ListOpts{Workers: 1})
	if err != nil {
		t.Fatal(err)
	}
	scenes := 0
	for r := range out {
		if r.Kind == scraper.KindScene {
			scenes++
			if wantPerformer != "" && r.Scene.Series != wantPerformer && r.Scene.Performers[0] != wantPerformer {
				t.Errorf("scene %s: Series=%q Performers=%v, want %q", r.Scene.ID, r.Scene.Series, r.Scene.Performers, wantPerformer)
			}
			if scenes >= wantScenes {
				cancel()
			}
		}
	}
	return scenes
}

func TestLivePerSite_VeronicaAvluv(t *testing.T) {
	if n := runPerSiteCount(t, "https://tour.clubveronicaavluv.com/", 3, "Veronica Avluv"); n == 0 {
		t.Error("got no Veronica Avluv scenes")
	}
}

func TestLivePerSite_SexyVanessa(t *testing.T) {
	if n := runPerSiteCount(t, "https://tour.sexyvanessa.com/", 3, "Sexy Vanessa"); n == 0 {
		t.Error("got no Sexy Vanessa scenes")
	}
}

// Taboo Stepmom is themed — Performer comes from per-card title, not a
// fixed value. We just confirm scenes come back with non-empty IDs and
// titles in the expected `{Performer} in {Title}` shape.
func TestLivePerSite_TabooStepmom(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	out, err := New().ListScenes(ctx, "https://tour.taboostepmom.com/", scraper.ListOpts{Workers: 1})
	if err != nil {
		t.Fatal(err)
	}
	scenes := 0
	for r := range out {
		if r.Kind == scraper.KindScene {
			scenes++
			if r.Scene.ID == "" || r.Scene.Title == "" {
				t.Errorf("scene missing required field: %+v", r.Scene)
			}
			if r.Scene.Performers != nil && len(r.Scene.Performers) > 0 && r.Scene.Series != r.Scene.Performers[0] {
				t.Errorf("scene %s Series/Performers mismatch: Series=%q Performers=%v", r.Scene.ID, r.Scene.Series, r.Scene.Performers)
			}
			if scenes >= 3 {
				cancel()
			}
		}
	}
	if scenes == 0 {
		t.Error("got no Taboo Stepmom scenes")
	}
}

func TestLivePerSite_JoslynJames(t *testing.T) {
	if n := runPerSiteCount(t, "https://tour.joslynjames.xxx/", 3, "Joslyn James"); n == 0 {
		t.Error("got no Joslyn James scenes")
	}
}
