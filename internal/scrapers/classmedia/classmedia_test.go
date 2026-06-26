package classmedia

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wasylq/FSS/scraper"
)

func TestCleanText(t *testing.T) {
	cases := map[string]string{
		"<h1>Hello <b>World</b></h1>": "Hello World",
		"  Tom &amp; Jerry  ":         "Tom & Jerry",
		"a\n\tb   c":                  "a b c",
	}
	for in, want := range cases {
		if got := cleanText(in); got != want {
			t.Errorf("cleanText(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDeslug(t *testing.T) {
	cases := map[string]string{
		"cosplay-love":   "Cosplay Love",
		"a-b-c":          "A B C",
		"single":         "Single",
		"trailing-dash-": "Trailing Dash",
		"":               "",
	}
	for in, want := range cases {
		if got := deslug(in); got != want {
			t.Errorf("deslug(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSslID(t *testing.T) {
	cases := map[string]string{
		"https://www.subspaceland.com/video/jane/the-scene/": "jane/the-scene",
		"https://www.subspaceland.com/video/jane/the-scene":  "jane/the-scene",
		"no-video-segment": "no-video-segment",
	}
	for in, want := range cases {
		if got := sslID(in); got != want {
			t.Errorf("sslID(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMatchesURL(t *testing.T) {
	ssl := NewSubspaceland()
	oj := NewOldje()
	o3 := NewOldje3some()
	if !ssl.MatchesURL("https://www.subspaceland.com/video/x/y") {
		t.Error("subspaceland should match")
	}
	if !oj.MatchesURL("http://oldje.com/gallery/2") {
		t.Error("oldje should match")
	}
	if !o3.MatchesURL("https://www.oldje-3some.com/videos") {
		t.Error("oldje3some should match")
	}
	if oj.MatchesURL("https://www.oldje-3some.com/") {
		t.Error("oldje must not match oldje-3some")
	}
}

// ---- Subspaceland detail parsing ----

const subspacelandDetailHTML = `<html><head>
<meta name="description" content="A tense &amp; intimate session.">
</head><body>
<h1>The Binding <span>Session</span></h1>
<div class="meta">Released on 14 Mar 2025</div>
<a href="https://www.subspaceland.com/model/jane-doe">Jane Doe</a>
<a href="https://www.subspaceland.com/tag/rope">Rope</a>
<a href="https://www.subspaceland.com/tag/bondage">Bondage</a>
<a href="https://www.subspaceland.com/tag/rope">Rope</a>
<img src="/sets/4096/main.jpg">
</body></html>`

func TestScrapeSubspacelandDetail(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, subspacelandDetailHTML)
	}))
	defer ts.Close()

	s := NewSubspaceland()
	s.Client = ts.Client()
	now := time.Date(2026, 6, 25, 0, 0, 0, 0, time.UTC)

	detailURL := ts.URL + "/video/jane-doe/the-binding-session"
	scene, ok := s.scrapeSubspacelandDetail(context.Background(), "https://www.subspaceland.com", detailURL, now)
	if !ok {
		t.Fatal("scrapeSubspacelandDetail returned ok=false")
	}
	if scene.Title != "The Binding Session" {
		t.Errorf("Title = %q", scene.Title)
	}
	if scene.ID != "jane-doe/the-binding-session" {
		t.Errorf("ID = %q", scene.ID)
	}
	wantDate := time.Date(2025, 3, 14, 0, 0, 0, 0, time.UTC)
	if !scene.Date.Equal(wantDate) {
		t.Errorf("Date = %v, want %v", scene.Date, wantDate)
	}
	if len(scene.Performers) != 1 || scene.Performers[0] != "Jane Doe" {
		t.Errorf("Performers = %v", scene.Performers)
	}
	if len(scene.Tags) != 2 || scene.Tags[0] != "Rope" || scene.Tags[1] != "Bondage" {
		t.Errorf("Tags = %v, want dedup [Rope Bondage]", scene.Tags)
	}
	if scene.Thumbnail != "https://www.subspaceland.com/sets/4096/mov_img/movie_preview.jpg" {
		t.Errorf("Thumbnail = %q", scene.Thumbnail)
	}
	if scene.Description != "A tense & intimate session." {
		t.Errorf("Description = %q", scene.Description)
	}
	if scene.Studio != "Subspaceland" {
		t.Errorf("Studio = %q", scene.Studio)
	}
}

func TestScrapeSubspacelandDetail_noTitleDropped(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, "<html><body>no heading here</body></html>")
	}))
	defer ts.Close()

	s := NewSubspaceland()
	s.Client = ts.Client()
	_, ok := s.scrapeSubspacelandDetail(context.Background(), "x", ts.URL+"/video/a/b", time.Now())
	if ok {
		t.Error("expected ok=false when no <h1> title")
	}
}

func TestFetchSitemap_dedupsAndNormalizes(t *testing.T) {
	sitemap := `<?xml version="1.0"?><urlset>
<url><loc>http://www.subspaceland.com/video/jane/scene-one</loc></url>
<url><loc>https://www.subspaceland.com/video/jane/scene-one</loc></url>
<url><loc>https://subspaceland.com/video/mary/scene-two</loc></url>
</urlset>`
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/sitemap.xml" {
			_, _ = fmt.Fprint(w, sitemap)
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	s := NewSubspaceland()
	s.Client = ts.Client()
	s.cfg.base = ts.URL

	urls, err := s.fetchSitemap(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// http:// normalized to https:// then deduped -> 2 unique.
	if len(urls) != 2 {
		t.Fatalf("got %d urls, want 2: %v", len(urls), urls)
	}
	for _, u := range urls {
		if u[:8] != "https://" {
			t.Errorf("url not normalized to https: %q", u)
		}
	}
}

// ---- Oldje listing parsing ----

const oldjeGalleryHTML = `<html><body>
<div class="set"><img src="/sets/537/cosplay-love.webp"></div>
<div class="set"><img src="/sets/538/the-old-painter.webp"></div>
<div class="set"><img src="/sets/537/cosplay-love.webp"></div>
</body></html>`

func TestRunOldje(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		switch r.URL.Path {
		case "/gallery/1":
			_, _ = fmt.Fprint(w, oldjeGalleryHTML)
		default:
			_, _ = fmt.Fprint(w, "<html><body></body></html>")
		}
	}))
	defer ts.Close()

	s := NewOldje()
	s.Client = ts.Client()
	s.cfg.base = ts.URL

	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for r := range ch {
		switch r.Kind {
		case scraper.KindScene:
			got[r.Scene.ID] = r.Scene.Title
			if r.Scene.Thumbnail == "" {
				t.Errorf("scene %s missing thumbnail", r.Scene.ID)
			}
		case scraper.KindError:
			t.Errorf("unexpected error: %v", r.Err)
		}
	}
	if len(got) != 2 {
		t.Fatalf("got %d scenes, want 2 (deduped): %v", len(got), got)
	}
	if got["537"] != "Cosplay Love" || got["538"] != "The Old Painter" {
		t.Errorf("scenes = %v", got)
	}
}

