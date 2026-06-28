package puremediautil

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/Wasylq/FSS/scraper"
)

func modelsIndexHTML(base string) string {
	return fmt.Sprintf(`<html><body>
<div class="models">
  <a href="%s/tour/models/Jane-Doe.html">Jane Doe</a>
  <a href="%s/tour/models/Amy-Lee.html">Amy Lee</a>
  <a href="%s/tour/models/models.html">All Models</a>
</div>
</body></html>`, base, base, base)
}

func janeDoeHTML(base string) string {
	return fmt.Sprintf(`<html><body>
<div class="sceneItem">
  <a href="%s/tour/trailers/scene-one.html"><img></a>
  <span class="updatedScenes">Jun 22, 2026</span>
</div>
<div class="sceneItem">
  <a href="%s/tour/trailers/scene-two.html"><img></a>
  <span class="updatedScenes">Jun 15, 2026</span>
</div>
</body></html>`, base, base)
}

func amyLeeHTML(base string) string {
	return fmt.Sprintf(`<html><body>
<div class="sceneItem">
  <a href="%s/tour/trailers/scene-one.html"><img></a>
  <span class="updatedScenes">Jun 22, 2026</span>
</div>
<div class="sceneItem">
  <a href="%s/tour/trailers/scene-three.html"><img></a>
  <span class="updatedScenes">Jun 8, 2026</span>
</div>
</body></html>`, base, base)
}

const sceneOneHTML = `<html><head>
<meta property="og:title" content="dancer tryouts">
<meta property="og:description" content="OG fallback description">
</head><body>
<div class="vpTitle"><h1>Dancer Tryouts</h1></div>
<div class="descriptionR"><div class="description"><h4>Description</h4><p>A hot debut scene with &amp; lots of action.</p></div></div>
<img src0_1x="/tour/content/contentthumbs/90/34/19034-1x.jpg">
</body></html>`

const sceneTwoHTML = `<html><head>
<meta property="og:title" content="second scene">
</head><body>
<div class="vpTitle"><h1>Second Scene</h1></div>
<div class="descriptionR"><div class="description"><h4>Description</h4><p>Another one.</p></div></div>
<img src0_1x="/tour/content/contentthumbs/90/35/19035-1x.jpg">
</body></html>`

const sceneThreeHTML = `<html><head></head><body>
<meta property="og:title" content="third scene">
<meta property="og:description" content="Third via og only">
<img src0_1x="/tour/content/contentthumbs/90/36/19036-1x.jpg">
</body></html>`

func testConfig(base string) SiteConfig {
	return SiteConfig{
		ID:       "purets",
		Studio:   "PureTS",
		SiteBase: base,
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?pure-ts\.com`),
	}
}

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		switch r.URL.Path {
		case "/tour/models/models.html":
			_, _ = fmt.Fprint(w, modelsIndexHTML(srv.URL))
		case "/tour/models/Jane-Doe.html":
			_, _ = fmt.Fprint(w, janeDoeHTML(srv.URL))
		case "/tour/models/Amy-Lee.html":
			_, _ = fmt.Fprint(w, amyLeeHTML(srv.URL))
		case "/tour/trailers/scene-one.html":
			_, _ = fmt.Fprint(w, sceneOneHTML)
		case "/tour/trailers/scene-two.html":
			_, _ = fmt.Fprint(w, sceneTwoHTML)
		case "/tour/trailers/scene-three.html":
			_, _ = fmt.Fprint(w, sceneThreeHTML)
		default:
			http.NotFound(w, r)
		}
	}))
	return srv
}

func TestListScenes_endToEnd(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	s := New(testConfig(ts.URL))
	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}

	byID := map[string]scraper.SceneResult{}
	total := -1
	for r := range ch {
		switch r.Kind {
		case scraper.KindScene:
			byID[r.Scene.ID] = r
		case scraper.KindTotal:
			total = r.Total
		case scraper.KindError:
			t.Errorf("unexpected error: %v", r.Err)
		}
	}
	if len(byID) != 3 {
		t.Fatalf("got %d scenes, want 3", len(byID))
	}
	if total != 3 {
		t.Errorf("Progress total = %d, want 3", total)
	}

	one, ok := byID["19034"]
	if !ok {
		t.Fatal("missing scene id 19034")
	}
	sc := one.Scene
	if sc.Title != "Dancer Tryouts" {
		t.Errorf("Title = %q, want vpTitle", sc.Title)
	}
	if sc.Date.Year() != 2026 || sc.Date.Month() != 6 || sc.Date.Day() != 22 {
		t.Errorf("Date = %v, want 2026-06-22", sc.Date)
	}
	if sc.Description != "A hot debut scene with & lots of action." {
		t.Errorf("Description = %q", sc.Description)
	}
	if sc.Thumbnail != ts.URL+"/tour/content/contentthumbs/90/34/19034-1x.jpg" {
		t.Errorf("Thumbnail = %q", sc.Thumbnail)
	}
	if sc.Studio != "PureTS" {
		t.Errorf("Studio = %q", sc.Studio)
	}
	// scene-one is listed by both models -> performer union.
	perf := map[string]bool{}
	for _, p := range sc.Performers {
		perf[p] = true
	}
	if len(sc.Performers) != 2 || !perf["Jane Doe"] || !perf["Amy Lee"] {
		t.Errorf("Performers = %v, want {Jane Doe, Amy Lee}", sc.Performers)
	}

	// scene-three has no vpTitle/descriptionR -> falls back to og: tags.
	three, ok := byID["19036"]
	if !ok {
		t.Fatal("missing scene id 19036")
	}
	if three.Scene.Title != "third scene" {
		t.Errorf("scene-three Title = %q, want og fallback", three.Scene.Title)
	}
	if three.Scene.Description != "Third via og only" {
		t.Errorf("scene-three Description = %q, want og fallback", three.Scene.Description)
	}
}

func TestMatchesURL(t *testing.T) {
	s := New(testConfig("https://pure-ts.com"))
	cases := []struct {
		url   string
		match bool
	}{
		{"https://pure-ts.com/", true},
		{"http://www.pure-ts.com/tour/models/models.html", true},
		{"https://pure-bbw.com/", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.match {
			t.Errorf("MatchesURL(%q) = %v", c.url, got)
		}
	}
}

func TestHumanizeModel(t *testing.T) {
	cases := []struct{ in, want string }{
		{"https://pure-ts.com/tour/models/Jane-Doe.html", "Jane Doe"},
		{"https://pure-ts.com/tour/models/Amy_Lee.html", "Amy Lee"},
		{"https://pure-ts.com/tour/models/Bella%20Rae.html", "Bella Rae"},
	}
	for _, c := range cases {
		if got := humanizeModel(c.in); got != c.want {
			t.Errorf("humanizeModel(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestCleanText(t *testing.T) {
	cases := []struct{ in, want string }{
		{"<b>Hi</b> &amp; <i>there</i>  friend", "Hi & there friend"},
		{"  spaced   out  ", "spaced out"},
	}
	for _, c := range cases {
		if got := cleanText(c.in); got != c.want {
			t.Errorf("cleanText(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
