package fotoroutil

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/scraper"
)

// Fixtures shaped to the WP REST payloads fotoroutil consumes.

const tagsJSON = `[
  {"id":5,"name":"Jane Doe"},
  {"id":6,"name":"John &amp; Smith"}
]`

const categoriesJSON = `[
  {"id":10,"name":"Bondage"},
  {"id":11,"name":"Uncategorized"}
]`

const postsJSON = `[
  {
    "id":101,
    "date":"2026-01-15T12:30:00",
    "link":"https://example.com/?p=101",
    "title":{"rendered":"First &amp; Best Scene"},
    "excerpt":{"rendered":"<p>An  excerpt with <b>markup</b> &amp; spaces.</p>"},
    "content":{"rendered":"<p>body</p>"},
    "tags":[5,6],
    "categories":[10,11],
    "jetpack_featured_media_url":"https://cdn.example.com/img/101.jpg"
  },
  {
    "id":102,
    "date":"2026-01-10T08:00:00",
    "link":"/scenes/second.html",
    "title":{"rendered":"Second Scene"},
    "excerpt":{"rendered":""},
    "content":{"rendered":"<div><img src=\"https://cdn.example.com/img/from-content.jpg\" /></div>"},
    "tags":[5],
    "categories":[10],
    "jetpack_featured_media_url":""
  }
]`

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/wp-json/wp/v2/tags":
			// resolveTagID hits the same path with a slug query param.
			if slug := r.URL.Query().Get("slug"); slug != "" {
				if slug == "jane-doe" {
					_, _ = fmt.Fprint(w, `[{"id":5}]`)
				} else {
					_, _ = fmt.Fprint(w, `[]`)
				}
				return
			}
			_, _ = fmt.Fprint(w, tagsJSON)
		case "/wp-json/wp/v2/categories":
			_, _ = fmt.Fprint(w, categoriesJSON)
		case "/wp-json/wp/v2/posts":
			w.Header().Set("X-WP-Total", "2")
			_, _ = fmt.Fprint(w, postsJSON)
		default:
			http.NotFound(w, r)
		}
	}))
}

func drain(t *testing.T, ch <-chan scraper.SceneResult) []models.Scene {
	t.Helper()
	var scenes []models.Scene
	for r := range ch {
		switch r.Kind {
		case scraper.KindScene:
			scenes = append(scenes, r.Scene)
		case scraper.KindError:
			t.Errorf("unexpected error: %v", r.Err)
		}
	}
	return scenes
}

func TestListScenes_tagsAsPerformers(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	s := New(SiteConfig{
		ID:       "fototest",
		Studio:   "Foto Test",
		SiteBase: ts.URL,
		MatchRe:  regexp.MustCompile(`.*`),
	})

	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}
	scenes := drain(t, ch)
	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2", len(scenes))
	}

	first := scenes[0]
	if first.ID != "101" {
		t.Errorf("ID = %q, want 101", first.ID)
	}
	if first.Title != "First & Best Scene" {
		t.Errorf("Title = %q", first.Title)
	}
	if first.SiteID != "fototest" {
		t.Errorf("SiteID = %q", first.SiteID)
	}
	if first.Studio != "Foto Test" {
		t.Errorf("Studio = %q", first.Studio)
	}
	if first.Date.Year() != 2026 || first.Date.Month() != 1 || first.Date.Day() != 15 {
		t.Errorf("Date = %v, want 2026-01-15", first.Date)
	}
	// tags → performers (TagsAsTags=false), with HTML unescaping.
	if len(first.Performers) != 2 || first.Performers[0] != "Jane Doe" || first.Performers[1] != "John & Smith" {
		t.Errorf("Performers = %v", first.Performers)
	}
	if len(first.Tags) != 0 {
		t.Errorf("Tags = %v, want empty", first.Tags)
	}
	// "Uncategorized" is filtered out.
	if len(first.Categories) != 1 || first.Categories[0] != "Bondage" {
		t.Errorf("Categories = %v", first.Categories)
	}
	if first.Thumbnail != "https://cdn.example.com/img/101.jpg" {
		t.Errorf("Thumbnail = %q", first.Thumbnail)
	}
	if first.Description != "An excerpt with markup & spaces." {
		t.Errorf("Description = %q", first.Description)
	}
	if first.URL != "https://example.com/?p=101" {
		t.Errorf("URL = %q", first.URL)
	}

	second := scenes[1]
	// relative link resolves against SiteBase.
	if second.URL != ts.URL+"/scenes/second.html" {
		t.Errorf("second URL = %q", second.URL)
	}
	// thumbnail falls back to first <img> in content.
	if second.Thumbnail != "https://cdn.example.com/img/from-content.jpg" {
		t.Errorf("second Thumbnail = %q", second.Thumbnail)
	}
}

