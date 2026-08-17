package bananafever

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

// scenePage carries a VideoObject with an ARRAY thumbnailUrl — the shape the
// live site emits, and the one that made parseutil skip the whole block until
// it learned to accept it — plus a BreadcrumbList that must not be mistaken
// for the scene.
func scenePage(title, desc, date string, actors []string) string {
	var actorJSON strings.Builder
	for i, a := range actors {
		if i > 0 {
			actorJSON.WriteString(",")
		}
		fmt.Fprintf(&actorJSON, `{"@type":"Person","name":%q}`, a)
	}
	return fmt.Sprintf(`<html><head>
<meta property="og:title" content="%s" />
<script type="application/ld+json">
{"@context":"https://schema.org","@type":"VideoObject","name":%q,"description":%q,
 "thumbnailUrl":["https://cdn.example/thumb-1.jpg","https://cdn.example/thumb-2.jpg"],
 "uploadDate":%q,
 "contentUrl":"https://stream.example/manifest/video.m3u8",
 "actor":[%s]}
</script>
<script type="application/ld+json">
{"@context":"https://schema.org","@type":"BreadcrumbList","itemListElement":[
 {"@type":"ListItem","position":1,"name":"Home","item":"https://bananafever.com/"}]}
</script>
</head><body></body></html>`, title, title, desc, date, actorJSON.String())
}

