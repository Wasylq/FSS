package fratx

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
	"github.com/Anastylosis/FSS/models"
	"github.com/Anastylosis/FSS/scraper"
)

func card(id, title, date string) string {
	return fmt.Sprintf(`<div class="video-item">
  <div class="video-thumb-wrap">
    <a href="trailer.php?id=%s"><img src="https://cdn.example/thumbs/%s-1x.jpg" data-alt-src="a;b;c" class="video-thumb"></a>
  </div>
  <div class="video-info">
    <span class="video-title">%s</span>
    <span class="video-date">%s</span>
  </div>
</div>`, id, id, title, date)
}

// detailPage puts a "Related Videos" grid after the info block, exactly as the
// live page does — its cards carry titles and dates of their own.
func detailPage(id, title, desc string, tags, models []string) string {
	var tagHTML, modelHTML strings.Builder
	for i, tg := range tags {
		fmt.Fprintf(&tagHTML, `<a href="category.php?id=%d" class="tag"><span class="tag-text">%s</span></a>`, 10+i, tg)
	}
	for i, m := range models {
		fmt.Fprintf(&modelHTML, `<li><a href="sets.php?id=%d">%s</a></li>`, 20+i, m)
	}
	return fmt.Sprintf(`<html><body>
<div class="VideoBannerWrap">
  <div class="VideoPlayer">
    <video id="video_%s" poster="https://cdn.example/poster/%s.jpg"><source src="https://cdn.example/%s/playlist.m3u8"></video>
  </div>
  <div class="VideoInfoWrap">
    <div class="VideoTagsWrap">%s</div>
    <div class="ModelNamesWrap"><ul class="ModelNames">%s</ul></div>
    <div class="info"><div class="name"> <span>%s</span></div><div class="date"></div></div>
    <div class="VideoDescription"> September 10th, 2025 - %s</div>
  </div><!-- END VideoInfoWrap -->
</div>
<h2 class="video-header">Related Videos</h2>
<div class="video-grid">%s</div>
</body></html>`, id, id, id, tagHTML.String(), modelHTML.String(), title, desc,
		card("999", "A RELATED SCENE", "Jan 1, 2001"))
}

type stubSite struct {
	*httptest.Server
	mu   sync.Mutex
	hits map[string]int
}

func (s *stubSite) hit(p string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hits[p]++
}

func (s *stubSite) count(p string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.hits[p]
}

// newSite serves two categories that overlap on one scene, plus detail pages.
// Past a category's last page it re-serves page 1, which is what the live tour
// does, so the walk has to stop on "no new ids".
func newSite(t *testing.T) *stubSite {
	t.Helper()
	cats := map[string][][]string{
		"9": {
			{card("455", "CAMPUS COCKS", "Sep 10, 2025"), card("452", "STUDS SPORTIN COCKS", "Jul 30, 2025")},
			{card("430", "OLDER ONE", "Jan 5, 2024")},
		},
		// Overlaps category 9 on 452 — the union has to dedupe.
		"12": {{card("452", "STUDS SPORTIN COCKS", "Jul 30, 2025"), card("460", "NEWEST", "Oct 22, 2025")}},
	}
	site := &stubSite{hits: map[string]int{}}
	site.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		site.hit(r.URL.Path + "?" + r.URL.RawQuery)
		switch r.URL.Path {
		case "/":
			_, _ = fmt.Fprint(w, `<html><body>
<a href="/category.php?id=9">nine</a><a href="/category.php?id=12">twelve</a>
<a href="/category.php?id=9">nine again</a></body></html>`)
		case "/category.php":
			pages := cats[r.URL.Query().Get("id")]
			page, _ := strconv.Atoi(r.URL.Query().Get("page"))
			if page < 1 || page > len(pages) {
				page = 1 // clamp, as the live tour does
			}
			_, _ = fmt.Fprint(w, "<html><body>"+strings.Join(pages[page-1], "")+"</body></html>")
		case "/trailer.php":
			id := r.URL.Query().Get("id")
			_, _ = fmt.Fprint(w, detailPage(id, "SCENE "+id, "A description.",
				[]string{"big dick", "rough"}, []string{"Blake 4", "Parker"}))
		case "/sets.php":
			_, _ = fmt.Fprint(w, "<html><body>"+card("455", "CAMPUS COCKS", "Sep 10, 2025")+"</body></html>")
		default:
			http.NotFound(w, r)
		}
	}))
	return site
}

func newTestScraper(srv *httptest.Server) *Scraper {
	s := New()
	s.Client = srv.Client()
	s.baseOverride = srv.URL
	return s
}

