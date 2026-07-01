package reflectivedesire

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wasylq/FSS/scraper"
)

func sitemapXML(base string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<urlset>
  <url><loc>%s/videos/</loc></url>
  <url><loc>%s/videos/pleasure/</loc></url>
  <url><loc>%s/videos/tickled-pink/</loc></url>
  <url><loc>%s/videos/self-restraint/</loc></url>
  <url><loc>%s/about/</loc></url>
</urlset>`, base, base, base, base, base)
}

func detailHTML(name, desc, thumb, dur, upload string, actors ...string) string {
	parts := make([]string, len(actors))
	for i, a := range actors {
		parts[i] = fmt.Sprintf(`{"@type":"Person","name":"%s"}`, a)
	}
	return fmt.Sprintf(`<html><head>
<meta property="og:title" content="%s">
<script type="application/ld+json">{"@context":"https://schema.org","@type":"VideoObject","name":"%s","description":"%s","thumbnailUrl":"%s","keywords":["latex","bondage","bdsm"],"duration":"%s","uploadDate":"%s","actor":[%s]}</script>
</head><body></body></html>`, name, name, desc, thumb, dur, upload, strings.Join(parts, ","))
}

func categoryHTML() string {
	return `<html><head><title>Category</title></head><body>no videoobject here</body></html>`
}

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := map[string]bool{
		"https://reflectivedesire.com/videos/tickled-pink/": true,
		"https://www.reflectivedesire.com/":                 true,
		"https://example.com/x":                             false,
		"":                                                  false,
	}
	for u, want := range cases {
		if got := s.MatchesURL(u); got != want {
			t.Errorf("MatchesURL(%q) = %v, want %v", u, got, want)
		}
	}
}

func TestParseDate(t *testing.T) {
	d := parseDate("2025-10-07T00:00:00.000Z")
	if d.Year() != 2025 || d.Month() != 10 || d.Day() != 7 {
		t.Errorf("parseDate millis = %v", d)
	}
	if !parseDate("").IsZero() {
		t.Errorf("empty should be zero")
	}
}

func TestFetchSitemapAndScene(t *testing.T) {
	orig, origSM := siteBase, sitemapURL
	defer func() { siteBase, sitemapURL = orig, origSM }()

	var base string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/sitemap.xml":
			_, _ = fmt.Fprint(w, sitemapXML(base))
		case strings.HasPrefix(r.URL.Path, "/videos/tickled-pink"):
			_, _ = fmt.Fprint(w, detailHTML("Tickled Pink", "A latex bondage video.",
				"https://images.reflectivedesire.com/t.jpg", "PT13M22S", "2025-10-07T00:00:00.000Z", "Vespa", "Succubunnyy"))
		case strings.HasPrefix(r.URL.Path, "/videos/self-restraint"):
			_, _ = fmt.Fprint(w, detailHTML("Self Restraint", "Solo bondage.",
				"https://images.reflectivedesire.com/s.jpg", "PT21M46S", "2024-05-01", "Vespa"))
		default:
			_, _ = fmt.Fprint(w, categoryHTML())
		}
	}))
	defer ts.Close()
	base = ts.URL
	siteBase = ts.URL
	sitemapURL = ts.URL + "/sitemap.xml"

	s := &Scraper{Client: ts.Client()}

	items, err := s.fetchSitemap(context.Background())
	if err != nil {
		t.Fatalf("fetchSitemap: %v", err)
	}
	// 2 real videos + 1 category page (/videos/pleasure/); /videos/ root and /about/ excluded.
	if len(items) != 3 {
		t.Fatalf("got %d items, want 3: %+v", len(items), items)
	}

	ch, err := s.ListScenes(context.Background(), "studioURL", scraper.ListOpts{})
	if err != nil {
		t.Fatalf("ListScenes: %v", err)
	}
	got := map[string]scraper.SceneResult{}
	for r := range ch {
		if r.Err != nil {
			t.Errorf("unexpected error: %v", r.Err)
			continue
		}
		if r.Kind == scraper.KindScene {
			got[r.Scene.ID] = r
		}
	}
	if len(got) != 2 {
		t.Fatalf("got %d scenes, want 2 (category dropped): %v keys", len(got), len(got))
	}
	sc := got["tickled-pink"].Scene
	if sc.Title != "Tickled Pink" {
		t.Errorf("Title = %q", sc.Title)
	}
	if sc.Duration != 13*60+22 {
		t.Errorf("Duration = %d, want 802", sc.Duration)
	}
	if sc.Studio != studioName {
		t.Errorf("Studio = %q", sc.Studio)
	}
	if len(sc.Performers) != 2 || sc.Performers[0] != "Vespa" {
		t.Errorf("Performers = %v", sc.Performers)
	}
	if sc.Date.Year() != 2025 || sc.Date.Month() != 10 || sc.Date.Day() != 7 {
		t.Errorf("Date = %v", sc.Date)
	}
}
