package vrporn

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/scraper"
)

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url  string
		want bool
	}{
		{"https://vrporn.com/studio/hotkinkyjo-hkjvr-virtual-reality-vr180/", true},
		{"https://vrporn.com/studio/virtualrealporn/", true},
		{"https://vrporn.com/pornstars/aria-taylor/", true},
		{"https://www.vrporn.com/studio/test/", true},
		{"https://vrporn.com/", false},
		{"https://vrporn.com/some-video-slug/", false},
		{"https://example.com/", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.want {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.want)
		}
	}
}

func TestResolveURL(t *testing.T) {
	cases := []struct {
		url      string
		wantMode urlMode
		wantSlug string
	}{
		{"https://vrporn.com/studio/hotkinkyjo-hkjvr-virtual-reality-vr180/", modeStudio, "hotkinkyjo-hkjvr-virtual-reality-vr180"},
		{"https://vrporn.com/studio/virtualrealporn/", modeStudio, "virtualrealporn"},
		{"https://vrporn.com/pornstars/aria-taylor/", modeModel, "aria-taylor"},
		{"https://vrporn.com/", modeStudio, ""},
	}
	for _, c := range cases {
		mode, slug := resolveURL(c.url)
		if mode != c.wantMode || slug != c.wantSlug {
			t.Errorf("resolveURL(%q) = (%d, %q), want (%d, %q)", c.url, mode, slug, c.wantMode, c.wantSlug)
		}
	}
}

const fixtureAPI = `{
  "status": {"code": 1, "message": "Ok"},
  "data": {
    "pages": 1,
    "total": 2,
    "items": [
      {
        "id": "aaa-111",
        "name": "Test Scene One",
        "slug": "test-scene-one",
        "publishedAt": 1717200000,
        "time": 521,
        "models": ["Performer A", "Performer B"],
        "studio": {"name": "Test Studio", "slug": "test-studio"},
        "previewImage": {"path": "https://cdn.vrporn.com/img1.jpg"},
        "likes": 42,
        "views": 1000
      },
      {
        "id": "bbb-222",
        "name": "Test Scene Two",
        "slug": "test-scene-two",
        "publishedAt": 1717100000,
        "time": 300,
        "models": ["Performer C"],
        "studio": {"name": "Test Studio", "slug": "test-studio"},
        "previewImage": {"path": "https://cdn.vrporn.com/img2.jpg"},
        "likes": 10,
        "views": 500
      }
    ]
  }
}`

func TestRun(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, fixtureAPI)
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client()}
	// Override apiBase by using the test server URL directly in studioURL
	// The scraper extracts the slug and builds the API URL from apiBase constant,
	// so we need to make the test server respond to any request.
	out := make(chan scraper.SceneResult)
	go func() {
		defer close(out)
		scraper.Paginate(context.Background(), scraper.ListOpts{}, "vrporn", out, func(ctx context.Context, page int) (scraper.PageResult, error) {
			u := ts.URL + "/proxy/api/content/v1/videos/studio/test?page=1&limit=100&sort=new"
			var resp apiResponse
			if err := s.fetchJSON(ctx, u, &resp); err != nil {
				return scraper.PageResult{}, err
			}
			scenes := make([]models.Scene, 0, len(resp.Data.Items))
			now := time.Now().UTC()
			for _, item := range resp.Data.Items {
				scenes = append(scenes, toScene(item, ts.URL+"/studio/test/", now))
			}
			return scraper.PageResult{
				Scenes: scenes,
				Total:  resp.Data.Total,
				Done:   true,
			}, nil
		})
	}()

	scenes := testutil.CollectScenes(t, out)

	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2", len(scenes))
	}

	sc := scenes[0]
	if sc.ID != "aaa-111" {
		t.Errorf("ID = %q", sc.ID)
	}
	if sc.Title != "Test Scene One" {
		t.Errorf("Title = %q", sc.Title)
	}
	if sc.URL != "https://vrporn.com/test-scene-one/" {
		t.Errorf("URL = %q", sc.URL)
	}
	if sc.Duration != 521 {
		t.Errorf("Duration = %d", sc.Duration)
	}
	if len(sc.Performers) != 2 || sc.Performers[0] != "Performer A" {
		t.Errorf("Performers = %v", sc.Performers)
	}
	if sc.Studio != "Test Studio" {
		t.Errorf("Studio = %q", sc.Studio)
	}
	if sc.Views != 1000 {
		t.Errorf("Views = %d", sc.Views)
	}
	if sc.Date.IsZero() {
		t.Error("Date is zero")
	}
	if sc.Thumbnail != "https://cdn.vrporn.com/img1.jpg" {
		t.Errorf("Thumbnail = %q", sc.Thumbnail)
	}
}

func TestToScene(t *testing.T) {
	item := apiItem{
		ID:           "uuid-123",
		Name:         "Test",
		Slug:         "test-video",
		PublishedAt:  0,
		Time:         60,
		Models:       nil,
		Studio:       apiStudio{Name: "Studio", Slug: "studio"},
		PreviewImage: apiImage{Path: ""},
	}
	now := time.Now().UTC()
	sc := toScene(item, "https://vrporn.com/studio/studio/", now)

	if sc.Date.IsZero() != true {
		t.Error("zero publishedAt should give zero date")
	}
	if sc.Thumbnail != "" {
		t.Errorf("empty path should give empty thumbnail, got %q", sc.Thumbnail)
	}
	if sc.URL != "https://vrporn.com/test-video/" {
		t.Errorf("URL = %q", sc.URL)
	}
}
