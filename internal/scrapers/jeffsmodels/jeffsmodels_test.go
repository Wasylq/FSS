package jeffsmodels

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wasylq/FSS/scraper"
)

// ---- fixtures ----

func sitemapXML(base string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>%s/</loc></url>
  <url><loc>%s/updates/</loc></url>
  <url>
    <loc>%s/update/93/</loc>
    <lastmod>2016-12-26T11:21:26+00:00</lastmod>
  </url>
  <url>
    <loc>%s/update/92/</loc>
    <lastmod>2016-12-28T11:19:39+00:00</lastmod>
  </url>
  <url>
    <loc>%s/update/93/</loc>
    <lastmod>2016-12-26T11:21:26+00:00</lastmod>
  </url>
</urlset>`, base, base, base, base, base)
}

func detailHTML() string {
	return `<html><head><title>Busty First Timer Banged - Sinful Samia</title></head><body>
<h1>Busty First Timer Banged</h1>
<img src="https://media.example.com/updates/0093/01.jpg" class="video-banner" alt="preview image">
<h4><a href="/models/32/" class="model-name no-text-decoration female">Sinful Samia</a>
<a href="/models/17/" class="model-name female">Danni Dawson</a>
<small class="updated-at">Dec 26, 2016</small></h4>
<p class="read-more">Samia is a gorgeous, zaftig, blond, tattooed chubby beauty &amp; friend.</p>
<div class="related"><a href="/models/80/" class="item-talent female">Stazi</a></div>
</body></html>`
}

// ---- TestMatchesURL ----

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url   string
		match bool
	}{
		{"https://jeffsmodels.com/", true},
		{"https://www.jeffsmodels.com/update/93/", true},
		{"http://jeffsmodels.com/update/93/", true},
		{"https://example.com/x", false},
		{"", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.match {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.match)
		}
	}
}

// ---- TestCleanText ----

func TestCleanText(t *testing.T) {
	got := cleanText("  <span>Hello&amp;</span>   World  ")
	if got != "Hello& World" {
		t.Errorf("cleanText = %q, want %q", got, "Hello& World")
	}
}

// ---- TestFetchSitemap ----

func TestFetchSitemap(t *testing.T) {
	var ts *httptest.Server
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, sitemapXML(ts.URL))
	}))
	defer ts.Close()

	orig := sitemapURL
	defer func() { sitemapURL = orig }()
	sitemapURL = ts.URL + "/sitemap.xml"

	s := &Scraper{Client: ts.Client()}
	items, err := s.fetchSitemap(context.Background())
	if err != nil {
		t.Fatalf("fetchSitemap error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2 (dup dropped): %+v", len(items), items)
	}
	if items[0].id != "93" {
		t.Errorf("item0.id = %q, want 93", items[0].id)
	}
	if items[0].lastmod.IsZero() {
		t.Errorf("item0 lastmod is zero")
	}
}

// ---- TestListScenes (end-to-end) ----

func TestListScenes(t *testing.T) {
	var ts *httptest.Server
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/sitemap.xml") {
			_, _ = fmt.Fprint(w, sitemapXML(ts.URL))
			return
		}
		_, _ = fmt.Fprint(w, detailHTML())
	}))
	defer ts.Close()

	origSitemap, origBase := sitemapURL, siteBase
	defer func() { sitemapURL, siteBase = origSitemap, origBase }()
	sitemapURL = ts.URL + "/sitemap.xml"
	siteBase = ts.URL

	s := &Scraper{Client: ts.Client()}
	ch, err := s.ListScenes(context.Background(), "studioURL", scraper.ListOpts{})
	if err != nil {
		t.Fatalf("ListScenes error: %v", err)
	}
	got := map[string]string{}
	var sample = map[string]struct {
		performers []string
		thumb      string
		desc       string
	}{}
	for r := range ch {
		if r.Err != nil {
			t.Errorf("unexpected error: %v", r.Err)
			continue
		}
		if r.Kind == scraper.KindScene {
			got[r.Scene.ID] = r.Scene.Title
			sample[r.Scene.ID] = struct {
				performers []string
				thumb      string
				desc       string
			}{r.Scene.Performers, r.Scene.Thumbnail, r.Scene.Description}
		}
	}
	if len(got) != 2 {
		t.Fatalf("got %d scenes, want 2: %v", len(got), got)
	}
	if got["93"] != "Busty First Timer Banged" {
		t.Errorf("scene 93 title = %q", got["93"])
	}
	s93 := sample["93"]
	if strings.Join(s93.performers, ",") != "Sinful Samia,Danni Dawson" {
		t.Errorf("scene 93 performers = %v", s93.performers)
	}
	if s93.thumb != "https://media.example.com/updates/0093/01.jpg" {
		t.Errorf("scene 93 thumb = %q", s93.thumb)
	}
	if !strings.HasPrefix(s93.desc, "Samia is a gorgeous") {
		t.Errorf("scene 93 desc = %q", s93.desc)
	}
}
