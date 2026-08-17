package tripforfuck

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

// card reproduces one listing tile. The poster is behind a lazyload
// placeholder, so the real path is on `_data-src` and `src` is an SVG.
func card(id, title string, daysAgo int) string {
	return fmt.Sprintf(`<div class="movie-list__item">
  <a href="/member/movie/%s/index.html" class="movie-list__image mb-2 d-block">
    <img src="/image/private/movie_detail/player-thumbnail.svg" class="lazyload" alt="%s"
      _data-src="/content/sample/movie/%s/thumb/poster.jpg"
      data-srcset="/content/sample/movie/%s/thumb/poster_w320.jpg 1x" />
  </a>
  <p class="mb-1 movie-list__title">
    <a href="/member/movie/%s/index.html" class="text-primary">%s</a>
  </p>
  <p class="mb-1 small movie-list__model"> </p>
  <p class="small text-muted">%d days ago</p>
</div>`, id, title, id, id, id, title, daysAgo)
}

// detailPage puts the cast in one paragraph after the h1, and surrounds it with
// the site navigation and a related-performers rail — both of which link
// `/member/actor/` too.
func detailPage(title, desc string, cast, tags []string, daysAgo int) string {
	var castHTML, tagHTML, railHTML strings.Builder
	for i, c := range cast {
		fmt.Fprintf(&castHTML, `<a class="text-primary" href="/member/actor/0023%d/index.html">%s</a> `, i, c)
	}
	for i, tg := range tags {
		fmt.Fprintf(&tagHTML, `<a href="/member/movie/list/index.html?id_tag=%d" class="btn btn-dark btn-sm">%s</a>`, i, tg)
	}
	for _, n := range []string{"Somebody Else", "Another Person"} {
		fmt.Fprintf(&railHTML, `<a href="/member/actor/99999/index.html">%s</a>`, n)
	}
	return fmt.Sprintf(`<html><body>
<nav><a href="/member/actor/list/index.html">Models</a></nav>
<div class="container">
  <h1 class="h3 mb-2">%s</h1>
  <div class="d-flex"><div>
    <p class="mb-0">%s</p>
    <div class="d-flex small movie-status"><p> %d days ago </p></div>
  </div></div>
  <p>%s</p>
  <div class="search-tags mb-5 d-flex flex-wrap">%s</div>
  <div class="related"><h2>More from these models</h2>%s</div>
</div></body></html>`, title, castHTML.String(), daysAgo, desc, tagHTML.String(), railHTML.String())
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
		case r.URL.Path == "/member/movie/list/index.html":
			page, _ := strconv.Atoi(r.URL.Query().Get("page"))
			if page < 1 {
				page = 1
			}
			if page > pages {
				_, _ = fmt.Fprint(w, `<html><body><div class="movie-list"></div></body></html>`)
				return
			}
			var sb strings.Builder
			sb.WriteString(`<html><body>`)
			for i := 0; i < per; i++ {
				n := 400 - ((page-1)*per + i)
				sb.WriteString(card(fmt.Sprintf("%d-1", n), fmt.Sprintf("Movie %d", n), 100+i))
			}
			sb.WriteString(`</body></html>`)
			_, _ = fmt.Fprint(w, sb.String())
		case strings.HasPrefix(r.URL.Path, "/member/movie/"):
			_, _ = fmt.Fprint(w, detailPage("Detail Title", "Line one.<br /> Line two.",
				[]string{"Xenia Virgin", "Shelena Ivanova"}, []string{"Creampie", "European"}, 490))
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
	scenes, errs, total, _ := collect(t, s, "https://www.tripforfuck.com/", scraper.ListOpts{Workers: 2})
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(scenes) != 12 || total != 12 {
		t.Fatalf("got %d scenes (total %d), want 12", len(scenes), total)
	}
}

// The site navigation and a related-performers rail both link `/member/actor/`,
// so matching those links across the page turned a two-woman scene into a dozen
// names. The cast is the one paragraph after the h1.
func TestOnlyTheCastParagraphSuppliesPerformers(t *testing.T) {
	d := parseDetail(detailPage("A Title", "A synopsis.",
		[]string{"Xenia Virgin", "Shelena Ivanova"}, []string{"Creampie"}, 490))

	if strings.Join(d.performers, ",") != "Xenia Virgin,Shelena Ivanova" {
		t.Errorf("performers = %v", d.performers)
	}
	for _, p := range d.performers {
		if p == "Models" || p == "Somebody Else" || p == "Another Person" {
			t.Errorf("a navigation or rail link became a performer: %q", p)
		}
	}
	if strings.Join(d.tags, ",") != "Creampie" {
		t.Errorf("tags = %v", d.tags)
	}
}

// Synopses are one paragraph of <br>-separated lines; the breaks must become
// spaces rather than running the sentences together.
func TestSynopsisLineBreaksBecomeSpaces(t *testing.T) {
	d := parseDetail(detailPage("T", "Line one.<br /> Line two.<br />Line three.", []string{"X"}, nil, 1))
	if d.description != "Line one. Line two. Line three." {
		t.Errorf("description = %q", d.description)
	}
}