// ---- Oldje-3some listing parsing ----

const oldje3someListHTML = `<html><body>
<a href="/videos/set/abc123" class="card"> <img src="/view/photoCoverBig/789"></a>
<a href="/videos/set/def456" class="card"> <img src="/view/photoCoverBig/790"></a>
<a href="/videos/set/abc123" class="card"> <img src="/view/photoCoverBig/789"></a>
</body></html>`

func TestRunOldje3some(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		switch r.URL.Path {
		case "/videos":
			_, _ = fmt.Fprint(w, oldje3someListHTML)
		default:
			_, _ = fmt.Fprint(w, "<html><body></body></html>")
		}
	}))
	defer ts.Close()

	s := NewOldje3some()
	s.Client = ts.Client()
	s.cfg.base = ts.URL

	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	urls := map[string]string{}
	for r := range ch {
		switch r.Kind {
		case scraper.KindScene:
			got[r.Scene.ID] = r.Scene.Title
			urls[r.Scene.ID] = r.Scene.URL
		case scraper.KindError:
			t.Errorf("unexpected error: %v", r.Err)
		}
	}
	if len(got) != 2 {
		t.Fatalf("got %d scenes, want 2 (deduped): %v", len(got), got)
	}
	if got["789"] != "Abc123" {
		t.Errorf("title for 789 = %q, want Abc123 (deslugged)", got["789"])
	}
	if urls["789"] != ts.URL+"/videos/set/abc123" {
		t.Errorf("URL for 789 = %q", urls["789"])
	}
}

func TestIDAndPatterns(t *testing.T) {
	s := NewSubspaceland()
	if s.ID() != "subspaceland" {
		t.Errorf("ID = %q", s.ID())
	}
	if len(s.Patterns()) == 0 {
		t.Error("Patterns empty")
	}
}
