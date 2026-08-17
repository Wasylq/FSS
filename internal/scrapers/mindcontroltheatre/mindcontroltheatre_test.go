package mindcontroltheatre

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
	"github.com/Anastylosis/FSS/models"
	"github.com/Anastylosis/FSS/scraper"
)

// moviePage reproduces a movie, including the three per-format prices and the
// "Other customers enjoyed…" rail of other movies that follows it.
func moviePage(slug, title string, cast []string, desc, data string) string {
	var castHTML strings.Builder
	for i, c := range cast {
		if i > 0 {
			castHTML.WriteString(" and ")
		}
		fmt.Fprintf(&castHTML, `<a href="/performer/%s">%s</a>`, strings.ToLower(strings.ReplaceAll(c, " ", "-")), c)
	}
	return fmt.Sprintf(`<html><body><div id="page_content">
<h1>%s </h1>
<div class="buybox"><h3>Buy Now</h3><ul>
  <li><a class="add_product" href="/add-product/aaa">%s (4K): $34.99</a></li>
  <li><a class="add_product" href="/add-product/bbb">%s (HD): $32.99</a></li>
  <li><a class="add_product" href="/add-product/ccc">%s (DVD): $40.00</a></li>
</ul></div>
<div id="images"><a href="/movie-image/large/%s/1.jpg"><img src="/movie-image/small/%s/1.jpg"></a></div>
<div id="cast"> starring %s. </div>
<video controls poster="https://mindcontroltheatre.com/movie-image/large/%s/0.jpg">
  <source type="video/mp4" src="https://trailers.mindcontroltheatre.com/mct/trailers/%s-trailer.mp4"/>
</video>
<div id="description">
  <p>%s</p>
  <p id="data">%s</p>
</div>
<h2>Other customers enjoyed...</h2>
<div class="moviebox"><a href="/movie/another-one"><img src="/movie-image/small/another-one/0.jpg"/><br/>Another One</a></div>
</div></body></html>`, title, title, title, title, slug, slug, castHTML.String(), slug, slug, desc, data)
}

const ageGate = `<html><head><meta http-equiv="refresh" content="0;url=/age?r=/"></head><body>Age check</body></html>`

