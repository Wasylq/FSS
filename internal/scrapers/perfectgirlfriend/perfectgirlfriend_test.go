package perfectgirlfriend

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wasylq/FSS/scraper"
)

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url  string
		want bool
	}{
		{"https://perfectgirlfriend.com", true},
		{"https://www.perfectgirlfriend.com/some-scene/", true},
		{"https://familytherapyxxx.com", false},
		{"https://example.com", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.want {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.want)
		}
	}
}

const fixtureVideoPage = `<!doctype html><html><head>
<title>Good Morning With Girlfriends | Perfect Girlfriend</title>
<link rel='shortlink' href='https://perfectgirlfriend.com/?p=3176' />
<script type="application/ld+json">{"@type":"VideoObject","name":"Good Morning With Girlfriends","description":"A romantic morning.","thumbnailUrl":"https://cdn.example/thumb.jpg","uploadDate":"2026-04-18T08:00:00+00:00","contentUrl":"https://cdn.example/video.mp4"}</script>
</head><body></body></html>`

func TestParsePage(t *testing.T) {
	scene, skip, err := parsePage(
		"https://perfectgirlfriend.com",
		"https://perfectgirlfriend.com/good-morning-with-girlfriends/",
		[]byte(fixtureVideoPage),
		fixedTime(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if skip {
		t.Fatal("expected video page, got skip")
	}

	if scene.ID != "3176" {
		t.Errorf("ID = %q, want 3176", scene.ID)
	}
	if scene.Title != "Good Morning With Girlfriends" {
		t.Errorf("Title = %q", scene.Title)
	}
	if scene.Description != "A romantic morning." {
		t.Errorf("Description = %q", scene.Description)
	}
	if scene.Thumbnail != "https://cdn.example/thumb.jpg" {
		t.Errorf("Thumbnail = %q", scene.Thumbnail)
	}
	if scene.Date.Year() != 2026 || scene.Date.Month() != 4 || scene.Date.Day() != 18 {
		t.Errorf("Date = %v", scene.Date)
	}
	if scene.SiteID != "perfectgirlfriend" {
		t.Errorf("SiteID = %q", scene.SiteID)
	}
	if scene.Studio != "Perfect Girlfriend" {
		t.Errorf("Studio = %q", scene.Studio)
	}
}

const fixtureNonVideoPage = `<!doctype html><html><head>
<title>Perfect Girlfriend</title>
<link rel='shortlink' href='https://perfectgirlfriend.com/?p=1' />
</head><body><p>Homepage.</p></body></html>`

func TestParsePageSkipsNonVideo(t *testing.T) {
	_, skip, err := parsePage(
		"https://perfectgirlfriend.com",
		"https://perfectgirlfriend.com/",
		[]byte(fixtureNonVideoPage),
		fixedTime(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if !skip {
		t.Error("expected skip=true for non-video page")
	}
}

func TestListScenes(t *testing.T) {
	sitemapXML := `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>%s/</loc></url>
  <url><loc>%s/good-morning/</loc></url>
</urlset>`

	videoPage := `<html><head>
<title>Good Morning | Perfect Girlfriend</title>
<link rel='shortlink' href='%s/?p=3176' />
<script type="application/ld+json">{"@type":"VideoObject","name":"Good Morning","description":"Desc.","thumbnailUrl":"https://cdn.example/t.jpg","uploadDate":"2026-04-18T08:00:00+00:00"}</script>
</head><body></body></html>`

	homepage := `<html><head><title>Perfect Girlfriend</title></head>
<body><p>Welcome.</p></body></html>`

	var ts *httptest.Server
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/sitemap.xml":
			_, _ = fmt.Fprintf(w, sitemapXML, ts.URL, ts.URL)
		case strings.Contains(r.URL.Path, "good-morning"):
			_, _ = fmt.Fprintf(w, videoPage, ts.URL)
		default:
			_, _ = w.Write([]byte(homepage))
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

	var scenes []string
	for r := range ch {
		if r.Kind == scraper.KindTotal || r.Kind == scraper.KindStoppedEarly {
			continue
		}
		if r.Err != nil {
			t.Errorf("unexpected error: %v", r.Err)
			continue
		}
		scenes = append(scenes, r.Scene.Title)
	}

	if len(scenes) != 1 {
		t.Fatalf("got %d scenes, want 1: %v", len(scenes), scenes)
	}
	if scenes[0] != "Good Morning" {
		t.Errorf("scene title = %q, want Good Morning", scenes[0])
	}
}

func fixedTime() time.Time {
	return time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
}
