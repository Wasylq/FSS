package kbproductions

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wasylq/FSS/scraper"
)

const listingPage = `<html><head>
<script id="__NEXT_DATA__" type="application/json">
{
  "props": {
    "pageProps": {
      "contents": {
        "total": 2,
        "page": "1",
        "per_page": "8",
        "total_pages": 1,
        "data": [
          {
            "id": 100,
            "title": "Test Scene",
            "slug": "test-scene",
            "publish_date": "2026/05/15 12:00:00",
            "seconds_duration": 872,
            "videos_duration": "14:32",
            "thumb": "https://cdn.example.com/thumb.jpg",
            "models": ["Jane"],
            "models_slugs": [{"name": "Jane", "slug": "jane"}],
            "tags": ["Solo"],
            "description": "A test.",
            "content_price": 0,
            "site": "Test Site"
          },
          {
            "id": 99,
            "title": "Scene Two",
            "slug": "scene-two",
            "publish_date": "2026/04/01 08:00:00",
            "seconds_duration": 600,
            "thumb": "https://cdn.example.com/thumb2.jpg",
            "models": [],
            "models_slugs": [],
            "tags": [],
            "description": "",
            "content_price": 15,
            "site": "Test Site"
          }
        ]
      }
    }
  }
}
</script></head><body></body></html>`

func newTestServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, listingPage)
	}))
}

func TestFetchListing(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	s := &siteScraper{cfg: sites[0], client: ts.Client()}

	items, total, totalPages, err := s.fetchListing(context.Background(), ts.URL+"/videos?page=1")
	if err != nil {
		t.Fatalf("fetchListing: %v", err)
	}
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	if totalPages != 1 {
		t.Errorf("totalPages = %d, want 1", totalPages)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	if items[0].Title != "Test Scene" {
		t.Errorf("title = %q", items[0].Title)
	}
	if items[0].SecondsDuration != 872 {
		t.Errorf("duration = %d", items[0].SecondsDuration)
	}
}

func TestToScene(t *testing.T) {
	s := &siteScraper{cfg: siteConfig{id: "testsite", domain: "test.com", studio: "Test Studio"}}
	item := contentItem{
		ID:              100,
		Title:           "Test Scene",
		Slug:            "test-scene",
		PublishDate:     "2026/05/15 12:00:00",
		SecondsDuration: 872,
		Thumb:           "https://cdn.example.com/thumb.jpg",
		ModelsSlugs:     []modelSlug{{Name: "Jane", Slug: "jane"}},
		Tags:            []string{"Solo"},
		Description:     "A test.",
		Site:            "Test Studio",
	}
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	scene := s.toScene(item, "https://test.com", now)

	if scene.ID != "100" {
		t.Errorf("ID = %q", scene.ID)
	}
	if scene.URL != "https://test.com/videos/test-scene" {
		t.Errorf("URL = %q", scene.URL)
	}
	wantDate := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	if !scene.Date.Equal(wantDate) {
		t.Errorf("Date = %v, want %v", scene.Date, wantDate)
	}
	if len(scene.Performers) != 1 || scene.Performers[0] != "Jane" {
		t.Errorf("Performers = %v", scene.Performers)
	}
}

func TestListScenes(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	s := &siteScraper{cfg: sites[0], client: ts.Client()}

	ch, err := s.ListScenes(context.Background(), ts.URL+"/videos", scraper.ListOpts{})
	if err != nil {
		t.Fatalf("ListScenes: %v", err)
	}

	var sceneCount int
	for r := range ch {
		switch r.Kind {
		case scraper.KindScene:
			sceneCount++
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
		{"https://melina-may.com/", "melinamay"},
		{"https://www.passionpov.com/videos", "passionpov"},
		{"https://shehergirls.com", "shehergirls"},
		{"https://vrallure.com/videos", "vrallure"},
		{"https://www.manpuppy.com/", "manpuppy"},
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
