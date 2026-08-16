package hungyoungbrit

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

// card reproduces one listing tile. The title attribute double-encodes emoji
// (`&amp;#x1F608;`), which one unescape pass leaves as a numeric entity.
func card(id int, slug, title string) string {
	return fmt.Sprintf(`<div class="col-sm-6 col-md-6 video-thumb" data-setid="%d">
  <div class="show show-first">
    <a title="%s" href="https://www.hungyoungbrit.com/tour/updates/%s.html">
      <img id="set-target-%d" alt="%s"
        src="https://cdn.example/tour/content/contentthumbs/43/73/%d-1x.jpg?expires=1787064583&amp;token=abc"
        src0_1x="https://cdn.example/tour/content/contentthumbs/43/73/%d-1x.jpg?expires=1787064583&amp;token=abc" />
      <h3 class="scene-title">%s</h3>
    </a>
  </div>
</div>`, id, title, slug, id, title, id, id, title)
}

// detailPage carries the live panel AND a commented-out second copy of it —
// the tour really ships both, and the commented one holds a *fuller* synopsis
// plus its own title, models and date. A parser that reads comments picks up
// markup that is not on the page.
func detailPage(title, desc string, models []string, minutes int, date string) string {
	var live, dead strings.Builder
	for _, m := range models {
		fmt.Fprintf(&live, `<a href="https://join.example/signup">%s</a> / `, m)
		fmt.Fprintf(&dead, `<a href="https://join.example/signup">GHOST %s</a> / `, m)
	}
	return fmt.Sprintf(`<html><head>
<meta property="og:title" content="%s - HungYoungBrit.com"/>
<meta property="og:description" content="%s"/>
</head><body>
<!-- <span class="update_title">COMMENTED TITLE</span>
     <span class="tour_update_models">%s</span>
     <span class="update_date">01/01/1999</span>
     <span class="latest_update_description">A much fuller synopsis that is not really on the page.</span> -->
<img src="https://cdn.example/tour/content/HYB260612/14.jpg?expires=1&amp;token=b" />
<div class="panel panel-info">
  <div class="panel-heading"><h3 class='titleHYB'> %s </h3></div>
  <div class="panel-body">
    <span class="update_models">%s</span>
    <hr><h4>Scene Length: %d&nbsp;minute(s) </h4>
    <hr><h4>Release Date: %s</h4>
    <hr><h4> RATING: 5.0 </h4>
  </div>
</div>
</body></html>`, title, desc, dead.String(), title, live.String(), minutes, date)
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

// newSite serves `pages` listing pages of `per` cards, then a card-free page —
// which is what the live tour does past the last page.
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
				_, _ = fmt.Fprint(w, `<html><body><div class="row"></div></body></html>`)
				return
			}
			var sb strings.Builder
			sb.WriteString(`<html><body>`)
			for i := 0; i < per; i++ {
				id := 400 - ((page-1)*per + i)
				sb.WriteString(card(id, fmt.Sprintf("scene-%d", id), fmt.Sprintf("Scene %d &amp;#x1F608;", id)))
			}
			sb.WriteString(`</body></html>`)
			_, _ = fmt.Fprint(w, sb.String())
		case strings.HasPrefix(r.URL.Path, "/tour/updates/"):
			slug := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/tour/updates/"), ".html")
			_, _ = fmt.Fprint(w, detailPage("Detail "+slug, "A truncated description...",
				[]string{"Alexis Scott", "Kian Roberts"}, 16, "2026-08-11"))
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
	scenes, errs, total, _ := collect(t, s, "https://www.hungyoungbrit.com/tour/", scraper.ListOpts{Workers: 2})
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

// The tour ships a commented-out copy of the whole scene panel, with a fuller
// synopsis and different values. None of it is really on the page.
func TestCommentedPanelIsNotRead(t *testing.T) {
	d := parseDetail(detailPage("Real Title", "The truncated description...",
		[]string{"Alexis Scott", "Kian Roberts"}, 16, "2026-08-11"))

	if d.title != "Real Title" {
		t.Errorf("title = %q", d.title)
	}
	if d.description != "The truncated description..." {
		t.Errorf("description = %q — the commented synopsis is not on the page", d.description)
	}
	for _, p := range d.performers {
		if strings.HasPrefix(p, "GHOST") {
			t.Errorf("a commented-out credit was stored: %q", p)
		}
	}
	if strings.Join(d.performers, ",") != "Alexis Scott,Kian Roberts" {
		t.Errorf("performers = %v", d.performers)
	}
	if d.duration != 16*60 {
		t.Errorf("duration = %d", d.duration)
	}
	if d.date.Format("2006-01-02") != "2026-08-11" {
		t.Errorf("date = %v — the commented 01/01/1999 may have won", d.date)
	}
}

