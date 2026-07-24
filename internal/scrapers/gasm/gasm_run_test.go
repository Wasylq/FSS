package gasm

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/scraper"
)

// These tests drive run() end-to-end against an httptest server. They mutate
// the package-level siteBase, so unlike the parser tests in gasm_test.go they
// must not call t.Parallel().

const emptyListingHTML = `<div class="_results _results_posts"></div>`

// listingPage2HTML is a distinct second page, so pagination can be observed
// rather than inferred.
const listingPage2HTML = `<div class="_results _results_posts">
<div class="_results_item _results_posts_item">
<div class="post_item video" data-post-id="300">
<a class="preview" href="/post/details/300"
   data-media-poster="https://cdn.example.com/thumb300.jpeg">
<img class="_image item_cover" src="https://cdn.example.com/thumb300.jpeg"/>
</a>
<span class="counter"><i class="far fa-clock"></i> <b>07:30</b></span>
<a class="post_title" href="/post/details/300" title="Third Scene">Third Scene</a>
<a class="post_channel" href="/studio/profile/teststudio">Channel: teststudio</a>
</div></div></div>
<div class="_pagination"><div class="paginationArea">
<a class="highlight">2</a>
<a href="?page=2" data-page="2" class="pageBtn" title="last">last</a>
</div></div>`

// detailFor renders the shared detail fixture under an arbitrary post id, so a
// server can answer /post/details/<id> for every id the listing advertises.
func detailFor(id string) string {
	switch id {
	case "200":
		// duration:null and cover:"" — exercises the listing fallbacks.
		return detailHTMLNullDuration
	default:
		return detailHTML
	}
}

type gasmServer struct {
	*httptest.Server
	mu           sync.Mutex
	listingPages []string // response per 1-based page
	profileCode  int
	listingCode  int
	detailCodes  map[string]int

	profileHits int
	listingHits int
	detailHits  []string
}

