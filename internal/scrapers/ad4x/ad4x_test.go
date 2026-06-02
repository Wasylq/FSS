package ad4x

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
        "per_page": "24",
        "total_pages": 1,
        "data": [
          {
            "id": 100,
            "title": "Test Scene",
            "slug": "test-scene",
            "publish_date": "2026/05/15 12:00:00",
            "seconds_duration": 1706,
            "videos_duration": "28:26",
            "thumb": "https://cdn.example.com/thumb.jpg",
            "models": ["JANE DOE"],
            "models_slugs": [{"name": "Jane Doe", "slug": "jane-doe"}],
            "tags": ["Solo", "Big Tits"],
            "description": "A test description.",
            "content_price": 15,
            "rating": 4.07,
            "site": "AD4X",
            "site_domain": "ad4x.com"
          },
          {
            "id": 99,
            "title": "Another Scene",
            "slug": "another-scene",
            "publish_date": "2026/05/10 10:30:00",
            "seconds_duration": 900,
            "videos_duration": "15:00",
            "thumb": "https://cdn.example.com/thumb2.jpg",
            "models": ["ALICE"],
            "models_slugs": [{"name": "Alice", "slug": "alice"}],
            "tags": [],
            "description": "",
            "content_price": 0,
            "site": "AD4X"
          }
        ]
      }
    }
  }
}
</script></head><body></body></html>`

const modelPage = `<html><head>
<script id="__NEXT_DATA__" type="application/json">
{
  "props": {
    "pageProps": {
      "model": {
        "contents": {
          "total": 1,
          "page": "1",
          "total_pages": 1,
          "data": [
            {
              "id": 50,
              "title": "Model Scene",
              "slug": "model-scene",
              "publish_date": "2026/04/01 08:00:00",
              "seconds_duration": 600,
              "thumb": "https://cdn.example.com/model.jpg",
              "models": [],
              "models_slugs": [{"name": "Test Model", "slug": "test-model"}],
              "tags": ["Casting"],
              "description": "Model casting.",
              "content_price": 15,
              "site": "AD4X"
            }
          ]
        }
      }
    }
  }
}
</script></head><body></body></html>`

func newTestServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/en/models/jane-doe":
			_, _ = fmt.Fprint(w, modelPage)
		default:
			_, _ = fmt.Fprint(w, listingPage)
		}
	}))
}

func TestFetchListing(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	s := New()
	s.Client = ts.Client()
	s.baseURL = ts.URL

	items, total, _, err := s.fetchListing(context.Background(), ts.URL+"/en/videos?page=1")
	if err != nil {
		t.Fatalf("fetchListing: %v", err)
	}
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	if items[0].Title != "Test Scene" {
		t.Errorf("title = %q", items[0].Title)
	}
	if items[0].SecondsDuration != 1706 {
		t.Errorf("duration = %d, want 1706", items[0].SecondsDuration)
	}
	if len(items[0].ModelsSlugs) != 1 || items[0].ModelsSlugs[0].Name != "Jane Doe" {
		t.Errorf("models = %v", items[0].ModelsSlugs)
	}
}

func TestToScene(t *testing.T) {
	item := contentItem{
		ID:              100,
		Title:           "Test Scene",
		Slug:            "test-scene",
		PublishDate:     "2026/05/15 12:00:00",
		SecondsDuration: 1706,
		Thumb:           "https://cdn.example.com/thumb.jpg",
		Models:          []string{"JANE DOE"},
		ModelsSlugs:     []modelSlug{{Name: "Jane Doe", Slug: "jane-doe"}},
		Tags:            []string{"Solo"},
		Description:     "A test.",
		ContentPrice:    15,
	}
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	scene := toScene(item, "https://ad4x.com", now)

	if scene.ID != "100" {
		t.Errorf("ID = %q", scene.ID)
	}
	if scene.Title != "Test Scene" {
		t.Errorf("Title = %q", scene.Title)
	}
	if scene.Duration != 1706 {
		t.Errorf("Duration = %d", scene.Duration)
	}
	wantDate := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	if !scene.Date.Equal(wantDate) {
		t.Errorf("Date = %v, want %v", scene.Date, wantDate)
	}
	if len(scene.Performers) != 1 || scene.Performers[0] != "Jane Doe" {
		t.Errorf("Performers = %v", scene.Performers)
	}
	if scene.URL != "https://ad4x.com/en/videos/test-scene" {
		t.Errorf("URL = %q", scene.URL)
	}
	if len(scene.PriceHistory) != 1 || scene.PriceHistory[0].Regular != 15 {
		t.Errorf("PriceHistory = %v", scene.PriceHistory)
	}
}

func TestListScenes(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	s := New()
	s.Client = ts.Client()
	s.baseURL = ts.URL

	ch, err := s.ListScenes(context.Background(), ts.URL+"/en/videos", scraper.ListOpts{})
	if err != nil {
		t.Fatalf("ListScenes: %v", err)
	}

	var sceneCount int
	for r := range ch {
		switch r.Kind {
		case scraper.KindScene:
			sceneCount++
			if r.Scene.ID == "" {
				t.Error("empty ID")
			}
		case scraper.KindError:
			t.Errorf("unexpected error: %v", r.Err)
		}
	}
	if sceneCount != 2 {
		t.Errorf("got %d scenes, want 2", sceneCount)
	}
}

func TestModelPage(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	s := New()
	s.Client = ts.Client()
	s.baseURL = ts.URL

	ch, err := s.ListScenes(context.Background(), ts.URL+"/en/models/jane-doe", scraper.ListOpts{})
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
	if sceneCount != 1 {
		t.Errorf("got %d scenes, want 1", sceneCount)
	}
}

func TestMatchesURL(t *testing.T) {
	s := New()
	tests := []struct {
		url  string
		want bool
	}{
		{"https://ad4x.com/en/videos", true},
		{"https://www.ad4x.com/", true},
		{"https://ad4x.com/en/models/lexi", true},
		{"https://example.com", false},
	}
	for _, tt := range tests {
		if got := s.MatchesURL(tt.url); got != tt.want {
			t.Errorf("MatchesURL(%q) = %v, want %v", tt.url, got, tt.want)
		}
	}
}
