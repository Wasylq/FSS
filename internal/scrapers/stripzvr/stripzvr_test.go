package stripzvr

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

// scenePageHTML reproduces a Yoast `@graph`. The WebSite node repeats `name`
// and `description` for the whole site and comes after the WebPage node, so a
// parser that takes the last one it sees stores the site tagline as the scene.
func scenePageHTML(title, desc, thumb, date string) string {
	return fmt.Sprintf(`<html><head>
<meta property="og:title" content="OG %s" />
<meta property="og:description" content="OG description" />
<meta property="og:image" content="https://cdn.example/og.jpg" />
<script type="application/ld+json">
{"@context":"https://schema.org","@graph":[
 {"@type":"WebPage","name":%q,"description":%q,"thumbnailUrl":%q,"datePublished":%q},
 {"@type":"ImageObject"},
 {"@type":"WebSite","name":"StripzVR - The Sexiest Woman stripping naked in Virtual Reality",
  "description":"The Sexiest girls stripping naked in immersive Virtual Reality"}]}
</script></head><body></body></html>`, title, title, desc, thumb, date)
}

func sitemapXML(base string, paths ...string) string {
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?><urlset>`)
	for _, p := range paths {
		fmt.Fprintf(&sb, "<url><loc>%s%s</loc></url>", base, p)
	}
	sb.WriteString(`</urlset>`)
	return sb.String()
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

// liveHost is the host the sitemaps advertise. The scraper must re-base those
// absolute URLs onto whatever origin it is actually scraping, or an offline
// test quietly fetches production.
const liveHost = "https://www.stripzvr.com"

func newSite(t *testing.T) *stubSite {
	t.Helper()
	site := &stubSite{hits: map[string]int{}}
	site.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		site.hit(r.URL.Path)
		switch r.URL.Path {
		case "/page-sitemap.xml":
			_, _ = fmt.Fprint(w, sitemapXML(liveHost,
				"/", "/community/", "/p8/",
				"/tiny-tina/cum-back-for-you/",
				"/alisia/hot-pants/",
				"/members/bonus-updates/",
				"/members/lauren-brock/roll-your-weed-on-it/",
			))
		case "/page-sitemap2.xml":
			_, _ = fmt.Fprint(w, sitemapXML(liveHost,
				"/arina-shy/hot-pants/",
				// Repeated from the first sitemap: must not be scraped twice.
				"/tiny-tina/cum-back-for-you/",
			))
		default:
			segs := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
			if len(segs) != 2 {
				http.NotFound(w, r)
				return
			}
			_, _ = fmt.Fprint(w, scenePageHTML(
				"Hot Pants featuring "+slugToName(segs[0])+" @ StripzVR.com - StripzVR",
				"A scene description.",
				"https://cdn.example/"+segs[1]+".jpg",
				"2021-05-15T12:27:23+00:00"))
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

// Site pages, the /members/ mirrors and repeats across the two sitemaps all
// have to be filtered out; only `/{performer}/{scene}/` is a scene.
func TestSitemapWalkSelectsOnlySceneePages(t *testing.T) {
	site := newSite(t)
	defer site.Close()

	s := newTestScraper(site.Server)
	scenes, errs, total := collect(t, s, "https://www.stripzvr.com/", scraper.ListOpts{Workers: 2})
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(scenes) != 3 || total != 3 {
		t.Fatalf("got %d scenes (total %d), want 3", len(scenes), total)
	}
	if site.count("/community/") != 0 || site.count("/p8/") != 0 {
		t.Error("a one-segment site page was fetched as a scene")
	}
	if site.count("/members/lauren-brock/roll-your-weed-on-it/") != 0 {
		t.Error("the paywalled mirror was fetched; it duplicates a public scene")
	}
	if got := site.count("/tiny-tina/cum-back-for-you/"); got != 1 {
		t.Errorf("the repeated sitemap entry was fetched %d times, want 1", got)
	}
}

// The scene slug is not unique — the site reuses titles across performers — so
// the id has to carry the performer too or the two collapse into one scene.
func TestSceneIDIncludesThePerformerSlug(t *testing.T) {
	site := newSite(t)
	defer site.Close()

	s := newTestScraper(site.Server)
	scenes, _, _ := collect(t, s, "https://www.stripzvr.com/", scraper.ListOpts{Workers: 1})

	ids := map[string]bool{}
	for _, sc := range scenes {
		if ids[sc.ID] {
			t.Errorf("duplicate id %q", sc.ID)
		}
		ids[sc.ID] = true
	}
	for _, want := range []string{"alisia/hot-pants", "arina-shy/hot-pants", "tiny-tina/cum-back-for-you"} {
		if !ids[want] {
			t.Errorf("missing id %q; got %v", want, ids)
		}
	}
}

// The WebPage node is the scene's; the WebSite node that follows it describes
// the whole site and must not overwrite it.
func TestWebPageNodeWinsOverTheWebSiteNode(t *testing.T) {
	got := parsePage(scenePageHTML(
		"Cum Back For You featuring Tiny Tina @ StripzVR.com - StripzVR",
		"A scene description.", "https://cdn.example/t.jpg", "2021-05-15T12:27:23+00:00"))

	if got.title != "Cum Back For You" {
		t.Errorf("title = %q — the branding suffix should be stripped", got.title)
	}
	if got.description != "A scene description." {
		t.Errorf("description = %q, want the scene's own", got.description)
	}
	if got.thumbnail != "https://cdn.example/t.jpg" {
		t.Errorf("thumbnail = %q", got.thumbnail)
	}
	if got.date.Format("2006-01-02") != "2021-05-15" {
		t.Errorf("date = %v", got.date)
	}
}

// With no usable JSON-LD the OpenGraph tags are the fallback.
func TestOpenGraphFallback(t *testing.T) {
	body := `<html><head>
<meta property="og:title" content="Bare All featuring Alisia @ StripzVR.com - StripzVR" />
<meta property="og:description" content="OG description" />
<meta property="og:image" content="https://cdn.example/og.jpg" />
</head><body></body></html>`
	got := parsePage(body)
	if got.title != "Bare All" {
		t.Errorf("title = %q", got.title)
	}
	if got.description != "OG description" {
		t.Errorf("description = %q", got.description)
	}
	if got.thumbnail != "https://cdn.example/og.jpg" {
		t.Errorf("thumbnail = %q", got.thumbnail)
	}
}

// The title is the only place the site spells the name properly; the slug is
// the fallback for a title that does not carry it.
func TestPerformerNamePrefersTheTitle(t *testing.T) {
	cases := []struct {
		title, slug, want string
	}{
		{"Cum Back For You featuring Tiny Tina @ StripzVR.com", "tiny-tina", "Tiny Tina"},
		{"Bare All", "melena-maria-rya", "Melena Maria Rya"},
		{"", "quinn-linden", "Quinn Linden"},
	}
	for _, c := range cases {
		if got := performerName(c.title, c.slug); got != c.want {
			t.Errorf("performerName(%q, %q) = %q, want %q", c.title, c.slug, got, c.want)
		}
	}
}

func TestScenePath(t *testing.T) {
	cases := map[string]string{
		"https://www.stripzvr.com/tiny-tina/cum-back-for-you/": "/tiny-tina/cum-back-for-you/",
		"https://www.stripzvr.com/tiny-tina/cum-back-for-you":  "/tiny-tina/cum-back-for-you/",
		"https://www.stripzvr.com/community/":                  "",
		"https://www.stripzvr.com/":                            "",
		"https://www.stripzvr.com/members/bonus-updates/":      "",
		"https://www.stripzvr.com/members/lauren-brock/x/":     "",
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

// A single scene URL skips the sitemap walk entirely.
func TestSingleSceneURLSkipsTheSitemaps(t *testing.T) {
	site := newSite(t)
	defer site.Close()

	s := newTestScraper(site.Server)
	scenes, errs, total := collect(t, s, "https://www.stripzvr.com/alisia/hot-pants/", scraper.ListOpts{Workers: 1})
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(scenes) != 1 || total != 1 {
		t.Fatalf("got %d scenes (total %d), want 1", len(scenes), total)
	}
	if site.count("/page-sitemap.xml") != 0 {
		t.Error("a single-scene URL read the sitemaps")
	}
	if scenes[0].ID != "alisia/hot-pants" {
		t.Errorf("ID = %q", scenes[0].ID)
	}
}

// The second sitemap only exists while the catalogue needs it, so a missing
// one is not worth failing the run over. A missing first one is.
func TestSecondSitemapMayBeAbsent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/page-sitemap.xml":
			_, _ = fmt.Fprint(w, sitemapXML(liveHost, "/tiny-tina/cum-back-for-you/"))
		case "/page-sitemap2.xml":
			http.NotFound(w, r)
		default:
			_, _ = fmt.Fprint(w, scenePageHTML("A Scene featuring Tiny Tina @ StripzVR.com", "d", "t", "2021-05-15T12:27:23+00:00"))
		}
	}))
	defer srv.Close()

	s := newTestScraper(srv)
	scenes, errs, _ := collect(t, s, "https://www.stripzvr.com/", scraper.ListOpts{Workers: 1})
	if len(errs) != 0 {
		t.Errorf("an absent second sitemap was reported: %v", errs)
	}
	if len(scenes) != 1 {
		t.Fatalf("got %d scenes, want 1", len(scenes))
	}
}

func TestMissingFirstSitemapIsAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.NotFound(w, nil)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	s := newTestScraper(srv)
	scenes, errs, _ := collect(t, s, "https://www.stripzvr.com/", scraper.ListOpts{Workers: 1})
	if len(scenes) != 0 {
		t.Fatalf("got %d scenes", len(scenes))
	}
	if len(errs) == 0 {
		t.Fatal("a missing first sitemap reported no error")
	}
}

// A sitemap that fetched cleanly and named no scenes is a template or URL-shape
// change, which must not read as an empty catalogue to an authoritative save.
func TestSitemapWithNoScenesIsAParseError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, sitemapXML(liveHost, "/", "/community/", "/p8/"))
	}))
	defer srv.Close()

	s := newTestScraper(srv)
	scenes, errs, _ := collect(t, s, "https://www.stripzvr.com/", scraper.ListOpts{Workers: 1})
	if len(scenes) != 0 {
		t.Fatalf("got %d scenes", len(scenes))
	}
	if len(errs) == 0 {
		t.Fatal("a sitemap naming no scenes reported no error")
	}
	if k := scraper.Classify(errs[0]); k != scraper.FailureParse {
		t.Errorf("classified as %v, want FailureParse", k)
	}
}

func TestMatchesURL(t *testing.T) {
	s := New()
	for _, u := range []string{
		"https://stripzvr.com",
		"https://www.stripzvr.com/",
		"http://www.stripzvr.com/tiny-tina/cum-back-for-you/",
	} {
		if !s.MatchesURL(u) {
			t.Errorf("MatchesURL(%q) = false, want true", u)
		}
	}
	for _, u := range []string{"https://stripzvrfan.com/", "https://example.com/stripzvr.com"} {
		if s.MatchesURL(u) {
			t.Errorf("MatchesURL(%q) = true, want false", u)
		}
	}
}

func TestScenesValidate(t *testing.T) {
	site := newSite(t)
	defer site.Close()

	s := newTestScraper(site.Server)
	scenes, _, _ := collect(t, s, "https://www.stripzvr.com/", scraper.ListOpts{Workers: 2})
	for _, sc := range scenes {
		testutil.ValidateScene(t, sc)
	}
}

func TestContextCancellationStopsTheWalk(t *testing.T) {
	site := newSite(t)
	defer site.Close()

	s := newTestScraper(site.Server)
	ctx, cancel := context.WithCancel(context.Background())
	ch, err := s.ListScenes(ctx, "https://www.stripzvr.com/", scraper.ListOpts{Workers: 2, Delay: 10 * time.Millisecond})
	if err != nil {
		t.Fatalf("ListScenes: %v", err)
	}
	cancel()
	for range ch { //nolint:revive // drain so the goroutines can finish their sends
	}
}