func newGasmServer(t *testing.T) *gasmServer {
	t.Helper()

	gs := &gasmServer{
		listingPages: []string{listingHTML, emptyListingHTML},
		detailCodes:  map[string]int{},
	}

	gs.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gs.mu.Lock()
		defer gs.mu.Unlock()

		switch {
		case strings.HasPrefix(r.URL.Path, "/studio/profile/"):
			gs.profileHits++
			if gs.profileCode != 0 {
				w.WriteHeader(gs.profileCode)
				return
			}
			_, _ = w.Write([]byte(profileHTML))

		case r.URL.Path == "/op/results/paginate":
			gs.listingHits++
			if gs.listingCode != 0 {
				w.WriteHeader(gs.listingCode)
				return
			}
			_ = r.ParseForm()
			page := r.FormValue("aParams[page]")
			idx := 0
			_, _ = fmt.Sscanf(page, "%d", &idx)
			if idx >= 1 && idx <= len(gs.listingPages) {
				_, _ = w.Write([]byte(gs.listingPages[idx-1]))
				return
			}
			_, _ = w.Write([]byte(emptyListingHTML))

		case strings.HasPrefix(r.URL.Path, "/post/details/"):
			id := strings.TrimPrefix(r.URL.Path, "/post/details/")
			gs.detailHits = append(gs.detailHits, id)
			if code, ok := gs.detailCodes[id]; ok {
				w.WriteHeader(code)
				return
			}
			_, _ = w.Write([]byte(detailFor(id)))

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	orig := siteBase
	siteBase = gs.URL
	t.Cleanup(func() {
		siteBase = orig
		gs.Close()
	})
	return gs
}

// results splits a scrape into its result kinds.
type results struct {
	scenes  []models.Scene
	errs    []error
	total   int
	stopped bool
}

func drain(t *testing.T, ch <-chan scraper.SceneResult) results {
	t.Helper()
	var r results
	for res := range ch {
		switch res.Kind {
		case scraper.KindScene:
			r.scenes = append(r.scenes, res.Scene)
		case scraper.KindError:
			r.errs = append(r.errs, res.Err)
		case scraper.KindTotal:
			r.total = res.Total
		case scraper.KindStoppedEarly:
			r.stopped = true
		}
	}
	return r
}

func runScrape(t *testing.T, gs *gasmServer, studioURL string, opts scraper.ListOpts) results {
	t.Helper()
	s := New()
	s.client = gs.Client()
	ch, err := s.ListScenes(context.Background(), studioURL, opts)
	if err != nil {
		t.Fatalf("ListScenes: %v", err)
	}
	return drain(t, ch)
}

func TestRunEndToEnd(t *testing.T) {
	gs := newGasmServer(t)
	r := runScrape(t, gs, "https://mmvfilms.com/", scraper.ListOpts{Workers: 2})

	if len(r.errs) != 0 {
		t.Fatalf("unexpected errors: %v", r.errs)
	}
	if len(r.scenes) != 2 {
		t.Fatalf("got %d scenes, want 2", len(r.scenes))
	}
	// profileHTML advertises 3 videos, so the progress total comes from there
	// rather than from the number of scenes actually returned.
	if r.total != 3 {
		t.Errorf("progress total = %d, want 3", r.total)
	}

	byID := map[string]models.Scene{}
	for _, sc := range r.scenes {
		byID[sc.ID] = sc
		if sc.SiteID != "gasm" {
			t.Errorf("scene %s: SiteID = %q, want gasm", sc.ID, sc.SiteID)
		}
		if sc.StudioURL != "https://mmvfilms.com/" {
			t.Errorf("scene %s: StudioURL = %q", sc.ID, sc.StudioURL)
		}
		if sc.ScrapedAt.IsZero() {
			t.Errorf("scene %s: ScrapedAt not set", sc.ID)
		}
	}
	if _, ok := byID["100"]; !ok {
		t.Errorf("scene 100 missing; got %v", byID)
	}
	if _, ok := byID["200"]; !ok {
		t.Errorf("scene 200 missing; got %v", byID)
	}
}

// resolveSlug fails before any request is made.
func TestRunUnresolvableURLErrorsWithoutFetching(t *testing.T) {
	gs := newGasmServer(t)
	r := runScrape(t, gs, "https://not-a-gasm-site.example/", scraper.ListOpts{Workers: 1})

	if len(r.errs) != 1 {
		t.Fatalf("got %d errors, want 1: %v", len(r.errs), r.errs)
	}
	if !strings.Contains(r.errs[0].Error(), "cannot resolve GASM studio slug") {
		t.Errorf("error = %v", r.errs[0])
	}
	if len(r.scenes) != 0 {
		t.Errorf("got %d scenes, want 0", len(r.scenes))
	}
	if gs.profileHits != 0 {
		t.Errorf("made %d profile requests, want 0 — slug resolution should fail first", gs.profileHits)
	}
}

func TestRunBootstrapFailureIsFatal(t *testing.T) {
	gs := newGasmServer(t)
	// 404 rather than 500: httpx fails fast on non-retryable 4xx, whereas a
	// 5xx would burn the 0s/2s/4s retry backoff on every run.
	gs.profileCode = http.StatusNotFound

	r := runScrape(t, gs, "https://mmvfilms.com/", scraper.ListOpts{Workers: 1})

	if len(r.errs) != 1 {
		t.Fatalf("got %d errors, want 1: %v", len(r.errs), r.errs)
	}
	if !strings.Contains(r.errs[0].Error(), "bootstrap") {
		t.Errorf("error should be wrapped as bootstrap: %v", r.errs[0])
	}
	if gs.listingHits != 0 {
		t.Errorf("fetched %d listings after a failed bootstrap, want 0", gs.listingHits)
	}
}

func TestRunListingFailureIsReportedAndStops(t *testing.T) {
	gs := newGasmServer(t)
	gs.listingCode = http.StatusNotFound

	r := runScrape(t, gs, "https://mmvfilms.com/", scraper.ListOpts{Workers: 1})

	if len(r.errs) != 1 {
		t.Fatalf("got %d errors, want 1: %v", len(r.errs), r.errs)
	}
	if !strings.Contains(r.errs[0].Error(), "listing page 1") {
		t.Errorf("error should name the failing page: %v", r.errs[0])
	}
	if len(r.scenes) != 0 {
		t.Errorf("got %d scenes, want 0", len(r.scenes))
	}
}

func TestRunEmptyListingYieldsNoScenes(t *testing.T) {
	gs := newGasmServer(t)
	gs.listingPages = []string{emptyListingHTML}

	r := runScrape(t, gs, "https://mmvfilms.com/", scraper.ListOpts{Workers: 1})

	if len(r.errs) != 0 {
		t.Fatalf("unexpected errors: %v", r.errs)
	}
	if len(r.scenes) != 0 {
		t.Errorf("got %d scenes, want 0", len(r.scenes))
	}
	if len(gs.detailHits) != 0 {
		t.Errorf("fetched %d details for an empty listing, want 0", len(gs.detailHits))
	}
}

func TestRunStopsEarlyOnKnownID(t *testing.T) {
	gs := newGasmServer(t)

	// 100 is the first item on page 1, so the walk stops immediately.
	r := runScrape(t, gs, "https://mmvfilms.com/", scraper.ListOpts{
		Workers:  1,
		KnownIDs: map[string]bool{"100": true},
	})

	if !r.stopped {
		t.Error("expected StoppedEarly to be emitted")
	}
	if len(r.scenes) != 0 {
		t.Errorf("got %d scenes, want 0 — the first item was already known", len(r.scenes))
	}
	if gs.listingHits != 1 {
		t.Errorf("fetched %d listing pages, want 1", gs.listingHits)
	}
}

func TestRunKnownIDMidPageKeepsEarlierScenes(t *testing.T) {
	gs := newGasmServer(t)

	// 200 is the second item, so 100 is dispatched before the walk stops.
	r := runScrape(t, gs, "https://mmvfilms.com/", scraper.ListOpts{
		Workers:  1,
		KnownIDs: map[string]bool{"200": true},
	})

	if !r.stopped {
		t.Error("expected StoppedEarly to be emitted")
	}
	if len(r.scenes) != 1 || r.scenes[0].ID != "100" {
		t.Errorf("want exactly scene 100 before the known ID, got %+v", r.scenes)
	}
}

func TestRunDetailFailureIsNotFatal(t *testing.T) {
	gs := newGasmServer(t)
	gs.detailCodes = map[string]int{"100": http.StatusNotFound}

	r := runScrape(t, gs, "https://mmvfilms.com/", scraper.ListOpts{Workers: 1})

	if len(r.errs) != 1 {
		t.Fatalf("got %d errors, want 1: %v", len(r.errs), r.errs)
	}
	if !strings.Contains(r.errs[0].Error(), "detail 100") {
		t.Errorf("error should name the failing detail: %v", r.errs[0])
	}
	// The sibling scene still comes through.
	if len(r.scenes) != 1 || r.scenes[0].ID != "200" {
		t.Errorf("want scene 200 to survive the failure, got %+v", r.scenes)
	}
}

// Scene 200's detail page has duration:null and cover:"", so run() must fall
// back to the duration and thumbnail advertised in the listing card.
func TestRunFallsBackToListingThumbnailAndDuration(t *testing.T) {
	gs := newGasmServer(t)

	r := runScrape(t, gs, "https://mmvfilms.com/", scraper.ListOpts{Workers: 1})

	var got *models.Scene
	for i := range r.scenes {
		if r.scenes[i].ID == "200" {
			got = &r.scenes[i]
		}
	}
	if got == nil {
		t.Fatalf("scene 200 missing: %+v", r.scenes)
	}
	if got.Thumbnail != "https://cdn.example.com/thumb200.jpeg" {
		t.Errorf("Thumbnail = %q, want the listing card's poster", got.Thumbnail)
	}
	if got.Duration != 300 {
		t.Errorf("Duration = %d, want 300 (05:00 from the listing card)", got.Duration)
	}
}

func TestRunWalksMultiplePages(t *testing.T) {
	gs := newGasmServer(t)
	gs.listingPages = []string{listingHTML, listingPage2HTML}

	r := runScrape(t, gs, "https://mmvfilms.com/", scraper.ListOpts{Workers: 2})

	if len(r.errs) != 0 {
		t.Fatalf("unexpected errors: %v", r.errs)
	}
	if len(r.scenes) != 3 {
		t.Fatalf("got %d scenes, want 3 across two pages", len(r.scenes))
	}
	// Page 2 advertises totalPages=2, so the walk stops without a third fetch.
	if gs.listingHits != 2 {
		t.Errorf("fetched %d listing pages, want 2 — totalPages should stop the walk", gs.listingHits)
	}
	ids := map[string]bool{}
	for _, sc := range r.scenes {
		ids[sc.ID] = true
	}
	if !ids["300"] {
		t.Errorf("scene 300 from page 2 missing: %v", ids)
	}
}

func TestRunHonoursContextCancellation(t *testing.T) {
	gs := newGasmServer(t)

	s := New()
	s.client = gs.Client()
	ctx, cancel := context.WithCancel(context.Background())
	ch, err := s.ListScenes(ctx, "https://mmvfilms.com/", scraper.ListOpts{Workers: 1})
	if err != nil {
		t.Fatalf("ListScenes: %v", err)
	}
	cancel()

	// The channel must close rather than block; a leaked goroutine would hang
	// this test rather than fail it, so guard with a timeout.
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range ch {
		}
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("run did not return after context cancellation")
	}
}

