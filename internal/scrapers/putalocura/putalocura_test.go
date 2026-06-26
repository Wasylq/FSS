package putalocura

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

const indexXML = `<?xml version="1.0" encoding="UTF-8"?>
<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <sitemap><loc>%s/sitemap-pages.xml</loc></sitemap>
  <sitemap><loc>%s/sitemap-scenes-es.xml</loc></sitemap>
</sitemapindex>`

const scenesXML = `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>%s/micro-escena/la-vecina-caliente</loc></url>
  <url><loc>%s/casting/primera-vez-laura</loc></url>
</urlset>`

const detailHTML = `<!DOCTYPE html>
<html><head></head><body>
<h1>
  <span class="model-name">La Vecina Caliente &amp; Torbe</span>
  <span class="dash">-</span>
  <span class="site-name">Nata Lee</span>
</h1>
<div class="released-views"><span>21/05/2023</span> - <span>28min</span></div>
<video><source src="https://sd.putalocura.com/trailers/vecina.mp4" type="video/mp4"></video>
<div class="desc-wrap"><p class="desc">Una <b>escena</b> increible.</p></div>
</body></html>`

// ---- TestMatchesURL ----

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url   string
		match bool
	}{
		{"https://www.putalocura.com/micro-escena/foo", true},
		{"http://putalocura.com/", true},
		{"https://example.com/", false},
		{"", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.match {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.match)
		}
	}
}

// ---- TestNormalizeURL ----

func TestNormalizeURL(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"https://www.www.putalocura.com/casting/x", "https://www.putalocura.com/casting/x"},
		{"  https://www.www.putalocura.com/y  ", "https://www.putalocura.com/y"},
		{"https://www.putalocura.com/z", "https://www.putalocura.com/z"},
	}
	for _, c := range cases {
		if got := normalizeURL(c.in); got != c.want {
			t.Errorf("normalizeURL(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// ---- TestSceneIDAndCategory ----

func TestSceneIDAndCategory(t *testing.T) {
	u := "https://www.putalocura.com/micro-escena/la-vecina-caliente"
	if got := sceneID(u); got != "la-vecina-caliente" {
		t.Errorf("sceneID = %q", got)
	}
	if got := sceneCategory(u); got != "micro-escena" {
		t.Errorf("sceneCategory = %q", got)
	}
}

// ---- TestParseDuration ----

func TestParseDuration(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"28min", 1680},
		{"1h 5min", 3900},
		{"2h", 7200},
		{"", 0},
	}
	for _, c := range cases {
		if got := parseDuration(c.in); got != c.want {
			t.Errorf("parseDuration(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

// ---- TestToScene (detail parser) ----

func TestToScene(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, detailHTML)
	}))
	defer ts.Close()

	s := New()
	now := time.Date(2026, 6, 25, 0, 0, 0, 0, time.UTC)
	sceneURL := ts.URL + "/micro-escena/la-vecina-caliente"
	scene, ok := s.toScene(context.Background(), "https://www.putalocura.com", sceneURL, now)
	if !ok {
		t.Fatal("toScene returned ok=false")
	}

	if scene.ID != "la-vecina-caliente" {
		t.Errorf("ID = %q", scene.ID)
	}
	if scene.SiteID != "putalocura" {
		t.Errorf("SiteID = %q", scene.SiteID)
	}
	if scene.Title != "La Vecina Caliente & Torbe" {
		t.Errorf("Title = %q", scene.Title)
	}
	if len(scene.Performers) != 1 || scene.Performers[0] != "Nata Lee" {
		t.Errorf("Performers = %v", scene.Performers)
	}
	if len(scene.Categories) != 1 || scene.Categories[0] != "micro-escena" {
		t.Errorf("Categories = %v", scene.Categories)
	}
	wantDate := time.Date(2023, 5, 21, 0, 0, 0, 0, time.UTC)
	if !scene.Date.Equal(wantDate) {
		t.Errorf("Date = %v, want %v", scene.Date, wantDate)
	}
	if scene.Duration != 1680 {
		t.Errorf("Duration = %d, want 1680", scene.Duration)
	}
	if scene.Preview != "https://sd.putalocura.com/trailers/vecina.mp4" {
		t.Errorf("Preview = %q", scene.Preview)
	}
	if scene.Thumbnail != scene.Preview {
		t.Errorf("Thumbnail = %q, want = Preview", scene.Thumbnail)
	}
	if scene.Description != "Una escena increible." {
		t.Errorf("Description = %q", scene.Description)
	}
}

// ---- TestToSceneNoTitle drops the scene ----

func TestToSceneNoTitle(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, "<html><body>no title markup</body></html>")
	}))
	defer ts.Close()

	s := New()
	if _, ok := s.toScene(context.Background(), "https://www.putalocura.com", ts.URL+"/x/y", time.Now()); ok {
		t.Error("expected ok=false for page with no title")
	}
}

// ---- TestListScenes end-to-end (sitemap index -> scenes -> details) ----

func TestListScenes(t *testing.T) {
	var base string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/sitemap.xml":
			_, _ = fmt.Fprintf(w, indexXML, base, base)
		case "/sitemap-scenes-es.xml":
			_, _ = fmt.Fprintf(w, scenesXML, base, base)
		default:
			_, _ = fmt.Fprint(w, detailHTML)
		}
	}))
	defer ts.Close()
	base = ts.URL

	old := siteBase
	siteBase = ts.URL
	defer func() { siteBase = old }()

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
			if r.Scene.Studio != "Puta Locura" {
				t.Errorf("Studio = %q", r.Scene.Studio)
			}
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
