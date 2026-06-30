package puffyutil

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/Wasylq/FSS/scraper"
)

// listingHTML serves two scene cards in two different dialects: a Family-A
// "video-frame" card (absolute href, title attr, date span) and a VirtualPee
// "row" card (relative href without trailing slash, <strong> date).
func listingHTML(base string) string {
	return fmt.Sprintf(`<html><body>
<div class="video-frame">
  <a class="kt_imgrc" href="%s/videos/victoria-sucked/" title="Victoria Sucked">
    <img class="thumb" src="//media.test.example/videos/victoria-sucked/cover/l.jpg">
  </a>
  <span class="Name"><a href="%s/videos/victoria-sucked/">Victoria Sucked</a></span>
  <span class="model-box"><i>Girl:</i> <a href="/girls/victoria/">Victoria</a></span>
  <div class="home-meta"><span class="date">Jun 26, 2026</span></div>
</div>
<li class="row">
  <a href="/videos/video-diana-gold"><span class="vid_wrap"><img src="//media.test.example/videos/video-diana-gold/cover/hd.jpg"></span></a>
  <figcaption><ul><li><strong>Aug 30, 2025</strong></li></ul></figcaption>
</li>
<ul class="footer-nav"><li><a href="/videos/page-2/">Go to page 2</a></li><li><a href="/girls/">Models</a></li></ul>
</body></html>`, base, base)
}

// detailA is the Family-A template: model-movie / <b> labels / movie-description.
const detailA = `<html><head>
<meta property="og:image" content="https://media.test.example/videos/victoria-sucked/cover/hd.jpg">
</head><body>
<div class="video-data">
  <span class="model-movie">Featuring: <b><a href="https://www.test.example/girls/victoria/">Victoria</a></b></span>
  <span class="date-movie">Released on: <b>Jun 26, 2026</b></span>
  <span class="info-movie">Duration: <b>27' 45''</b></span>
</div>
<div class="movie-description">Victoria looked sexy in her sheer black lingerie.</div>
</body></html>`

// detailB is the VirtualPee template: video_title h2 / vid_duration / meta desc.
const detailB = `<html><head>
<meta property="og:image" content="//media.test.example/videos/video-diana-gold/cover/hd.jpg">
<meta name="description" content="Diana Gold VR pee scene.">
</head><body>
<h2 class="video_title">Diana Gold with <strong><a href='/girls/diana-gold'>Diana Gold</a></strong></h2>
<ul class="vid_meta"><li class="vid_duration">17:40</li><li class="vid_type">SBS</li></ul>
</body></html>`

func testConfig(base string) SiteConfig {
	return SiteConfig{
		ID:          "wetandpuffy",
		Studio:      "Wet and Puffy",
		SiteBase:    base,
		ListingPath: "videos",
		ScenePrefix: "",
		MatchRe:     regexp.MustCompile(`^https?://(?:www\.)?wetandpuffy\.com`),
	}
}

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		switch r.URL.Path {
		case "/videos/", "/videos/page-2/":
			// page-2 repeats page-1's cards so the dedup stop fires.
			_, _ = fmt.Fprint(w, listingHTML(srv.URL))
		case "/videos/victoria-sucked/":
			_, _ = fmt.Fprint(w, detailA)
		case "/videos/video-diana-gold":
			_, _ = fmt.Fprint(w, detailB)
		default:
			http.NotFound(w, r)
		}
	}))
	return srv
}