// sitemapXML mirrors the live feed, including the `hreflang` alternates that
// address the same scene under a language prefix.
func sitemapXML(base string, paths ...string) string {
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?><urlset xmlns:xhtml="http://www.w3.org/1999/xhtml">`)
	for _, p := range paths {
		fmt.Fprintf(&sb, `<url><loc>%s%s</loc>`+
			`<xhtml:link rel="alternate" hreflang="en" href="%s%s"/>`+
			`<xhtml:link rel="alternate" hreflang="zh-CN" href="%s/cn%s"/>`+
			`</url>`, base, p, base, p, base, p)
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

const liveHost = "https://bananafever.com"

func newSite(t *testing.T) *stubSite {
	t.Helper()
	site := &stubSite{hits: map[string]int{}}
	site.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		site.hit(r.URL.Path)
		switch {
		case r.URL.Path == "/sitemap-videos.xml":
			_, _ = fmt.Fprint(w, sitemapXML(liveHost,
				"/video/first-scene-aB3xYz",
				"/video/second-scene-Qw9ErT",
				// A repeat of the first: must not be fetched twice.
				"/video/first-scene-aB3xYz",
			))
		case strings.HasPrefix(r.URL.Path, "/video/"):
			_, _ = fmt.Fprint(w, scenePage("A Scene Title", "A description.",
				"2026-07-20T00:00:00.000Z", []string{"Chloe Scott", "Jax"}))
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

func TestSitemapWalkFetchesEachSceneOnce(t *testing.T) {
	site := newSite(t)
	defer site.Close()

	s := newTestScraper(site.Server)
	scenes, errs, total := collect(t, s, "https://bananafever.com/", scraper.ListOpts{Workers: 2})
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(scenes) != 2 || total != 2 {
		t.Fatalf("got %d scenes (total %d), want 2", len(scenes), total)
	}
	if got := site.count("/video/first-scene-aB3xYz"); got != 1 {
		t.Errorf("the repeated sitemap entry was fetched %d times, want 1", got)
	}
}

// The site is multilingual and every entry advertises `/cn/video/…` and other
// alternates for the same scene. Only the `<loc>` is followed, or a scene ends
// up stored once per language.
func TestLanguageAlternatesAreNotFollowed(t *testing.T) {
	site := newSite(t)
	defer site.Close()

	s := newTestScraper(site.Server)
	collect(t, s, "https://bananafever.com/", scraper.ListOpts{Workers: 1})
	if site.count("/cn/video/first-scene-aB3xYz") != 0 {
		t.Error("a language alternate was scraped as its own scene")
	}
}

func TestSceneFieldsComeFromTheVideoObject(t *testing.T) {
	site := newSite(t)
	defer site.Close()

	s := newTestScraper(site.Server)
	scenes, _, _ := collect(t, s, "https://bananafever.com/video/first-scene-aB3xYz", scraper.ListOpts{Workers: 1})
	if len(scenes) != 1 {
		t.Fatalf("got %d scenes, want 1", len(scenes))
	}
	got := scenes[0]

	// The id is the slug's trailing short token, not the whole slug.
	if got.ID != "aB3xYz" {
		t.Errorf("ID = %q, want the short id", got.ID)
	}
	if got.Title != "A Scene Title" {
		t.Errorf("Title = %q", got.Title)
	}
	if got.Description != "A description." {
		t.Errorf("Description = %q", got.Description)
	}
	// An array thumbnailUrl resolves to its first entry.
	if got.Thumbnail != "https://cdn.example/thumb-1.jpg" {
		t.Errorf("Thumbnail = %q", got.Thumbnail)
	}
	if got.Date.Format("2006-01-02") != "2026-07-20" {
		t.Errorf("Date = %v", got.Date)
	}
	if strings.Join(got.Performers, ",") != "Chloe Scott,Jax" {
		t.Errorf("Performers = %v", got.Performers)
	}
	if got.Studio != studioName || got.SiteID != siteID {
		t.Errorf("Studio/SiteID = %q/%q", got.Studio, got.SiteID)
	}
}

// The BreadcrumbList block sits alongside the VideoObject and names the scene
// too; nothing may take it for the scene record.
func TestBreadcrumbBlockIsNotMistakenForTheScene(t *testing.T) {
	site := newSite(t)
	defer site.Close()

	s := newTestScraper(site.Server)
	scenes, _, _ := collect(t, s, "https://bananafever.com/video/first-scene-aB3xYz", scraper.ListOpts{Workers: 1})
	if len(scenes) != 1 {
		t.Fatalf("got %d scenes, want exactly one", len(scenes))
	}
	if scenes[0].Title == "Home" {
		t.Error("the breadcrumb was read as the scene")
	}
}

func TestScenePath(t *testing.T) {
	cases := map[string]string{
		"https://bananafever.com/video/first-scene-aB3xYz":  "/video/first-scene-aB3xYz",
		"https://bananafever.com/video/first-scene-aB3xYz/": "/video/first-scene-aB3xYz",
		"https://bananafever.com/cn/video/first-aB3xYz":     "",
		"https://bananafever.com/videos":                    "",
		"https://bananafever.com/":                          "",
		"https://bananafever.com/categories":                "",
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

// A sitemap that fetched cleanly and named no scenes is a feed change, not an
// empty catalogue, and must not read as one to an authoritative --full save.
func TestSitemapWithNoScenesIsAParseError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, sitemapXML(liveHost, "/videos", "/categories"))
	}))
	defer srv.Close()

	s := newTestScraper(srv)
	scenes, errs, _ := collect(t, s, "https://bananafever.com/", scraper.ListOpts{Workers: 1})
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

// A page with no VideoObject is a template change, not an empty scene.
func TestPageWithoutAVideoObjectIsAParseError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/sitemap-videos.xml" {
			_, _ = fmt.Fprint(w, sitemapXML(liveHost, "/video/first-scene-aB3xYz"))
			return
		}
		_, _ = fmt.Fprint(w, "<html><body>nothing here</body></html>")
	}))
	defer srv.Close()

	s := newTestScraper(srv)
	scenes, errs, _ := collect(t, s, "https://bananafever.com/", scraper.ListOpts{Workers: 1})
	if len(scenes) != 0 {
		t.Fatalf("got %d scenes", len(scenes))
	}
	if len(errs) == 0 {
		t.Fatal("a page with no VideoObject reported no error")
	}
	if k := scraper.Classify(errs[0]); k != scraper.FailureParse {
		t.Errorf("classified as %v, want FailureParse", k)
	}
}

func TestMatchesURL(t *testing.T) {
	s := New()
	for _, u := range []string{
		"https://bananafever.com",
		"https://www.bananafever.com/",
		"http://bananafever.com/video/some-scene-aB3xYz",
	} {
		if !s.MatchesURL(u) {
			t.Errorf("MatchesURL(%q) = false, want true", u)
		}
	}
	for _, u := range []string{"https://bananafeverfan.com/", "https://example.com/bananafever.com"} {
		if s.MatchesURL(u) {
			t.Errorf("MatchesURL(%q) = true, want false", u)
		}
	}
}

func TestScenesValidate(t *testing.T) {
	site := newSite(t)
	defer site.Close()

	s := newTestScraper(site.Server)
	scenes, _, _ := collect(t, s, "https://bananafever.com/", scraper.ListOpts{Workers: 2})
	for _, sc := range scenes {
		testutil.ValidateScene(t, sc)
	}
}

func TestContextCancellationStopsTheWalk(t *testing.T) {
	site := newSite(t)
	defer site.Close()

	s := newTestScraper(site.Server)
	ctx, cancel := context.WithCancel(context.Background())
	ch, err := s.ListScenes(ctx, "https://bananafever.com/", scraper.ListOpts{Workers: 2, Delay: 10 * time.Millisecond})
	if err != nil {
		t.Fatalf("ListScenes: %v", err)
	}
	cancel()
	for range ch { //nolint:revive // drain so the goroutines can finish their sends
	}
}
