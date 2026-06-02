package innofsin

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

const postsJSON = `[
  {
    "id": 100,
    "date": "2026-05-15T12:00:00",
    "slug": "test-scene",
    "link": "BASEURL/test-scene/",
    "title": {"rendered": "Test Scene Title"},
    "content": {"rendered": "<p>A great scene with action.</p>"},
    "featured_media": 101,
    "categories": [1, 2],
    "_embedded": {
      "wp:featuredmedia": [{"source_url": "https://cdn.example.com/thumb.jpg"}],
      "wp:term": [[
        {"id": 1, "name": "Interracial"},
        {"id": 2, "name": "Scenes"}
      ]]
    }
  },
  {
    "id": 99,
    "date": "2026-05-10T10:30:00",
    "slug": "another-scene",
    "link": "BASEURL/another-scene/",
    "title": {"rendered": "Another Scene"},
    "content": {"rendered": "<p><b>Another</b> description here.</p>"},
    "featured_media": 102,
    "categories": [3],
    "_embedded": {
      "wp:featuredmedia": [{"source_url": "https://cdn.example.com/thumb2.jpg"}],
      "wp:term": [[
        {"id": 3, "name": "Anal"}
      ]]
    }
  }
]`

const detailHTML = `<html><body>
<div class="performers">
  <a href="/pornstars/jane-doe/">Jane Doe</a>
  <a href="/pornstars/alice-smith/">Alice Smith</a>
</div>
</body></html>`

func newTestServer() *httptest.Server {
	var ts *httptest.Server
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/wp-json/wp/v2/posts":
			w.Header().Set("X-WP-Total", "2")
			w.Header().Set("X-WP-TotalPages", "1")
			body := strings.ReplaceAll(postsJSON, "BASEURL", ts.URL)
			_, _ = fmt.Fprint(w, body)
		default:
			_, _ = fmt.Fprint(w, detailHTML)
		}
	}))
	return ts
}

func TestFetchPage(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	s := &siteScraper{
		cfg:    sites[0],
		client: ts.Client(),
	}

	posts, total, err := s.fetchPage(context.Background(), ts.URL+"/wp-json/wp/v2/posts?per_page=100&page=1&_embed")
	if err != nil {
		t.Fatalf("fetchPage: %v", err)
	}
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	if len(posts) != 2 {
		t.Fatalf("got %d posts, want 2", len(posts))
	}
	if posts[0].ID != 100 {
		t.Errorf("posts[0].ID = %d, want 100", posts[0].ID)
	}
	if posts[0].Title.Rendered != "Test Scene Title" {
		t.Errorf("title = %q", posts[0].Title.Rendered)
	}
	if len(posts[0].Embedded.Media) == 0 {
		t.Fatal("no embedded media")
	}
	if posts[0].Embedded.Media[0].SourceURL != "https://cdn.example.com/thumb.jpg" {
		t.Errorf("thumbnail = %q", posts[0].Embedded.Media[0].SourceURL)
	}
}

func TestFetchPerformers(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	s := &siteScraper{
		cfg:    sites[0],
		client: ts.Client(),
	}

	performers, err := s.fetchPerformers(context.Background(), ts.URL+"/test-scene/")
	if err != nil {
		t.Fatalf("fetchPerformers: %v", err)
	}
	if len(performers) != 2 {
		t.Fatalf("got %d performers, want 2", len(performers))
	}
	if performers[0] != "Jane Doe" {
		t.Errorf("performers[0] = %q, want Jane Doe", performers[0])
	}
	if performers[1] != "Alice Smith" {
		t.Errorf("performers[1] = %q, want Alice Smith", performers[1])
	}
}

func TestBuildScene(t *testing.T) {
	post := wpPost{
		ID:      100,
		Date:    "2026-05-15T12:00:00",
		Slug:    "test-scene",
		Link:    "https://example.com/test-scene/",
		Title:   wpField{Rendered: "Test &amp; Scene"},
		Content: wpField{Rendered: "<p>A <b>great</b> scene.</p>"},
		Embedded: wpEmbed{
			Media: []wpMedia{{SourceURL: "https://cdn.example.com/thumb.jpg"}},
			Terms: [][]wpTerm{{
				{ID: 1, Name: "Interracial"},
				{ID: 2, Name: "Scenes"},
				{ID: 3, Name: "Anal"},
			}},
		},
	}
	performers := []string{"Jane Doe"}
	cfg := siteConfig{id: "testsite", studio: "Test Studio"}
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	scene := buildScene(post, performers, cfg, "https://example.com", now)

	if scene.ID != "100" {
		t.Errorf("ID = %q", scene.ID)
	}
	if scene.Title != "Test & Scene" {
		t.Errorf("Title = %q", scene.Title)
	}
	wantDate := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	if !scene.Date.Equal(wantDate) {
		t.Errorf("Date = %v, want %v", scene.Date, wantDate)
	}
	if scene.Thumbnail != "https://cdn.example.com/thumb.jpg" {
		t.Errorf("Thumbnail = %q", scene.Thumbnail)
	}
	if scene.Description != "A great scene." {
		t.Errorf("Description = %q", scene.Description)
	}
	// "Scenes" should be filtered out
	if len(scene.Tags) != 2 || scene.Tags[0] != "Interracial" || scene.Tags[1] != "Anal" {
		t.Errorf("Tags = %v", scene.Tags)
	}
	if len(scene.Performers) != 1 || scene.Performers[0] != "Jane Doe" {
		t.Errorf("Performers = %v", scene.Performers)
	}
	if scene.Studio != "Test Studio" {
		t.Errorf("Studio = %q", scene.Studio)
	}
}

func TestListScenes(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	s := &siteScraper{
		cfg:    sites[0],
		client: ts.Client(),
	}

	ctx := context.Background()
	ch, err := s.ListScenes(ctx, ts.URL, scraper.ListOpts{})
	if err != nil {
		t.Fatalf("ListScenes: %v", err)
	}

	var sceneCount int
	for r := range ch {
		switch r.Kind {
		case scraper.KindScene:
			sceneCount++
			if r.Scene.ID == "" {
				t.Error("scene has empty ID")
			}
			if r.Scene.Title == "" {
				t.Error("scene has empty Title")
			}
		case scraper.KindError:
			t.Errorf("unexpected error: %v", r.Err)
		}
	}
	if sceneCount != 2 {
		t.Errorf("got %d scenes, want 2", sceneCount)
	}
}

func TestMatchesURL(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"https://mydeepdarksecret.com/", "mydeepdarksecret"},
		{"https://www.richardmannsworld.com/", "richardmannsworld"},
		{"https://bbctitans.com", "bbctitans"},
		{"https://www.richardmannevents.com/", "richardmannevents"},
	}
	for _, tt := range tests {
		found := false
		for _, cfg := range sites {
			s := newScraper(cfg)
			if s.MatchesURL(tt.url) {
				if s.ID() != tt.want {
					t.Errorf("MatchesURL(%q) matched %q, want %q", tt.url, s.ID(), tt.want)
				}
				found = true
				break
			}
		}
		if !found {
			t.Errorf("no scraper matched %q", tt.url)
		}
	}
}

func TestStripHTML(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"<p>Hello <b>world</b></p>", "Hello world"},
		{"<p>Test &amp; stuff</p>", "Test & stuff"},
		{"plain text", "plain text"},
		{"  <span>  spaced  </span>  ", "spaced"},
	}
	for _, tt := range tests {
		got := stripHTML(tt.in)
		if got != tt.want {
			t.Errorf("stripHTML(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
