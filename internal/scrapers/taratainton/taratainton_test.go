package taratainton

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
	"github.com/Wasylq/FSS/internal/scrapers/wputil"
	"github.com/Wasylq/FSS/scraper"
)

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url  string
		want bool
	}{
		{"https://www.taratainton.com", true},
		{"https://taratainton.com/home.html", true},
		{"https://www.manyvids.com/Profile/123/foo", false},
		{"https://example.com", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.want {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.want)
		}
	}
}

func TestParseDuration(t *testing.T) {
	cases := []struct {
		input string
		want  int
	}{
		{"17:27", 17*60 + 27},
		{"1:05:30", 1*3600 + 5*60 + 30},
		{"0:45", 45},
	}
	for _, c := range cases {
		if got := wputil.ParseDuration(c.input); got != c.want {
			t.Errorf("ParseDuration(%q) = %d, want %d", c.input, got, c.want)
		}
	}
}

func TestSlugFromURL(t *testing.T) {
	cases := []struct {
		url  string
		want string
	}{
		{"https://www.taratainton.com/some-video-title.html", "some-video-title"},
		{"https://www.taratainton.com/another-title/", "another-title"},
		{"https://www.taratainton.com/foo", "foo"},
	}
	for _, c := range cases {
		if got := wputil.SlugFromURL(c.url); got != c.want {
			t.Errorf("SlugFromURL(%q) = %q, want %q", c.url, got, c.want)
		}
	}
}

func TestVideoWidth(t *testing.T) {
	cases := []struct {
		height int
		want   int
	}{
		{2160, 3840},
		{1080, 1920},
		{720, 1280},
		{480, 854},
		{360, 0},
	}
	for _, c := range cases {
		if got := wputil.VideoWidth(c.height); got != c.want {
			t.Errorf("VideoWidth(%d) = %d, want %d", c.height, got, c.want)
		}
	}
}

const fixtureVideoPage = `<!doctype html><html><head>
<title>Taking Advantage of You - Tara Tainton</title>
<meta property="article:published_time" content="2009-10-26T09:40:00+00:00" />
<meta property="og:description" content="Oh my gosh, you&#039;re tied up!" />
<meta property="og:image" content="https://cdn.example.com/thumb.jpg" />
<link rel='shortlink' href='https://www.taratainton.com/?p=1241' />
</head><body>
<p>Price: $21.99&nbsp;&nbsp;Length: 19:05</p>
<p>*NOW in 1080p FULL HD!</p>
<a href="https://www.taratainton.com/tag/big-breasts" rel="tag">Big Breasts</a>
<a href="https://www.taratainton.com/tag/female-domination" rel="tag">Female Domination</a>
<a href="https://www.manyvids.com/Video/12345/some-video"><img src="btn.png"></a>
<a href="https://clips4sale.com/work/store/index.php?storeid=21571&buy=2813365"><img src="btn.png"></a>
</body></html>`

