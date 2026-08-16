package bbwhighway

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

// card reproduces one listing tile. The anchor text is truncated the way the
// live tour truncates it — the full title lives only in the `title` attribute —
// and the card names no performers at all.
func card(id int, slug, title string, date string) string {
	return fmt.Sprintf(`<div class="modelfeature grabthis">
  <div class="modelimg">
    <a href="https://bbwhighway.com/tour/trailers/%s.html" title="%s">
      <img id="set-target-%d-5634481" class="update_thumb thumbs stdimage"
        src0_1x="/tour/content//contentthumbs/83/56/%d-1x.jpg" cnt="6" v="0" />
    </a>
  </div>
  <div class="modeldetails clear"><div class="modeldata">
    <h3><a href="https://bbwhighway.com/tour/trailers/%s.html" title="%s">%s...</a></h3>
    <p>25:20 | %s</p>
  </div></div>
</div>`, slug, title, id, id, slug, title, title[:10], date)
}

func detailPage(id int, title string, performers []string) string {
	var models strings.Builder
	for _, p := range performers {
		fmt.Fprintf(&models, `<a href="https://bbwhighway.com/tour/models/%s.html">%s</a>, `,
			strings.ReplaceAll(p, " ", ""), p)
	}
	return fmt.Sprintf(`<html><body>
<div class="pagetitle"><div class="centerwrap clear"><h1>%s</h1></div></div>
<div class="indvideo">
  <div class="videoplayer"><img id="set-target-%d-8104780" class="update_thumb thumbs stdimage" /></div>
  <div class="videocontent"><p>%s</p></div>
  <div class="videodetails">
    <p class="date">08/17/2026 | 25:20 | Categories: </p>
    <p class="modelname">Starring <span class="tour_update_models"> %s </span></p>
  </div>
  <div class="videodetails"><p>(Avg Rating: 5)</p></div>
</div></body></html>`, title, id, title, models.String())
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

func newSite(t *testing.T, pages, per int) *stubSite {
	t.Helper()
	site := &stubSite{hits: map[string]int{}}
	site.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		site.hit(r.URL.Path)
		switch {
		case strings.HasPrefix(r.URL.Path, "/tour/categories/movies_"):
			n := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/tour/categories/movies_"), "_d.html")
			page, _ := strconv.Atoi(n)
			if page < 1 || page > pages {
				_, _ = fmt.Fprint(w, `<html><body><div class="centerwrap"></div></body></html>`)
				return
			}
			var sb strings.Builder
			sb.WriteString(`<html><body>`)
			for i := 0; i < per; i++ {
				id := 1142 - ((page-1)*per + i)
				sb.WriteString(card(id, fmt.Sprintf("scene-%d", id), fmt.Sprintf("A FULL TITLE FOR %d", id), "2026-08-17"))
			}
			sb.WriteString(`</body></html>`)
			_, _ = fmt.Fprint(w, sb.String())
		case strings.HasPrefix(r.URL.Path, "/tour/trailers/"):
			slug := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/tour/trailers/"), ".html")
			id, _ := strconv.Atoi(strings.TrimPrefix(slug, "scene-"))
			_, _ = fmt.Fprint(w, detailPage(id, "Detail Title "+slug,
				[]string{"Don XXX Prince", "PRINCESS CUPSSSS"}))
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

func TestWalksEveryListingPage(t *testing.T) {
	site := newSite(t, 3, 4)
	defer site.Close()

	s := newTestScraper(site.Server)
	scenes, errs, total, _ := collect(t, s, "https://bbwhighway.com/tour/", scraper.ListOpts{Workers: 2})
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(scenes) != 12 || total != 12 {
		t.Fatalf("got %d scenes (total %d), want 12", len(scenes), total)
	}
	if site.count("/tour/categories/movies_4_d.html") != 1 {
		t.Error("the card-free page that ends the walk was not reached exactly once")
	}
}

// The card's anchor text is truncated and it names nobody, so the detail page
// is what supplies the cast — and the full title comes from the card's `title`
// attribute rather than its visible text.
func TestCardTitleAttributeAndDetailCast(t *testing.T) {
	cards := parseCards(card(1142, "scene-1142", "A FULL TITLE FOR 1142", "2026-08-17"))
	if len(cards) != 1 {
		t.Fatalf("got %d cards, want 1", len(cards))
	}
	if cards[0].title != "A FULL TITLE FOR 1142" {
		t.Errorf("card title = %q — the truncated anchor text should not win", cards[0].title)
	}
	if cards[0].id != "1142" {
		t.Errorf("card id = %q", cards[0].id)
	}
	if cards[0].duration != 25*60+20 {
		t.Errorf("card duration = %d", cards[0].duration)
	}
	if cards[0].date.Format("2006-01-02") != "2026-08-17" {
		t.Errorf("card date = %v", cards[0].date)
	}

	d := parseDetail(detailPage(1142, "Detail Title", []string{"Don XXX Prince", "PRINCESS CUPSSSS"}))
	if strings.Join(d.performers, ",") != "Don XXX Prince,PRINCESS CUPSSSS" {
		t.Errorf("performers = %v", d.performers)
	}
	if d.date.Format("2006-01-02") != "2026-08-17" {
		t.Errorf("detail date = %v", d.date)
	}
	if d.duration != 25*60+20 {
		t.Errorf("detail duration = %d", d.duration)
	}
	// The tour prints a "Categories:" label with nothing behind it on every
	// scene; an empty list is the right answer, not a parse failure.
	if len(d.categories) != 0 {
		t.Errorf("categories = %v, want none", d.categories)
	}
}

// The tour's "description" paragraph is the title repeated, so nothing is
// stored for it — a description identical to the title is noise downstream.
func TestDescriptionIsNotStored(t *testing.T) {
	site := newSite(t, 1, 1)
	defer site.Close()

	s := newTestScraper(site.Server)
	scenes, _, _, _ := collect(t, s, "https://bbwhighway.com/tour/", scraper.ListOpts{Workers: 1})
	if len(scenes) != 1 {
		t.Fatalf("got %d scenes", len(scenes))
	}
	if scenes[0].Description != "" {
		t.Errorf("Description = %q, want empty", scenes[0].Description)
	}
}

func TestKnownIDStopsTheWalkEarly(t *testing.T) {
	site := newSite(t, 3, 4)
	defer site.Close()

	s := newTestScraper(site.Server)
	scenes, _, _, stopped := collect(t, s, "https://bbwhighway.com/tour/",
		scraper.ListOpts{Workers: 1, KnownIDs: map[string]bool{"1140": true}})

	if !stopped {
		t.Error("StoppedEarly was not reported")
	}
	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want the 2 ahead of the known id", len(scenes))
	}
}

func TestSingleSceneURLSkipsTheListing(t *testing.T) {
	site := newSite(t, 3, 4)
	defer site.Close()

	s := newTestScraper(site.Server)
	scenes, errs, total, _ := collect(t, s,
		"https://bbwhighway.com/tour/trailers/scene-1142.html", scraper.ListOpts{Workers: 1})
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(scenes) != 1 || total != 1 {
		t.Fatalf("got %d scenes (total %d), want 1", len(scenes), total)
	}
	if scenes[0].ID != "1142" {
		t.Errorf("ID = %q — a single-scene run takes it from the page", scenes[0].ID)
	}
	if site.count("/tour/categories/movies_1_d.html") != 0 {
		t.Error("a single-scene URL walked the listing")
	}
}

func TestEmptyFirstListingPageIsAParseError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, "<html><body>nothing here</body></html>")
	}))
	defer srv.Close()

	s := newTestScraper(srv)
	scenes, errs, _, _ := collect(t, s, "https://bbwhighway.com/tour/", scraper.ListOpts{Workers: 1})
	if len(scenes) != 0 {
		t.Fatalf("got %d scenes", len(scenes))
	}
	if len(errs) == 0 {
		t.Fatal("an unparseable listing reported no error")
	}
	if k := scraper.Classify(errs[0]); k != scraper.FailureParse {
		t.Errorf("classified as %v, want FailureParse", k)
	}
}