func TestListScenes_endToEnd(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	cfg := testConfig(ts.URL)
	s := New(cfg)
	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{Workers: 2})
	if err != nil {
		t.Fatal(err)
	}

	byID := map[string]scraper.SceneResult{}
	for r := range ch {
		switch r.Kind {
		case scraper.KindScene:
			byID[r.Scene.ID] = r
		case scraper.KindError:
			t.Errorf("unexpected error: %v", r.Err)
		}
	}
	if len(byID) != 2 {
		t.Fatalf("got %d scenes, want 2 (page-2 dup should stop, nav links ignored)", len(byID))
	}

	// --- Family-A scene ---
	a, ok := byID["victoria-sucked"]
	if !ok {
		t.Fatal("missing scene victoria-sucked")
	}
	sc := a.Scene
	if sc.Title != "Victoria Sucked" {
		t.Errorf("A Title = %q, want %q", sc.Title, "Victoria Sucked")
	}
	if sc.URL != ts.URL+"/videos/victoria-sucked/" {
		t.Errorf("A URL = %q", sc.URL)
	}
	if sc.Date.Year() != 2026 || sc.Date.Month() != 6 || sc.Date.Day() != 26 {
		t.Errorf("A Date = %v, want 2026-06-26", sc.Date)
	}
	if sc.Duration != 27*60+45 {
		t.Errorf("A Duration = %d, want %d", sc.Duration, 27*60+45)
	}
	if len(sc.Performers) != 1 || sc.Performers[0] != "Victoria" {
		t.Errorf("A Performers = %v, want [Victoria]", sc.Performers)
	}
	if sc.Thumbnail != "https://media.test.example/videos/victoria-sucked/cover/hd.jpg" {
		t.Errorf("A Thumbnail = %q (want og:image hd.jpg)", sc.Thumbnail)
	}
	if sc.Description != "Victoria looked sexy in her sheer black lingerie." {
		t.Errorf("A Description = %q", sc.Description)
	}
	if sc.Studio != "Wet and Puffy" || sc.SiteID != "wetandpuffy" {
		t.Errorf("A Studio/SiteID = %q/%q", sc.Studio, sc.SiteID)
	}

	// --- VirtualPee-style scene (relative href, no trailing slash) ---
	b, ok := byID["video-diana-gold"]
	if !ok {
		t.Fatal("missing scene video-diana-gold")
	}
	sc = b.Scene
	if sc.Title != "Diana Gold" {
		t.Errorf("B Title = %q, want %q (from slug)", sc.Title, "Diana Gold")
	}
	if sc.URL != ts.URL+"/videos/video-diana-gold" {
		t.Errorf("B URL = %q", sc.URL)
	}
	if sc.Date.Year() != 2025 || sc.Date.Month() != 8 || sc.Date.Day() != 30 {
		t.Errorf("B Date = %v, want 2025-08-30", sc.Date)
	}
	if sc.Duration != 17*60+40 {
		t.Errorf("B Duration = %d, want %d", sc.Duration, 17*60+40)
	}
	if len(sc.Performers) != 1 || sc.Performers[0] != "Diana Gold" {
		t.Errorf("B Performers = %v, want [Diana Gold]", sc.Performers)
	}
	if sc.Thumbnail != "https://media.test.example/videos/video-diana-gold/cover/hd.jpg" {
		t.Errorf("B Thumbnail = %q", sc.Thumbnail)
	}
	if sc.Description != "Diana Gold VR pee scene." {
		t.Errorf("B Description = %q", sc.Description)
	}
}

func TestMatchesURL(t *testing.T) {
	s := New(testConfig("https://wetandpuffy.com"))
	cases := []struct {
		url   string
		match bool
	}{
		{"https://wetandpuffy.com/", true},
		{"http://www.wetandpuffy.com/videos/page-2/", true},
		{"https://vipissy.com/updates/", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.match {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.match)
		}
	}
}

func TestTitleFromSlug(t *testing.T) {
	cases := []struct {
		prefix, slug, want string
	}{
		{"", "victoria-sucked", "Victoria Sucked"},
		{"video-", "video-wet-yoga", "Wet Yoga"},
		{"video-", "video-diana-gold", "Diana Gold"},
		{"", "alexa-throat", "Alexa Throat"},
	}
	for _, c := range cases {
		s := New(SiteConfig{ScenePrefix: c.prefix, ListingPath: "videos", MatchRe: regexp.MustCompile(`.*`)})
		if got := s.titleFromSlug(c.slug); got != c.want {
			t.Errorf("titleFromSlug(%q) = %q, want %q", c.slug, got, c.want)
		}
	}
}

func TestParseDurationValue(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"27' 45''", 27*60 + 45},
		{"33:33", 33*60 + 33},
		{"1:02:03", 3723},
		{"n/a", 0},
		{"5'", 300},
	}
	for _, c := range cases {
		if got := parseDurationValue(c.in); got != c.want {
			t.Errorf("parseDurationValue(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestCleanText(t *testing.T) {
	cases := []struct{ in, want string }{
		{"<b>Hi</b> &amp; there", "Hi & there"},
		{"  spaced   out  ", "spaced out"},
	}
	for _, c := range cases {
		if got := cleanText(c.in); got != c.want {
			t.Errorf("cleanText(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestAbsURL(t *testing.T) {
	s := New(testConfig("https://wetandpuffy.com"))
	cases := []struct{ in, want string }{
		{"https://www.wetandpuffy.com/videos/x/", "https://www.wetandpuffy.com/videos/x/"},
		{"//media.x.com/a.jpg", "https://media.x.com/a.jpg"},
		{"/videos/x", "https://wetandpuffy.com/videos/x"},
	}
	for _, c := range cases {
		if got := s.absURL(c.in); got != c.want {
			t.Errorf("absURL(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
