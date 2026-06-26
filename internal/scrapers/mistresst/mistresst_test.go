package mistresst

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wasylq/FSS/scraper"
)

// ---- fixtures ----

const sitemapXML = `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>%s/content/front</loc></url>
  <url><loc>%s/content/humiliating-cuckold</loc></url>
  <url><loc>%s/content/humiliating-cuckold</loc></url>
  <url><loc>%s/taxonomy/term/5</loc></url>
  <url><loc>%s/content/findom-training</loc></url>
  <url><loc>%s/about-us</loc></url>
</urlset>`

const detailHTML = `<!DOCTYPE html>
<html><head>
<meta property="og:title" content="Humiliating Cuckold &amp; Friends" />
<meta property="og:description" content="A cruel cuckolding session." />
<meta property="og:url" content="https://www.mistresst.net/content/humiliating-cuckold" />
<title>Humiliating Cuckold | Mistress T</title>
</head><body>
<div class="field field-name-post-date">
  <div class="field-items"><div class="field-item even">Tue, 05/16/2023 - 12:30</div></div>
</div>
<div class="field field-name-field-video-category">
  <div class="field-items"><div class="field-item even"><a href="/category/femdom">Femdom</a></div></div>
</div>
<div class="field field-name-field-tags"><div class="field-items">
  <div class="field-item even"><a href="/tags/cuckold">Cuckold</a></div>
  <div class="field-item odd"><a href="/tags/humiliation">Humiliation</a></div>
  <div class="field-item even"><a href="/tags/cuckold">Cuckold</a></div>
</div></div></div>
<div class="field field-name-field-video-poster">
  <div class="field-items"><div class="field-item even"><img typeof="foaf:Image" src="https://cdn.mistresst.net/posters/cuckold.jpg" /></div></div>
</div>
</body></html>`

// ---- TestMatchesURL ----

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url   string
		match bool
	}{
		{"https://www.mistresst.net/content/foo", true},
		{"http://mistresst.net/", true},
		{"https://www.clips4sale.com/studio/123", false},
		{"", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.match {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.match)
		}
	}
}

// ---- TestSceneURLs ----

func TestSceneURLs(t *testing.T) {
	body := []byte(fmt.Sprintf(sitemapXML, "https://x", "https://x", "https://x", "https://x", "https://x", "https://x"))
	urls := sceneURLs(body)
	if len(urls) != 2 {
		t.Fatalf("got %d urls, want 2: %v", len(urls), urls)
	}
	want := map[string]bool{
		"https://x/content/humiliating-cuckold": true,
		"https://x/content/findom-training":     true,
	}
	for _, u := range urls {
		if !want[u] {
			t.Errorf("unexpected url %q", u)
		}
	}
}

// ---- TestFetchScene (detail parser) ----

func TestFetchScene(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, detailHTML)
	}))
	defer ts.Close()

	s := New()
	now := time.Date(2026, 6, 25, 0, 0, 0, 0, time.UTC)
	scene, err := s.fetchScene(context.Background(), "https://www.mistresst.net", ts.URL+"/content/humiliating-cuckold", now)
	if err != nil {
		t.Fatalf("fetchScene error: %v", err)
	}

	if scene.ID != "humiliating-cuckold" {
		t.Errorf("ID = %q", scene.ID)
	}
	if scene.SiteID != "mistresst" {
		t.Errorf("SiteID = %q", scene.SiteID)
	}
	if scene.Title != "Humiliating Cuckold & Friends" {
		t.Errorf("Title = %q", scene.Title)
	}
	if scene.URL != "https://www.mistresst.net/content/humiliating-cuckold" {
		t.Errorf("URL = %q", scene.URL)
	}
	if scene.Description != "A cruel cuckolding session." {
		t.Errorf("Description = %q", scene.Description)
	}
	wantDate := time.Date(2023, 5, 16, 12, 30, 0, 0, time.UTC)
	if !scene.Date.Equal(wantDate) {
		t.Errorf("Date = %v, want %v", scene.Date, wantDate)
	}
	if len(scene.Categories) != 1 || scene.Categories[0] != "Femdom" {
		t.Errorf("Categories = %v", scene.Categories)
	}
	if len(scene.Tags) != 2 || scene.Tags[0] != "Cuckold" || scene.Tags[1] != "Humiliation" {
		t.Errorf("Tags = %v, want [Cuckold Humiliation] (deduped)", scene.Tags)
	}
	if scene.Thumbnail != "https://cdn.mistresst.net/posters/cuckold.jpg" {
		t.Errorf("Thumbnail = %q", scene.Thumbnail)
	}
}

// ---- TestListScenes end-to-end via worker pool ----

func TestListScenes(t *testing.T) {
	var base string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/sitemap.xml":
			_, _ = fmt.Fprintf(w, sitemapXML, base, base, base, base, base, base)
		default:
			_, _ = fmt.Fprint(w, detailHTML)
		}
	}))
	defer ts.Close()
	base = ts.URL

	old := sitemapURL
	sitemapURL = ts.URL + "/sitemap.xml"
	defer func() { sitemapURL = old }()

	s := New()
	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{})
	if err != nil {
		t.Fatalf("ListScenes error: %v", err)
	}

	var scenes int
	for r := range ch {
		switch r.Kind {
		case scraper.KindScene:
			scenes++
			if r.Scene.Title == "" {
				t.Errorf("empty title")
			}
			if r.Scene.Studio != "Mistress T" {
				t.Errorf("Studio = %q", r.Scene.Studio)
			}
		case scraper.KindError:
			t.Errorf("unexpected error: %v", r.Err)
		}
	}
	if scenes != 2 {
		t.Errorf("got %d scenes, want 2", scenes)
	}
}