// Titles double-encode emoji, so one unescape pass leaves `&#x1F608;` behind.
func TestDoubleEncodedEntitiesAreFullyDecoded(t *testing.T) {
	got := cleanText("Benny Takes it Deep &amp;#x1F608; Rough")
	if strings.Contains(got, "&#x") || strings.Contains(got, "&amp;") {
		t.Errorf("cleanText left an entity behind: %q", got)
	}
}

// The listing runs newest-first — set ids descend across pages — so a stored id
// ends the walk.
func TestKnownIDStopsTheWalkEarly(t *testing.T) {
	site := newSite(t, 3, 4)
	defer site.Close()

	s := newTestScraper(site.Server)
	scenes, _, _, stopped := collect(t, s, "https://www.hungyoungbrit.com/tour/",
		scraper.ListOpts{Workers: 1, KnownIDs: map[string]bool{"398": true}})

	if !stopped {
		t.Error("StoppedEarly was not reported")
	}
	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want the 2 ahead of the known id", len(scenes))
	}
	if site.count("/tour/categories/movies_2_d.html") != 0 {
		t.Error("the walk continued past the known id")
	}
}

// A single scene URL skips the listing, and takes its id from the page's own
// content path since there is no card to supply one.
func TestSingleSceneURLSkipsTheListing(t *testing.T) {
	site := newSite(t, 3, 4)
	defer site.Close()

	s := newTestScraper(site.Server)
	scenes, errs, total, _ := collect(t, s,
		"https://www.hungyoungbrit.com/tour/updates/scene-400.html", scraper.ListOpts{Workers: 1})
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(scenes) != 1 || total != 1 {
		t.Fatalf("got %d scenes (total %d), want 1", len(scenes), total)
	}
	if scenes[0].ID == "" {
		t.Error("the scene has no id")
	}
	if site.count("/tour/categories/movies_1_d.html") != 0 {
		t.Error("a single-scene URL walked the listing")
	}
}

// A listing that fetched cleanly and parsed to nothing is a template change,
// which must not read as an empty catalogue to an authoritative --full save.
func TestEmptyFirstListingPageIsAParseError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, "<html><body>nothing here</body></html>")
	}))
	defer srv.Close()

	s := newTestScraper(srv)
	scenes, errs, _, _ := collect(t, s, "https://www.hungyoungbrit.com/tour/", scraper.ListOpts{Workers: 1})
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

// A failed detail costs the cast, date and runtime, not the scene — the card
// already carries id, title and thumbnail.
func TestDetailFailureKeepsTheCardOnlyScene(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/tour/updates/") {
			http.Error(w, "gone", http.StatusInternalServerError)
			return
		}
		if strings.HasSuffix(r.URL.Path, "movies_1_d.html") {
			_, _ = fmt.Fprint(w, `<html><body>`+card(400, "scene-400", "Card Title")+`</body></html>`)
			return
		}
		_, _ = fmt.Fprint(w, `<html><body></body></html>`)
	}))
	defer srv.Close()

	s := newTestScraper(srv)
	scenes, errs, _, _ := collect(t, s, "https://www.hungyoungbrit.com/tour/", scraper.ListOpts{Workers: 1})
	if len(scenes) != 1 {
		t.Fatalf("got %d scenes, want the card-only scene", len(scenes))
	}
	if scenes[0].ID != "400" || scenes[0].Title != "Card Title" {
		t.Errorf("card fields lost: %+v", scenes[0])
	}
	if len(errs) == 0 {
		t.Error("the failed detail fetch was not reported")
	}
}

func TestMatchesURL(t *testing.T) {
	s := New()
	for _, u := range []string{
		"https://hungyoungbrit.com",
		"https://www.hungyoungbrit.com/tour/",
		"http://www.hungyoungbrit.com/tour/updates/some-scene.html",
	} {
		if !s.MatchesURL(u) {
			t.Errorf("MatchesURL(%q) = false, want true", u)
		}
	}
	for _, u := range []string{
		"https://join.hungyoungbrit.com/signup/signup.php",
		"https://hungyoungbritfan.com/",
	} {
		if s.MatchesURL(u) {
			t.Errorf("MatchesURL(%q) = true, want false", u)
		}
	}
}

func TestScenesValidate(t *testing.T) {
	site := newSite(t, 1, 3)
	defer site.Close()

	s := newTestScraper(site.Server)
	scenes, _, _, _ := collect(t, s, "https://www.hungyoungbrit.com/tour/", scraper.ListOpts{Workers: 2})
	for _, sc := range scenes {
		testutil.ValidateScene(t, sc)
	}
}

func TestContextCancellationStopsTheWalk(t *testing.T) {
	site := newSite(t, 40, 10)
	defer site.Close()

	s := newTestScraper(site.Server)
	ctx, cancel := context.WithCancel(context.Background())
	ch, err := s.ListScenes(ctx, "https://www.hungyoungbrit.com/tour/", scraper.ListOpts{Workers: 3, Delay: 10 * time.Millisecond})
	if err != nil {
		t.Fatalf("ListScenes: %v", err)
	}
	cancel()
	for range ch { //nolint:revive // drain so the goroutines can finish their sends
	}
}