func collect(t *testing.T, s *Scraper, studioURL string, opts scraper.ListOpts) ([]models.Scene, []error, int, bool) {
	t.Helper()
	ch, err := s.ListScenes(context.Background(), studioURL, opts)
	if err != nil {
		t.Fatalf("ListScenes: %v", err)
	}
	var scenes []models.Scene
	var errs []error
	total := 0
	stopped := false
	for res := range ch {
		switch res.Kind {
		case scraper.KindScene:
			scenes = append(scenes, res.Scene)
		case scraper.KindError:
			errs = append(errs, res.Err)
		case scraper.KindTotal:
			total = res.Total
		case scraper.KindStoppedEarly:
			stopped = true
		}
	}
	return scenes, errs, total, stopped
}

// There is no all-videos listing, so the catalogue is the union of every
// category the home page links — deduplicated, since categories overlap.
func TestBareHostSweepsEveryCategoryAndDedupes(t *testing.T) {
	site := newSite(t)
	defer site.Close()

	s := newTestScraper(site.Server)
	scenes, errs, total, _ := collect(t, s, "https://fratx.com/", scraper.ListOpts{Workers: 2})
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(scenes) != 4 || total != 4 {
		t.Fatalf("got %d scenes (total %d), want 4", len(scenes), total)
	}
	seen := map[string]bool{}
	for _, sc := range scenes {
		if seen[sc.ID] {
			t.Errorf("scene %s emitted twice", sc.ID)
		}
		seen[sc.ID] = true
	}
	if site.count("/trailer.php?id=452") != 1 {
		t.Errorf("the overlapping scene was fetched %d times, want 1", site.count("/trailer.php?id=452"))
	}
}

// The sweep visits categories in home-page order, which is no order at all as
// far as publication goes, so the union is sorted before the KnownIDs stop.
func TestUnionIsSortedNewestFirst(t *testing.T) {
	site := newSite(t)
	defer site.Close()

	s := newTestScraper(site.Server)
	scenes, _, _, _ := collect(t, s, "https://fratx.com/", scraper.ListOpts{Workers: 1})

	var dates []time.Time
	for _, sc := range scenes {
		dates = append(dates, sc.Date)
	}
	for i := 1; i < len(dates); i++ {
		if dates[i].After(dates[i-1]) {
			t.Fatalf("scene %d (%v) is newer than the one before it (%v)", i, dates[i], dates[i-1])
		}
	}
	if scenes[0].ID != "460" {
		t.Errorf("first scene = %s, want the newest (460)", scenes[0].ID)
	}
}

func TestKnownIDStopsAfterSorting(t *testing.T) {
	site := newSite(t)
	defer site.Close()

	s := newTestScraper(site.Server)
	scenes, _, _, stopped := collect(t, s, "https://fratx.com/",
		scraper.ListOpts{Workers: 1, KnownIDs: map[string]bool{"452": true}})

	if !stopped {
		t.Error("StoppedEarly was not reported")
	}
	// 460 (Oct 2025) and 455 (Sep 2025) precede 452 (Jul 2025).
	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want the 2 newer than the known one", len(scenes))
	}
}

// The detail page carries a "Related Videos" grid whose cards would otherwise
// supply a title and a date, and a footer full of anchors that would become
// performers. Everything is scoped to the info block.
func TestRelatedGridAndFooterDoNotLeakIntoTheScene(t *testing.T) {
	d := parseDetail(detailPage("455", "CAMPUS COCKS", "It's known around campus.",
		[]string{"big dick", "rough"}, []string{"Blake 4", "Parker"}))

	if d.title != "CAMPUS COCKS" {
		t.Errorf("title = %q", d.title)
	}
	if strings.Join(d.performers, ",") != "Blake 4,Parker" {
		t.Errorf("performers = %v", d.performers)
	}
	if strings.Join(d.tags, ",") != "big dick,rough" {
		t.Errorf("tags = %v", d.tags)
	}
	// The description opens with the publication date, which becomes the date
	// and is trimmed off the prose.
	if d.description != "It's known around campus." {
		t.Errorf("description = %q — the date prefix should be stripped", d.description)
	}
	if d.date.Format("2006-01-02") != "2025-09-10" {
		t.Errorf("date = %v, want the one in the description", d.date)
	}
	if !strings.Contains(d.poster, "poster/455") {
		t.Errorf("poster = %q", d.poster)
	}
}

