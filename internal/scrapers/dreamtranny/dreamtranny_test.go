package dreamtranny

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
  <url>
    <loc>%s/update/101/</loc>
    <lastmod>2023-05-16T00:00:00+00:00</lastmod>
  </url>
  <url>
    <loc>%s/update/102/</loc>
    <lastmod>2023-06-20T00:00:00+00:00</lastmod>
  </url>
  <url>
    <loc>%s/categories/anal/</loc>
    <lastmod>2023-01-01T00:00:00+00:00</lastmod>
  </url>
</urlset>`

const detailHTML = `<!DOCTYPE html>
<html><head>
<title>Hot POV Scene - Jane Doe | Dream Tranny</title>
<meta name="description" content="Fallback meta description." />
</head><body>
<a href="/model/jane-doe" class="model-name link">Jane Doe</a>
<a href="/model/jane-doe" class="model-name link">Jane Doe</a>
<a href="/model/mia-lux" class="model-name link">Mia Lux</a>
<img src="https://cdn.roguebucks.com/covers/101.jpg" class="video-banner" />
<p class="read-more">A <b>steamy</b> on-page synopsis here.</p>
<span class="updated-at">Jun 18, 2023</span>
</body></html>`

// ---- TestMatchesURL ----

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url   string
		match bool
	}{
		{"https://dreamtranny.com/update/101/", true},
		{"http://www.dreamtranny.com/", true},
		{"https://example.com/", false},
		{"", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.match {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.match)
		}
	}
}

// ---- TestFetchSitemap ----

func TestFetchSitemap(t *testing.T) {
	var base string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, sitemapXML, base, base, base)
	}))
	defer ts.Close()
	base = ts.URL

	old := sitemapURL
	sitemapURL = ts.URL + "/sitemap.xml"
	defer func() { sitemapURL = old }()

	s := New()
	items, err := s.fetchSitemap(context.Background())
	if err != nil {
		t.Fatalf("fetchSitemap error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2 (non-/update dropped): %v", len(items), items)
	}
	if items[0].id != "101" {
		t.Errorf("items[0].id = %q, want 101", items[0].id)
	}
	wantLM := time.Date(2023, 5, 16, 0, 0, 0, 0, time.UTC)
	if !items[0].lastmod.Equal(wantLM) {
		t.Errorf("items[0].lastmod = %v, want %v", items[0].lastmod, wantLM)
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
	it := sitemapItem{id: "101", url: ts.URL + "/update/101/", lastmod: time.Date(2023, 5, 16, 0, 0, 0, 0, time.UTC)}
	scene, err := s.fetchScene(context.Background(), "https://dreamtranny.com", it, now)
	if err != nil {
		t.Fatalf("fetchScene error: %v", err)
	}

	if scene.ID != "101" {
		t.Errorf("ID = %q", scene.ID)
	}
	if scene.SiteID != "dreamtranny" {
		t.Errorf("SiteID = %q", scene.SiteID)
	}
	if scene.Title != "Hot POV Scene" {
		t.Errorf("Title = %q, want %q (suffix+performer stripped)", scene.Title, "Hot POV Scene")
	}
	if len(scene.Performers) != 2 || scene.Performers[0] != "Jane Doe" || scene.Performers[1] != "Mia Lux" {
		t.Errorf("Performers = %v, want [Jane Doe Mia Lux] (deduped)", scene.Performers)
	}
	if scene.Description != "A steamy on-page synopsis here." {
		t.Errorf("Description = %q", scene.Description)
	}
	if scene.Thumbnail != "https://cdn.roguebucks.com/covers/101.jpg" {
		t.Errorf("Thumbnail = %q", scene.Thumbnail)
	}
	// On-page updated-at overrides the sitemap lastmod.
	wantDate := time.Date(2023, 6, 18, 0, 0, 0, 0, time.UTC)
	if !scene.Date.Equal(wantDate) {
		t.Errorf("Date = %v, want %v (updated-at)", scene.Date, wantDate)
	}
}

// ---- TestFetchScene date falls back to sitemap lastmod ----

func TestFetchSceneDateFallback(t *testing.T) {
	const noDateHTML = `<html><head><title>Plain - Model | Dream Tranny</title></head><body></body></html>`
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, noDateHTML)
	}))
	defer ts.Close()

	s := New()
	lm := time.Date(2022, 3, 4, 0, 0, 0, 0, time.UTC)
	it := sitemapItem{id: "9", url: ts.URL + "/update/9/", lastmod: lm}
	scene, err := s.fetchScene(context.Background(), "https://dreamtranny.com", it, time.Now())
	if err != nil {
		t.Fatalf("fetchScene error: %v", err)
	}
	if !scene.Date.Equal(lm) {
		t.Errorf("Date = %v, want %v (sitemap lastmod fallback)", scene.Date, lm)
	}
}

// ---- TestListScenes end-to-end ----

func TestListScenes(t *testing.T) {
	var base string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/sitemap.xml":
			_, _ = fmt.Fprintf(w, sitemapXML, base, base, base)
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
		case scraper.KindError:
			t.Errorf("unexpected error: %v", r.Err)
		}
	}
	if scenes != 2 {
		t.Errorf("got %d scenes, want 2", scenes)
	}
}