// The site publishes no absolute date anywhere — both the card and the detail
// page say "490 days ago" — so it is subtracted from the scrape time. That is
// stable across runs but accurate only to the day, so the time is dropped.
func TestRelativeDateResolvesToAWholeDay(t *testing.T) {
	now := time.Date(2026, 8, 18, 13, 45, 30, 0, time.UTC)
	got := dateFromDaysAgo(now, 490)
	if want := time.Date(2025, 4, 15, 0, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Errorf("dateFromDaysAgo = %v, want %v", got, want)
	}
	if got.Hour() != 0 || got.Minute() != 0 {
		t.Errorf("time of day kept: %v", got)
	}
	// Advancing both the clock and the counter lands on the same date, which
	// is what makes the stored value stable from run to run.
	later := dateFromDaysAgo(now.AddDate(0, 0, 10), 500)
	if !later.Equal(got) {
		t.Errorf("the date moved between runs: %v then %v", got, later)
	}
}

// The poster is behind a lazyload placeholder; the SVG on `src` is not it.
func TestCardThumbnailComesFromTheLazyloadAttribute(t *testing.T) {
	cards := parseCards(card("368-1", "A Movie", 482))
	if len(cards) != 1 {
		t.Fatalf("got %d cards", len(cards))
	}
	if !strings.HasSuffix(cards[0].thumb, "/368-1/thumb/poster.jpg") {
		t.Errorf("thumb = %q", cards[0].thumb)
	}
	if strings.Contains(cards[0].thumb, ".svg") {
		t.Error("the placeholder SVG was stored as the thumbnail")
	}
	if cards[0].daysAgo != 482 {
		t.Errorf("daysAgo = %d", cards[0].daysAgo)
	}
}

func TestKnownIDStopsTheWalkEarly(t *testing.T) {
	site := newSite(t, 3, 4)
	defer site.Close()

	s := newTestScraper(site.Server)
	scenes, _, _, stopped := collect(t, s, "https://www.tripforfuck.com/",
		scraper.ListOpts{Workers: 1, KnownIDs: map[string]bool{"398-1": true}})

	if !stopped {
		t.Error("StoppedEarly was not reported")
	}
	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want the 2 ahead of the known id", len(scenes))
	}
}

func TestSingleMovieURLSkipsTheListing(t *testing.T) {
	site := newSite(t, 3, 4)
	defer site.Close()

	s := newTestScraper(site.Server)
	scenes, errs, total, _ := collect(t, s,
		"https://www.tripforfuck.com/member/movie/368-1/index.html", scraper.ListOpts{Workers: 1})
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(scenes) != 1 || total != 1 {
		t.Fatalf("got %d scenes (total %d), want 1", len(scenes), total)
	}
	if scenes[0].ID != "368-1" {
		t.Errorf("ID = %q", scenes[0].ID)
	}
	if site.count("/member/movie/list/index.html") != 0 {
		t.Error("a single-movie URL walked the listing")
	}
}

func TestEmptyFirstListingPageIsAParseError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, "<html><body>nothing here</body></html>")
	}))
	defer srv.Close()

	s := newTestScraper(srv)
	scenes, errs, _, _ := collect(t, s, "https://www.tripforfuck.com/", scraper.ListOpts{Workers: 1})
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

func TestMatchesURL(t *testing.T) {
	s := New()
	for _, u := range []string{
		"https://tripforfuck.com",
		"https://www.tripforfuck.com/",
		"http://www.tripforfuck.com/member/movie/368-1/index.html",
	} {
		if !s.MatchesURL(u) {
			t.Errorf("MatchesURL(%q) = false, want true", u)
		}
	}
	for _, u := range []string{"https://tripforfuckfan.com/", "https://example.com/tripforfuck.com"} {
		if s.MatchesURL(u) {
			t.Errorf("MatchesURL(%q) = true, want false", u)
		}
	}
}

func TestScenesValidate(t *testing.T) {
	site := newSite(t, 1, 3)
	defer site.Close()

	s := newTestScraper(site.Server)
	scenes, _, _, _ := collect(t, s, "https://www.tripforfuck.com/", scraper.ListOpts{Workers: 2})
	for _, sc := range scenes {
		testutil.ValidateScene(t, sc)
	}
}

func TestContextCancellationStopsTheWalk(t *testing.T) {
	site := newSite(t, 40, 10)
	defer site.Close()

	s := newTestScraper(site.Server)
	ctx, cancel := context.WithCancel(context.Background())
	ch, err := s.ListScenes(ctx, "https://www.tripforfuck.com/", scraper.ListOpts{Workers: 3, Delay: 10 * time.Millisecond})
	if err != nil {
		t.Fatalf("ListScenes: %v", err)
	}
	cancel()
	for range ch { //nolint:revive // drain so the goroutines can finish their sends
	}
}