func TestParsePage(t *testing.T) {
	scene, skip, err := parsePage(
		"https://www.taratainton.com",
		"https://www.taratainton.com/taking-advantage.html",
		[]byte(fixtureVideoPage),
		fixedTime(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if skip {
		t.Fatal("expected video page, got skip")
	}

	if scene.ID != "1241" {
		t.Errorf("ID = %q, want 1241", scene.ID)
	}
	if scene.Title != "Taking Advantage of You" {
		t.Errorf("Title = %q", scene.Title)
	}
	if scene.Date.Year() != 2009 || scene.Date.Month() != 10 || scene.Date.Day() != 26 {
		t.Errorf("Date = %v", scene.Date)
	}
	if scene.Description != "Oh my gosh, you're tied up!" {
		t.Errorf("Description = %q", scene.Description)
	}
	if scene.Thumbnail != "https://cdn.example.com/thumb.jpg" {
		t.Errorf("Thumbnail = %q", scene.Thumbnail)
	}
	if scene.Duration != 19*60+5 {
		t.Errorf("Duration = %d, want %d", scene.Duration, 19*60+5)
	}
	if scene.Resolution != "1080p" {
		t.Errorf("Resolution = %q", scene.Resolution)
	}
	if scene.Width != 1920 || scene.Height != 1080 {
		t.Errorf("Width=%d Height=%d", scene.Width, scene.Height)
	}
	if len(scene.Tags) != 2 || scene.Tags[0] != "Big Breasts" {
		t.Errorf("Tags = %v", scene.Tags)
	}
	if len(scene.Performers) != 1 || scene.Performers[0] != "Tara Tainton" {
		t.Errorf("Performers = %v", scene.Performers)
	}
	if scene.SiteID != "taratainton" {
		t.Errorf("SiteID = %q", scene.SiteID)
	}
	if len(scene.PriceHistory) != 1 || scene.PriceHistory[0].Regular != 21.99 {
		t.Errorf("PriceHistory = %v", scene.PriceHistory)
	}
}

const fixtureBlogPage = `<!doctype html><html><head>
<title>Some Blog Post - Tara Tainton</title>
<meta property="article:published_time" content="2020-01-01T00:00:00+00:00" />
</head><body>
<p>Just a blog post with no price or length info.</p>
</body></html>`

func TestParsePageNonVideo(t *testing.T) {
	_, skip, err := parsePage(
		"https://www.taratainton.com",
		"https://www.taratainton.com/some-blog-post.html",
		[]byte(fixtureBlogPage),
		fixedTime(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if !skip {
		t.Error("expected skip=true for non-video page")
	}
}

func TestParsePageNoShortlink(t *testing.T) {
	page := `<!doctype html><html><head>
<title>Fallback ID Test - Tara Tainton</title>
</head><body>
<p>Price: $9.99&nbsp;&nbsp;Length: 5:00</p>
</body></html>`

	scene, skip, err := parsePage(
		"https://www.taratainton.com",
		"https://www.taratainton.com/fallback-id-test.html",
		[]byte(page),
		fixedTime(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if skip {
		t.Fatal("unexpected skip")
	}
	if scene.ID != "fallback-id-test" {
		t.Errorf("ID = %q, want fallback-id-test (slug)", scene.ID)
	}
}

func TestFetchSitemap(t *testing.T) {
	sitemapXML := `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>https://www.example.com/page-one.html</loc></url>
  <url><loc>https://www.example.com/page-two.html</loc></url>
  <url><loc>https://www.example.com/page-three.html</loc></url>
</urlset>`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(sitemapXML))
	}))
	defer ts.Close()

	urls, err := wputil.FetchSitemap(context.Background(), ts.Client(), ts.URL+"/sitemap.xml", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(urls) != 3 {
		t.Fatalf("got %d URLs, want 3", len(urls))
	}
	if urls[0].Loc != "https://www.example.com/page-one.html" {
		t.Errorf("urls[0] = %q", urls[0].Loc)
	}
}

func TestListScenes(t *testing.T) {
	sitemapXML := `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>%s/video-one.html</loc></url>
  <url><loc>%s/blog-post.html</loc></url>
</urlset>`

	videoPage := `<html><head>
<title>Video One - Tara Tainton</title>
<meta property="article:published_time" content="2024-01-15T10:00:00+00:00" />
<meta property="og:description" content="A video" />
<link rel='shortlink' href='%s/?p=42' />
</head><body>
<p>Price: $14.99&nbsp;&nbsp;Length: 10:30</p>
<a href="https://www.taratainton.com/tag/test-tag" rel="tag">Test Tag</a>
</body></html>`

	blogPage := `<html><head><title>Blog - Tara Tainton</title></head>
<body><p>Just a blog post.</p></body></html>`

	var ts *httptest.Server
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/post-sitemap.xml":
			_, _ = fmt.Fprintf(w, sitemapXML, ts.URL, ts.URL)
		case r.URL.Path == "/post-sitemap2.xml":
			_, _ = w.Write([]byte(`<?xml version="1.0"?><urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"></urlset>`))
		case strings.Contains(r.URL.Path, "video-one"):
			_, _ = fmt.Fprintf(w, videoPage, ts.URL)
		case strings.Contains(r.URL.Path, "blog-post"):
			_, _ = w.Write([]byte(blogPage))
		}
	}))
	defer ts.Close()

	s := &Scraper{
		client:   ts.Client(),
		siteBase: ts.URL,
		headers:  map[string]string{},
	}

	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{Workers: 1})
	if err != nil {
		t.Fatal(err)
	}

	scenes := testutil.CollectScenes(t, ch)

	if len(scenes) != 1 {
		t.Fatalf("got %d scenes, want 1 (blog post should be filtered)", len(scenes))
	}
	if scenes[0].Title != "Video One" {
		t.Errorf("scene title = %q, want Video One", scenes[0].Title)
	}
}

func TestListScenesKnownIDs(t *testing.T) {
	sitemapXML := `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>%s/known-video.html</loc></url>
  <url><loc>%s/new-video.html</loc></url>
</urlset>`

	knownPage := `<html><head><title>Known - Tara Tainton</title>
<link rel='shortlink' href='%s/?p=100' />
</head><body><p>Price: $10.00&nbsp;&nbsp;Length: 5:00</p></body></html>`

	newPage := `<html><head><title>New - Tara Tainton</title>
<link rel='shortlink' href='%s/?p=200' />
</head><body><p>Price: $15.00&nbsp;&nbsp;Length: 8:00</p></body></html>`

	var ts *httptest.Server
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/post-sitemap.xml":
			_, _ = fmt.Fprintf(w, sitemapXML, ts.URL, ts.URL)
		case r.URL.Path == "/post-sitemap2.xml":
			_, _ = w.Write([]byte(`<?xml version="1.0"?><urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"></urlset>`))
		case strings.Contains(r.URL.Path, "known-video"):
			_, _ = fmt.Fprintf(w, knownPage, ts.URL)
		case strings.Contains(r.URL.Path, "new-video"):
			_, _ = fmt.Fprintf(w, newPage, ts.URL)
		}
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client(), siteBase: ts.URL, headers: map[string]string{}}
	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{
		Workers:  1,
		KnownIDs: map[string]bool{"100": true},
	})
	if err != nil {
		t.Fatal(err)
	}

	scenes := testutil.CollectScenes(t, ch)

	if len(scenes) != 1 || scenes[0].ID != "200" {
		t.Errorf("got %d scenes, want [200] (known ID 100 should be skipped)", len(scenes))
	}
}

func fixedTime() time.Time {
	return time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC)
}
