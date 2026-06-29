package swearlutil

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/Wasylq/FSS/scraper"
)

// listJSON serves one page of the videos API with two items and pages=1.
const listJSON = `{
  "status": { "code": 1, "message": "Ok" },
  "data": {
    "pages": 1,
    "items": [
      {
        "id": "6a312f19377556f7c8057a82",
        "title": "Nymph &amp; Boutique",
        "slug": "nymph-boutique",
        "description": "<p>A <a href=\"/x\">taboo</a> &amp; fancy story.</p>",
        "publishedAt": 1781852460,
        "viewCount": 1804,
        "videoSettings": { "duration": 3799 },
        "models": [
          { "slug": "christina-sage", "title": "Christina Sage" },
          { "slug": "della-cate", "title": "Della Cate" }
        ],
        "poster": { "permalink": "/uploads/2026/06/poster1.jpg" }
      },
      {
        "id": "5678",
        "title": "Second Scene",
        "slug": "second-scene",
        "publishedAt": 1700000000,
        "videoSettings": { "duration": 1200 },
        "models": [],
        "poster": { "permalink": "" }
      }
    ]
  }
}`

const detailJSON = `{
  "status": { "code": 1, "message": "Ok" },
  "data": {
    "item": {
      "categories": [
        { "slug": "babe", "name": "Babe" },
        { "slug": "blonde", "name": "Blonde" }
      ]
    }
  }
}`

func testConfig() SiteConfig {
	return SiteConfig{
		ID:          "vrbangers",
		ContentHost: "content.vrbangers.com",
		SiteBase:    "https://vrbangers.com",
		Studio:      "VR Bangers",
		MatchRe:     regexp.MustCompile(`^https?://(?:www\.)?vrbangers\.com`),
	}
}

func newTestServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/content/v1/videos":
			if r.URL.Query().Get("page") == "1" {
				_, _ = fmt.Fprint(w, listJSON)
				return
			}
			_, _ = fmt.Fprint(w, `{"data":{"pages":1,"items":[]}}`)
		case r.URL.Path == "/api/content/v1/videos/nymph-boutique":
			_, _ = fmt.Fprint(w, detailJSON)
		default:
			http.NotFound(w, r)
		}
	}))
}

func drain(t *testing.T, srv *httptest.Server) ([]scraper.SceneResult, int) {
	t.Helper()
	s := New(testConfig())
	s.APIBase = srv.URL
	s.Client = srv.Client()

	ch, err := s.ListScenes(context.Background(), "https://vrbangers.com", scraper.ListOpts{})
	if err != nil {
		t.Fatalf("ListScenes: %v", err)
	}
	var scenes []scraper.SceneResult
	var total int
	for r := range ch {
		switch r.Kind {
		case scraper.KindScene:
			scenes = append(scenes, r)
		case scraper.KindTotal:
			total = r.Total
		case scraper.KindError:
			t.Errorf("unexpected error: %v", r.Err)
		}
	}
	return scenes, total
}

func TestListScenes_endToEnd(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	scenes, total := drain(t, srv)
	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2", len(scenes))
	}
	if total != pageSize { // pages(1) * pageSize
		t.Errorf("total = %d, want %d", total, pageSize)
	}

	first := scenes[0].Scene
	if first.ID != "6a312f19377556f7c8057a82" {
		t.Errorf("ID = %q", first.ID)
	}
	if first.SiteID != "vrbangers" {
		t.Errorf("SiteID = %q", first.SiteID)
	}
	if first.Title != "Nymph & Boutique" {
		t.Errorf("Title = %q (want HTML-unescaped)", first.Title)
	}
	if first.URL != "https://vrbangers.com/video/nymph-boutique/" {
		t.Errorf("URL = %q", first.URL)
	}
	if first.Studio != "VR Bangers" {
		t.Errorf("Studio = %q", first.Studio)
	}
	if first.Duration != 3799 {
		t.Errorf("Duration = %d, want 3799", first.Duration)
	}
	if first.Views != 1804 {
		t.Errorf("Views = %d, want 1804", first.Views)
	}
	// publishedAt 1781852460 -> 2026-06-19T...
	if first.Date.IsZero() || first.Date.Year() != 2026 {
		t.Errorf("Date = %v, want 2026", first.Date)
	}
	if len(first.Performers) != 2 || first.Performers[0] != "Christina Sage" || first.Performers[1] != "Della Cate" {
		t.Errorf("Performers = %v", first.Performers)
	}
	if first.Thumbnail != srv.URL+"/uploads/2026/06/poster1.jpg" {
		t.Errorf("Thumbnail = %q", first.Thumbnail)
	}
	if first.Description != "A taboo & fancy story." {
		t.Errorf("Description = %q (want HTML stripped/unescaped)", first.Description)
	}
	// categories come from the best-effort detail fetch
	if len(first.Categories) != 2 || first.Categories[0] != "Babe" || first.Categories[1] != "Blonde" {
		t.Errorf("Categories = %v", first.Categories)
	}

	// Second scene: no models, empty poster, detail 404 -> no categories.
	second := scenes[1].Scene
	if second.ID != "5678" {
		t.Errorf("second ID = %q", second.ID)
	}
	if len(second.Performers) != 0 {
		t.Errorf("second Performers = %v, want none", second.Performers)
	}
	if second.Thumbnail != "" {
		t.Errorf("second Thumbnail = %q, want empty", second.Thumbnail)
	}
	if len(second.Categories) != 0 {
		t.Errorf("second Categories = %v, want none (detail 404)", second.Categories)
	}
}

func TestPaginationDone(t *testing.T) {
	// pages=1 means the loop must stop after page 1; if Done weren't set the
	// loop would request page 2 (served as empty) which also stops it, but we
	// assert page 2 is never requested.
	var hits []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/content/v1/videos" {
			hits = append(hits, r.URL.Query().Get("page"))
			_, _ = fmt.Fprint(w, listJSON)
			return
		}
		if r.URL.Path == "/api/content/v1/videos/nymph-boutique" {
			_, _ = fmt.Fprint(w, detailJSON)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	scenes, _ := drain(t, srv)
	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2", len(scenes))
	}
	for _, p := range hits {
		if p != "1" {
			t.Errorf("requested page %q; want only page 1 (Done not honored)", p)
		}
	}
}

func TestMatchesURL(t *testing.T) {
	s := New(testConfig())
	cases := []struct {
		url   string
		match bool
	}{
		{"https://vrbangers.com/", true},
		{"https://www.vrbangers.com/video/foo/", true},
		{"http://vrbangers.com", true},
		{"https://vrbtrans.com/", false},
		{"", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.match {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.match)
		}
	}
}

func TestPatterns(t *testing.T) {
	s := New(testConfig())
	p := s.Patterns()
	if len(p) != 2 || p[0] != "vrbangers.com" || p[1] != "vrbangers.com/video/{slug}" {
		t.Errorf("Patterns = %v", p)
	}
}