func TestListScenes_tagsAsTags(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	s := New(SiteConfig{
		ID:         "fototest",
		Studio:     "Foto Test",
		SiteBase:   ts.URL,
		TagsAsTags: true,
		MatchRe:    regexp.MustCompile(`.*`),
	})

	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}
	scenes := drain(t, ch)
	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2", len(scenes))
	}
	first := scenes[0]
	if len(first.Tags) != 2 || first.Tags[0] != "Jane Doe" || first.Tags[1] != "John & Smith" {
		t.Errorf("Tags = %v", first.Tags)
	}
	if len(first.Performers) != 0 {
		t.Errorf("Performers = %v, want empty", first.Performers)
	}
}

func TestListScenes_tagPageFilter(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	s := New(SiteConfig{
		ID:       "fototest",
		Studio:   "Foto Test",
		SiteBase: ts.URL,
		MatchRe:  regexp.MustCompile(`.*`),
	})

	tagURL := ts.URL + "/tag/jane-doe/"
	ch, err := s.ListScenes(context.Background(), tagURL, scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}
	scenes := drain(t, ch)
	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2", len(scenes))
	}
	if scenes[0].StudioURL != tagURL {
		t.Errorf("StudioURL = %q, want %q", scenes[0].StudioURL, tagURL)
	}
}

func TestListScenes_tagPageNotFound(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	s := New(SiteConfig{
		ID:       "fototest",
		Studio:   "Foto Test",
		SiteBase: ts.URL,
		MatchRe:  regexp.MustCompile(`.*`),
	})

	ch, err := s.ListScenes(context.Background(), ts.URL+"/tag/missing/", scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}
	var gotErr bool
	for r := range ch {
		if r.Kind == scraper.KindError {
			gotErr = true
		}
	}
	if !gotErr {
		t.Error("expected an error for an unknown tag slug")
	}
}

func TestPostToScene_dateAndDefaults(t *testing.T) {
	s := New(SiteConfig{ID: "x", Studio: "X", SiteBase: "https://x.test", MatchRe: regexp.MustCompile(`.*`)})
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	p := wpPost{
		ID:    7,
		Date:  "2025-12-31T23:59:59",
		Link:  "https://x.test/s/7",
		Title: wpRendered{Rendered: "Title &amp; Co"},
		Tags:  []int{1},
	}
	tagMap := map[int]string{1: "Performer One"}
	scene := s.postToScene("https://x.test", p, tagMap, map[int]string{}, now)
	if scene.ID != "7" {
		t.Errorf("ID = %q", scene.ID)
	}
	if scene.Title != "Title & Co" {
		t.Errorf("Title = %q", scene.Title)
	}
	if scene.Date.Year() != 2025 || scene.Date.Day() != 31 {
		t.Errorf("Date = %v", scene.Date)
	}
	if scene.ScrapedAt != now {
		t.Errorf("ScrapedAt = %v", scene.ScrapedAt)
	}
	if len(scene.Performers) != 1 || scene.Performers[0] != "Performer One" {
		t.Errorf("Performers = %v", scene.Performers)
	}
}

func TestCleanText(t *testing.T) {
	cases := []struct{ in, want string }{
		{"<p>Hello   world</p>", "Hello world"},
		{"a &amp; b", "a & b"},
		{"  <b>x</b>  <i>y</i> ", "x y"},
		{"", ""},
	}
	for _, c := range cases {
		if got := cleanText(c.in); got != c.want {
			t.Errorf("cleanText(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestMatchesURLAndPatterns(t *testing.T) {
	s := New(SiteConfig{
		ID:       "fototest",
		SiteBase: "https://www.hucows.com",
		Patterns: []string{"hucows.com"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?hucows\.com`),
	})
	if !s.MatchesURL("https://www.hucows.com/") {
		t.Error("expected match")
	}
	if s.MatchesURL("https://example.com/") {
		t.Error("unexpected match")
	}
	pats := s.Patterns()
	var hasTag bool
	for _, p := range pats {
		if p == "www.hucows.com/tag/{slug}" {
			hasTag = true
		}
	}
	if !hasTag {
		t.Errorf("Patterns missing tag pattern: %v", pats)
	}
}