func TestRunAppliesDelayBetweenDetailFetches(t *testing.T) {
	gs := newGasmServer(t)

	const delay = 120 * time.Millisecond
	start := time.Now()
	r := runScrape(t, gs, "https://mmvfilms.com/", scraper.ListOpts{Workers: 1, Delay: delay})
	elapsed := time.Since(start)

	if len(r.scenes) != 2 {
		t.Fatalf("got %d scenes, want 2", len(r.scenes))
	}
	// Two details through a single worker, each preceded by the delay.
	if floor := 2 * delay; elapsed < floor {
		t.Errorf("elapsed %v < %v; the delay is not being applied per detail fetch", elapsed, floor)
	}
}

func TestListScenesDefaultsWorkerCount(t *testing.T) {
	gs := newGasmServer(t)

	// Workers unset must not deadlock or drop work; run() defaults it to 3.
	r := runScrape(t, gs, "https://mmvfilms.com/", scraper.ListOpts{})

	if len(r.errs) != 0 {
		t.Fatalf("unexpected errors: %v", r.errs)
	}
	if len(r.scenes) != 2 {
		t.Errorf("got %d scenes, want 2 with a defaulted worker count", len(r.scenes))
	}
}

func TestRunResolvesSlugFromProfileURL(t *testing.T) {
	gs := newGasmServer(t)

	r := runScrape(t, gs, "https://www.gasm.com/studio/profile/teststudio", scraper.ListOpts{Workers: 1})

	if len(r.errs) != 0 {
		t.Fatalf("unexpected errors: %v", r.errs)
	}
	if len(r.scenes) != 2 {
		t.Errorf("got %d scenes, want 2", len(r.scenes))
	}
}
