package scraper

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Wasylq/FSS/models"
)

func collectResults(ch <-chan SceneResult) []SceneResult {
	var results []SceneResult
	for r := range ch {
		results = append(results, r)
	}
	return results
}

func scene(id string) models.Scene {
	return models.Scene{ID: id, SiteID: "test", ScrapedAt: time.Now().UTC()}
}

func TestPaginate_basic(t *testing.T) {
	out := make(chan SceneResult, 100)
	fetchPage := func(_ context.Context, page int) (PageResult, error) {
		switch page {
		case 1:
			return PageResult{
				Scenes: []models.Scene{scene("a"), scene("b")},
				Total:  4,
			}, nil
		case 2:
			return PageResult{
				Scenes: []models.Scene{scene("c"), scene("d")},
				Done:   true,
			}, nil
		default:
			t.Fatalf("unexpected page %d", page)
			return PageResult{}, nil
		}
	}

	Paginate(context.Background(), ListOpts{}, "test", out, fetchPage)
	close(out)

	results := collectResults(out)
	var scenes []string
	gotTotal := 0
	for _, r := range results {
		switch r.Kind {
		case KindScene:
			scenes = append(scenes, r.Scene.ID)
		case KindTotal:
			gotTotal = r.Total
		}
	}
	if gotTotal != 4 {
		t.Errorf("total = %d, want 4", gotTotal)
	}
	if len(scenes) != 4 {
		t.Errorf("got %d scenes, want 4: %v", len(scenes), scenes)
	}
}

func TestPaginate_emptyFirstPage(t *testing.T) {
	out := make(chan SceneResult, 100)
	fetchPage := func(_ context.Context, page int) (PageResult, error) {
		return PageResult{}, nil
	}

	Paginate(context.Background(), ListOpts{}, "test", out, fetchPage)
	close(out)

	results := collectResults(out)
	if len(results) != 0 {
		t.Errorf("expected no results, got %d", len(results))
	}
}

// A page that filtered all its items out (Continue set, no Scenes) must not
// end the traversal — later pages still have scenes.
func TestPaginate_continuePastEmptyPage(t *testing.T) {
	out := make(chan SceneResult, 100)
	fetchPage := func(_ context.Context, page int) (PageResult, error) {
		switch page {
		case 1:
			return PageResult{Scenes: []models.Scene{scene("a")}, Total: 3}, nil
		case 2:
			// Page had items (e.g. photo-only) that filtered to zero scenes.
			return PageResult{Continue: true}, nil
		case 3:
			return PageResult{Scenes: []models.Scene{scene("b")}, Done: true}, nil
		default:
			t.Fatalf("unexpected page %d", page)
			return PageResult{}, nil
		}
	}

	Paginate(context.Background(), ListOpts{}, "test", out, fetchPage)
	close(out)

	var scenes []string
	for _, r := range collectResults(out) {
		if r.Kind == KindScene {
			scenes = append(scenes, r.Scene.ID)
		}
	}
	if len(scenes) != 2 || scenes[0] != "a" || scenes[1] != "b" {
		t.Errorf("scenes = %v, want [a b] (page 2 filtered-empty must not stop)", scenes)
	}
}

// Continue with Done true stops the loop after the empty page.
func TestPaginate_continueDoneStops(t *testing.T) {
	out := make(chan SceneResult, 100)
	calls := 0
	fetchPage := func(_ context.Context, page int) (PageResult, error) {
		calls++
		if page == 1 {
			return PageResult{Scenes: []models.Scene{scene("a")}}, nil
		}
		return PageResult{Continue: true, Done: true}, nil
	}

	Paginate(context.Background(), ListOpts{}, "test", out, fetchPage)
	close(out)

	if calls != 2 {
		t.Errorf("fetched %d pages, want 2 (Done must stop)", calls)
	}
}