func TestDetailFailureKeepsTheCardOnlyScene(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/tour/trailers/") {
			http.Error(w, "gone", http.StatusInternalServerError)
			return
		}
		if strings.HasSuffix(r.URL.Path, "movies_1_d.html") {
			_, _ = fmt.Fprint(w, `<html><body>`+card(1142, "scene-1142", "A FULL TITLE HERE", "2026-08-17")+`</body></html>`)
			return
		}
		_, _ = fmt.Fprint(w, `<html><body></body></html>`)
	}))
	defer srv.Close()

	s := newTestScraper(srv)
	scenes, errs, _, _ := collect(t, s, "https://bbwhighway.com/tour/", scraper.ListOpts{Workers: 1})
	if len(scenes) != 1 {
		t.Fatalf("got %d scenes, want the card-only scene", len(scenes))
	}
	got := scenes[0]
	if got.ID != "1142" || got.Title != "A FULL TITLE HERE" || got.Duration != 25*60+20 {
		t.Errorf("card fields lost: %+v", got)
	}
	if len(errs) == 0 {
		t.Error("the failed detail fetch was not reported")
	}
}

func TestMatchesURL(t *testing.T) {
	s := New()
	for _, u := range []string{
		"https://bbwhighway.com/",
		"https://www.bbwhighway.com/tour/",
		"http://bbwhighway.com/tour/trailers/some-scene.html",
	} {
		if !s.MatchesURL(u) {
			t.Errorf("MatchesURL(%q) = false, want true", u)
		}
	}
	for _, u := range []string{"https://bbwhighwayfan.com/", "https://example.com/bbwhighway.com"} {
		if s.MatchesURL(u) {
			t.Errorf("MatchesURL(%q) = true, want false", u)
		}
	}
}

func TestScenesValidate(t *testing.T) {
	site := newSite(t, 1, 3)
	defer site.Close()

	s := newTestScraper(site.Server)
	scenes, _, _, _ := collect(t, s, "https://bbwhighway.com/tour/", scraper.ListOpts{Workers: 2})
	for _, sc := range scenes {
		testutil.ValidateScene(t, sc)
	}
}

func TestContextCancellationStopsTheWalk(t *testing.T) {
	site := newSite(t, 40, 10)
	defer site.Close()

	s := newTestScraper(site.Server)
	ctx, cancel := context.WithCancel(context.Background())
	ch, err := s.ListScenes(ctx, "https://bbwhighway.com/tour/", scraper.ListOpts{Workers: 3, Delay: 10 * time.Millisecond})
	if err != nil {
		t.Fatalf("ListScenes: %v", err)
	}
	cancel()
	for range ch { //nolint:revive // drain so the goroutines can finish their sends
	}
}