func sitemapXML(base string, paths ...string) string {
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?><urlset>`)
	for _, p := range paths {
		fmt.Fprintf(&sb, "<url><loc>%s%s</loc><lastmod>2026-08-09</lastmod></url>", base, p)
	}
	sb.WriteString(`</urlset>`)
	return sb.String()
}

type stubSite struct {
	*httptest.Server
	mu       sync.Mutex
	hits     map[string]int
	gatehits int
}

func (s *stubSite) hit(p string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hits[p]++
}

func (s *stubSite) gate() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.gatehits++
}

func (s *stubSite) gated() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.gatehits
}

func (s *stubSite) count(p string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.hits[p]
}

const liveHost = "https://mindcontroltheatre.com"

// newSite refuses every request that arrives without the age cookie, which is
// what the live site does.
func newSite(t *testing.T) *stubSite {
	t.Helper()
	site := &stubSite{hits: map[string]int{}}
	site.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		site.hit(r.URL.Path)
		if c, err := r.Cookie("age"); err != nil || c.Value != "yes" {
			site.gate()
			_, _ = fmt.Fprint(w, ageGate)
			return
		}
		switch {
		case r.URL.Path == "/sitemap.xml":
			_, _ = fmt.Fprint(w, sitemapXML(liveHost,
				"/movie/ccvr-diagnostic-testing", "/movie/alison-interviewed",
				// Browse pages and assets share the URL space.
				"/movies", "/movies/page-2", "/info/2257",
			))
		case strings.HasPrefix(r.URL.Path, "/movie/"):
			slug := strings.TrimPrefix(r.URL.Path, "/movie/")
			_, _ = fmt.Fprint(w, moviePage(slug, "Codi Vore: Diagnostic Testing",
				[]string{"Alison Rey", "Codi Vore"}, "A synopsis.",
				"9 August 2026 • 3840x 2160 • 30 minutes • hd: 862.1MB • 4k: 3.3GB"))
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

func collect(t *testing.T, s *Scraper, studioURL string, opts scraper.ListOpts) ([]models.Scene, []error, int) {
	t.Helper()
	ch, err := s.ListScenes(context.Background(), studioURL, opts)
	if err != nil {
		t.Fatalf("ListScenes: %v", err)
	}
	var scenes []models.Scene
	var errs []error
	total := 0
	for res := range ch {
		switch res.Kind {
		case scraper.KindScene:
			scenes = append(scenes, res.Scene)
		case scraper.KindError:
			errs = append(errs, res.Err)
		case scraper.KindTotal:
			total = res.Total
		}
	}
	return scenes, errs, total
}

// Every page is behind an age interstitial; the cookie is sent on every request
// rather than the gate being followed.
func TestAgeCookieIsSentOnEveryRequest(t *testing.T) {
	site := newSite(t)
	defer site.Close()

	s := newTestScraper(site.Server)
	scenes, errs, total := collect(t, s, "https://mindcontroltheatre.com/", scraper.ListOpts{Workers: 2})
	if site.gated() != 0 {
		t.Errorf("%d requests hit the age gate", site.gated())
	}
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(scenes) != 2 || total != 2 {
		t.Fatalf("got %d scenes (total %d), want 2", len(scenes), total)
	}
}

// `/movies` is a browse page and `/info/…` a site page; only `/movie/{slug}`
// is a scene.
func TestSitemapSelectsOnlyMoviePages(t *testing.T) {
	site := newSite(t)
	defer site.Close()

	s := newTestScraper(site.Server)
	collect(t, s, "https://mindcontroltheatre.com/", scraper.ListOpts{Workers: 2})
	if site.count("/movies") != 0 || site.count("/info/2257") != 0 {
		t.Error("a browse or info page was fetched as a scene")
	}
}

func TestMovieFieldsComeFromThePage(t *testing.T) {
	site := newSite(t)
	defer site.Close()

	s := newTestScraper(site.Server)
	scenes, _, _ := collect(t, s, "https://mindcontroltheatre.com/movie/ccvr-diagnostic-testing",
		scraper.ListOpts{Workers: 1})
	if len(scenes) != 1 {
		t.Fatalf("got %d scenes", len(scenes))
	}
	got := scenes[0]

	if got.ID != "ccvr-diagnostic-testing" {
		t.Errorf("ID = %q", got.ID)
	}
	if got.Title != "Codi Vore: Diagnostic Testing" {
		t.Errorf("Title = %q", got.Title)
	}
	if strings.Join(got.Performers, ",") != "Alison Rey,Codi Vore" {
		t.Errorf("Performers = %v", got.Performers)
	}
	if got.Date.Format("2006-01-02") != "2026-08-09" {
		t.Errorf("Date = %v", got.Date)
	}
	if got.Duration != 30*60 {
		t.Errorf("Duration = %d", got.Duration)
	}
	if got.Resolution != "3840x2160" {
		t.Errorf("Resolution = %q", got.Resolution)
	}
	// The synopsis is the description block's prose, not its data line.
	if got.Description != "A synopsis." {
		t.Errorf("Description = %q", got.Description)
	}
	// Three per-format prices; the cheapest is what the scene costs.
	if len(got.PriceHistory) != 1 || got.PriceHistory[0].Regular != 32.99 {
		t.Errorf("PriceHistory = %v, want one snapshot at 32.99", got.PriceHistory)
	}
	if !strings.Contains(got.Preview, "-trailer.mp4") {
		t.Errorf("Preview = %q", got.Preview)
	}
}

// The "Other customers enjoyed…" rail links other movies; nothing may take one
// for this scene.
func TestRecommendationRailDoesNotBecomeTheScene(t *testing.T) {
	d := parseMovie(moviePage("ccvr-diagnostic-testing", "Real Title",
		[]string{"Alison Rey"}, "A synopsis.", "9 August 2026 • 1920x 1080 • 25 minutes"))
	if d.title != "Real Title" {
		t.Errorf("title = %q", d.title)
	}
	for _, p := range d.performers {
		if p == "Another One" {
			t.Error("a rail entry became a performer")
		}
	}
}

func TestScenePath(t *testing.T) {
	cases := map[string]string{
		"https://mindcontroltheatre.com/movie/a-slug":  "/movie/a-slug",
		"https://mindcontroltheatre.com/movie/a-slug/": "/movie/a-slug",
		"https://mindcontroltheatre.com/movies":        "",
		"https://mindcontroltheatre.com/movies/page-2": "",
		"https://mindcontroltheatre.com/info/2257":     "",
		"https://mindcontroltheatre.com/":              "",
	}
	for in, want := range cases {
		got, ok := scenePath(in)
		if want == "" {
			if ok {
				t.Errorf("scenePath(%q) = %q, want not-a-scene", in, got)
			}
			continue
		}
		if !ok || got != want {
			t.Errorf("scenePath(%q) = %q,%v; want %q,true", in, got, ok, want)
		}
	}
}

func TestSitemapWithNoMoviesIsAParseError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, sitemapXML(liveHost, "/movies", "/info/2257"))
	}))
	defer srv.Close()

	s := newTestScraper(srv)
	scenes, errs, _ := collect(t, s, "https://mindcontroltheatre.com/", scraper.ListOpts{Workers: 1})
	if len(scenes) != 0 {
		t.Fatalf("got %d scenes", len(scenes))
	}
	if len(errs) == 0 {
		t.Fatal("a sitemap naming no movies reported no error")
	}
	if k := scraper.Classify(errs[0]); k != scraper.FailureParse {
		t.Errorf("classified as %v, want FailureParse", k)
	}
}

func TestMatchesURL(t *testing.T) {
	s := New()
	for _, u := range []string{
		"https://mindcontroltheatre.com",
		"https://www.mindcontroltheatre.com/",
		"http://mindcontroltheatre.com/movie/a-slug",
	} {
		if !s.MatchesURL(u) {
			t.Errorf("MatchesURL(%q) = false, want true", u)
		}
	}
	for _, u := range []string{"https://daphnesfantasies.com/", "https://example.com/mindcontroltheatre.com"} {
		if s.MatchesURL(u) {
			t.Errorf("MatchesURL(%q) = true, want false", u)
		}
	}
}

func TestScenesValidate(t *testing.T) {
	site := newSite(t)
	defer site.Close()

	s := newTestScraper(site.Server)
	scenes, _, _ := collect(t, s, "https://mindcontroltheatre.com/", scraper.ListOpts{Workers: 2})
	for _, sc := range scenes {
		testutil.ValidateScene(t, sc)
	}
}

func TestContextCancellationStopsTheWalk(t *testing.T) {
	site := newSite(t)
	defer site.Close()

	s := newTestScraper(site.Server)
	ctx, cancel := context.WithCancel(context.Background())
	ch, err := s.ListScenes(ctx, "https://mindcontroltheatre.com/", scraper.ListOpts{Workers: 2, Delay: 10 * time.Millisecond})
	if err != nil {
		t.Fatalf("ListScenes: %v", err)
	}
	cancel()
	for range ch { //nolint:revive // drain so the goroutines can finish their sends
	}
}