func TestPaginate_knownIDsStopsEarly(t *testing.T) {
	out := make(chan SceneResult, 100)
	fetchPage := func(_ context.Context, page int) (PageResult, error) {
		return PageResult{
			Scenes: []models.Scene{scene("new1"), scene("known"), scene("new2")},
			Total:  10,
		}, nil
	}

	opts := ListOpts{KnownIDs: map[string]bool{"known": true}}
	Paginate(context.Background(), opts, "test", out, fetchPage)
	close(out)

	results := collectResults(out)
	var scenes []string
	stoppedEarly := false
	for _, r := range results {
		switch r.Kind {
		case KindScene:
			scenes = append(scenes, r.Scene.ID)
		case KindStoppedEarly:
			stoppedEarly = true
		}
	}
	if !stoppedEarly {
		t.Error("expected StoppedEarly")
	}
	if len(scenes) != 1 || scenes[0] != "new1" {
		t.Errorf("scenes = %v, want [new1]", scenes)
	}
}

func TestPaginate_fetchError(t *testing.T) {
	out := make(chan SceneResult, 100)
	fetchPage := func(_ context.Context, page int) (PageResult, error) {
		return PageResult{}, fmt.Errorf("server error")
	}

	Paginate(context.Background(), ListOpts{}, "test", out, fetchPage)
	close(out)

	results := collectResults(out)
	if len(results) != 1 || results[0].Kind != KindError {
		t.Errorf("expected one error result, got %v", results)
	}
}

func TestPaginate_contextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	out := make(chan SceneResult, 100)
	called := false
	fetchPage := func(_ context.Context, page int) (PageResult, error) {
		called = true
		return PageResult{}, nil
	}

	Paginate(ctx, ListOpts{}, "test", out, fetchPage)
	close(out)

	if called {
		t.Error("fetchPage should not be called when context is already cancelled")
	}
}

func TestPaginate_delayRespected(t *testing.T) {
	out := make(chan SceneResult, 100)
	pages := 0
	fetchPage := func(_ context.Context, page int) (PageResult, error) {
		pages = page
		if page < 3 {
			return PageResult{Scenes: []models.Scene{scene(fmt.Sprintf("s%d", page))}}, nil
		}
		return PageResult{}, nil
	}

	start := time.Now()
	Paginate(context.Background(), ListOpts{Delay: 50 * time.Millisecond}, "test", out, fetchPage)
	close(out)
	elapsed := time.Since(start)

	if pages != 3 {
		t.Errorf("fetched %d pages, want 3", pages)
	}
	if elapsed < 90*time.Millisecond {
		t.Errorf("elapsed %v, expected >= 90ms (2 delays)", elapsed)
	}
}

func TestPaginate_noProgressWhenTotalZero(t *testing.T) {
	out := make(chan SceneResult, 100)
	fetchPage := func(_ context.Context, page int) (PageResult, error) {
		return PageResult{
			Scenes: []models.Scene{scene("a")},
			Total:  0,
			Done:   true,
		}, nil
	}

	Paginate(context.Background(), ListOpts{}, "test", out, fetchPage)
	close(out)

	for r := range out {
		if r.Kind == KindTotal {
			t.Error("should not send Progress when Total is 0")
		}
	}
}

func TestPaginate_progressSentOnce(t *testing.T) {
	out := make(chan SceneResult, 100)
	fetchPage := func(_ context.Context, page int) (PageResult, error) {
		if page <= 2 {
			return PageResult{Scenes: []models.Scene{scene(fmt.Sprintf("s%d", page))}, Total: 10}, nil
		}
		return PageResult{}, nil
	}

	Paginate(context.Background(), ListOpts{}, "test", out, fetchPage)
	close(out)

	totalCount := 0
	for r := range out {
		if r.Kind == KindTotal {
			totalCount++
		}
	}
	if totalCount != 1 {
		t.Errorf("Progress sent %d times, want 1", totalCount)
	}
}
