package wputil

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSlugFromURL(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"https://example.com/post/my-scene/", "my-scene"},
		{"https://example.com/post/my-scene", "my-scene"},
		{"https://example.com/post/my-scene.html", "my-scene"},
		{"single-segment", "single-segment"},
	}
	for _, c := range cases {
		if got := SlugFromURL(c.in); got != c.want {
			t.Errorf("SlugFromURL(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestParseDuration(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"00:30", 30},
		{"01:30", 90},
		{"15:45", 945},
		{"1:00:00", 3600},
		{"2:30:00", 9000},
		{"", 0},
		{"abc", 0},
	}
	for _, c := range cases {
		if got := ParseDuration(c.in); got != c.want {
			t.Errorf("ParseDuration(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestVideoWidth(t *testing.T) {
	cases := []struct {
		height, want int
	}{
		{2160, 3840},
		{2400, 3840},
		{1080, 1920},
		{1440, 1920},
		{720, 1280},
		{900, 1280},
		{480, 854},
		{600, 854},
		{360, 0},
		{0, 0},
	}
	for _, c := range cases {
		if got := VideoWidth(c.height); got != c.want {
			t.Errorf("VideoWidth(%d) = %d, want %d", c.height, got, c.want)
		}
	}
}

func TestBrowserHeaders(t *testing.T) {
	h := BrowserHeaders()
	for _, k := range []string{"User-Agent", "Accept", "Accept-Language"} {
		if h[k] == "" {
			t.Errorf("missing header %q", k)
		}
	}
}

func TestParseMeta_full(t *testing.T) {
	body := []byte(`<html>
<head>
<title>Sample Scene Title - Mom Comes First</title>
<meta property="article:published_time" content="2026-01-15T12:00:00+00:00" />
<meta property="og:description" content="A &amp; sample description." />
<meta property="og:image" content="https://cdn.example/cover.jpg" />
<meta property="article:tag" content="POV" />
<meta property="article:tag" content="MILF" />
<meta property="article:tag" content="POV" />
<link rel='shortlink' href='https://example.com/?p=12345' />
<script type="application/ld+json">
{
  "@type": "VideoObject",
  "name": "x",
  "width": "1920",
  "height": "1080"
}
</script>
<script type="application/ld+json">
{ "articleSection": "Drama, Roleplay" }
</script>
</head>
<body></body>
</html>`)

	m := ParseMeta(body, " - Mom Comes First")

	if m.Title != "Sample Scene Title" {
		t.Errorf("Title = %q", m.Title)
	}
	want := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	if !m.Date.Equal(want) {
		t.Errorf("Date = %v, want %v", m.Date, want)
	}
	if m.Description != "A & sample description." {
		t.Errorf("Description = %q", m.Description)
	}
	if m.Thumbnail != "https://cdn.example/cover.jpg" {
		t.Errorf("Thumbnail = %q", m.Thumbnail)
	}
	if m.PostID != "12345" {
		t.Errorf("PostID = %q", m.PostID)
	}
	if len(m.Tags) != 2 || m.Tags[0] != "POV" || m.Tags[1] != "MILF" {
		t.Errorf("Tags = %v (expected dedup)", m.Tags)
	}
	if len(m.Categories) != 2 || m.Categories[0] != "Drama" || m.Categories[1] != "Roleplay" {
		t.Errorf("Categories = %v", m.Categories)
	}
	if m.Width != 1920 || m.Height != 1080 {
		t.Errorf("Width/Height = %d/%d", m.Width, m.Height)
	}
}

func TestParseMeta_empty(t *testing.T) {
	m := ParseMeta([]byte(`<html><body>nothing here</body></html>`), "")
	if m.Title != "" || !m.Date.IsZero() || m.Description != "" || m.Thumbnail != "" ||
		m.PostID != "" || len(m.Tags) != 0 || len(m.Categories) != 0 ||
		m.Width != 0 || m.Height != 0 {
		t.Errorf("expected zero-value Meta, got %+v", m)
	}
}

func TestParseMeta_badDate(t *testing.T) {
	body := []byte(`<meta property="article:published_time" content="not-a-date" />`)
	m := ParseMeta(body, "")
	if !m.Date.IsZero() {
		t.Errorf("expected zero Date for bad input, got %v", m.Date)
	}
}

func TestParseMeta_titleSuffixStripping(t *testing.T) {
	body := []byte(`<title>My Scene</title>`)
	m := ParseMeta(body, " - Suffix")
	if m.Title != "My Scene" {
		t.Errorf("Title = %q", m.Title)
	}

	body = []byte(`<title>My Scene - Suffix</title>`)
	m = ParseMeta(body, " - Suffix")
	if m.Title != "My Scene" {
		t.Errorf("Title with suffix stripped = %q", m.Title)
	}
}

func TestFetchSitemap(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>https://example.com/scene-one</loc></url>
  <url><loc>https://example.com/scene-two</loc></url>
</urlset>`))
	}))
	defer ts.Close()

	urls, err := FetchSitemap(context.Background(), ts.Client(), ts.URL, BrowserHeaders())
	if err != nil {
		t.Fatalf("FetchSitemap error: %v", err)
	}
	if len(urls) != 2 {
		t.Fatalf("got %d URLs, want 2", len(urls))
	}
	if urls[0].Loc != "https://example.com/scene-one" {
		t.Errorf("urls[0] = %+v", urls[0])
	}
}

func TestFetchSitemap_badXML(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`not xml`))
	}))
	defer ts.Close()

	if _, err := FetchSitemap(context.Background(), ts.Client(), ts.URL, nil); err == nil {
		t.Error("expected error on malformed XML")
	}
}

func TestFetchAllSitemaps(t *testing.T) {
	page := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		page++
		_, _ = fmt.Fprintf(w, `<urlset><url><loc>https://example.com/p%d</loc></url></urlset>`, page)
	}))
	defer ts.Close()

	urls, err := FetchAllSitemaps(context.Background(), ts.Client(), []string{ts.URL, ts.URL}, nil)
	if err != nil {
		t.Fatalf("FetchAllSitemaps error: %v", err)
	}
	if len(urls) != 2 {
		t.Fatalf("got %d URLs, want 2", len(urls))
	}
}

func TestFetchPage(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("<html>hello</html>"))
	}))
	defer ts.Close()

	body, err := FetchPage(context.Background(), ts.Client(), ts.URL, nil)
	if err != nil {
		t.Fatalf("FetchPage error: %v", err)
	}
	if string(body) != "<html>hello</html>" {
		t.Errorf("body = %q", body)
	}
}