// A category URL scrapes that slice alone.
func TestCategoryURLScrapesOnlyThatCategory(t *testing.T) {
	site := newSite(t)
	defer site.Close()

	s := newTestScraper(site.Server)
	scenes, errs, _, _ := collect(t, s, "https://fratx.com/category.php?id=12", scraper.ListOpts{Workers: 1})
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2", len(scenes))
	}
	if site.count("/?") != 0 {
		t.Error("a category URL read the home page to enumerate categories")
	}
}

// A single scene URL skips discovery.
func TestSingleSceneURLSkipsDiscovery(t *testing.T) {
	site := newSite(t)
	defer site.Close()

	s := newTestScraper(site.Server)
	scenes, errs, total, _ := collect(t, s, "https://fratx.com/trailer.php?id=455", scraper.ListOpts{Workers: 1})
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(scenes) != 1 || total != 1 {
		t.Fatalf("got %d scenes (total %d), want 1", len(scenes), total)
	}
	if scenes[0].ID != "455" || scenes[0].Title != "SCENE 455" {
		t.Errorf("scene = %+v", scenes[0])
	}
	if site.count("/category.php?id=9&page=1") != 0 {
		t.Error("a single-scene URL swept the categories")
	}
}

// A home page with no categories is a template change, not an empty site.
func TestHomePageWithNoCategoriesIsAParseError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, "<html><body>nothing here</body></html>")
	}))
	defer srv.Close()

	s := newTestScraper(srv)
	scenes, errs, _, _ := collect(t, s, "https://fratx.com/", scraper.ListOpts{Workers: 1})
	if len(scenes) != 0 {
		t.Fatalf("got %d scenes", len(scenes))
	}
	if len(errs) == 0 {
		t.Fatal("a home page with no categories reported no error")
	}
	if k := scraper.Classify(errs[0]); k != scraper.FailureParse {
		t.Errorf("classified as %v, want FailureParse", k)
	}
}

// One unreachable category must not end the sweep: bailing there would hand an
// authoritative --full save a catalogue missing everything after it.
func TestSweepContinuesPastAFailedCategory(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/":
			_, _ = fmt.Fprint(w, `<html><body><a href="/category.php?id=9">a</a><a href="/category.php?id=12">b</a></body></html>`)
		case r.URL.Path == "/category.php" && r.URL.Query().Get("id") == "9":
			http.Error(w, "boom", http.StatusInternalServerError)
		case r.URL.Path == "/category.php":
			// Every page serves the same card, so the walk ends on the second
			// fetch when it adds nothing new.
			_, _ = fmt.Fprint(w, "<html><body>"+card("460", "SURVIVOR", "Oct 22, 2025")+"</body></html>")
		default:
			_, _ = fmt.Fprint(w, detailPage("460", "SURVIVOR", "d", nil, nil))
		}
	}))
	defer srv.Close()

	s := newTestScraper(srv)
	scenes, errs, _, _ := collect(t, s, "https://fratx.com/", scraper.ListOpts{Workers: 1})
	if len(errs) == 0 {
		t.Error("the failed category was not reported")
	}
	if len(scenes) != 1 || scenes[0].ID != "460" {
		t.Errorf("got %v, want the surviving category's scene", scenes)
	}
}

func TestMatchesURL(t *testing.T) {
	s := New()
	for _, u := range []string{
		"https://fratx.com",
		"https://www.fratx.com/",
		"http://fratx.com/trailer.php?id=455",
		"https://fratx.com/category.php?id=9",
	} {
		if !s.MatchesURL(u) {
			t.Errorf("MatchesURL(%q) = false, want true", u)
		}
	}
	for _, u := range []string{"https://fratxxx.com/", "https://example.com/fratx.com"} {
		if s.MatchesURL(u) {
			t.Errorf("MatchesURL(%q) = true, want false", u)
		}
	}
}

func TestScenesValidate(t *testing.T) {
	site := newSite(t)
	defer site.Close()

	s := newTestScraper(site.Server)
	scenes, _, _, _ := collect(t, s, "https://fratx.com/", scraper.ListOpts{Workers: 2})
	for _, sc := range scenes {
		testutil.ValidateScene(t, sc)
	}
}

func TestContextCancellationStopsTheWalk(t *testing.T) {
	site := newSite(t)
	defer site.Close()

	s := newTestScraper(site.Server)
	ctx, cancel := context.WithCancel(context.Background())
	ch, err := s.ListScenes(ctx, "https://fratx.com/", scraper.ListOpts{Workers: 2, Delay: 10 * time.Millisecond})
	if err != nil {
		t.Fatalf("ListScenes: %v", err)
	}
	cancel()
	for range ch { //nolint:revive // drain so the goroutines can finish their sends
	}
}
