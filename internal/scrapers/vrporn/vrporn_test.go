package vrporn

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
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

// TestRun drives ListScenes end to end.
//
// This previously reimplemented the scraper's own Paginate callback inside the
// test — its comment explained that apiBase was a const it could not redirect —
// which left ListScenes and run at 0% and would have passed even if run were
// broken. apiBase is now a field, so the real code path is exercised.
func TestRun(t *testing.T) {
	var paths []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		_, _ = fmt.Fprint(w, fixtureAPI)
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client(), apiBase: ts.URL}

	ch, err := s.ListScenes(context.Background(), "https://vrporn.com/studio/test/", scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}
	scenes := testutil.CollectScenes(t, ch)

	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2", len(scenes))
	}
	// The studio mode must hit the studio endpoint, not the model one.
	if len(paths) == 0 || !strings.Contains(strings.Join(paths, " "), "/videos/studio/test") {
		t.Errorf("requested paths = %v, want the studio endpoint", paths)
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

// The model URL mode must route to the model endpoint.
func TestRunModelMode(t *testing.T) {
	var paths []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		_, _ = fmt.Fprint(w, fixtureAPI)
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client(), apiBase: ts.URL}
	ch, err := s.ListScenes(context.Background(), "https://vrporn.com/pornstars/somebody/", scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if scenes := testutil.CollectScenes(t, ch); len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2", len(scenes))
	}
	if !strings.Contains(strings.Join(paths, " "), "/videos/model/somebody") {
		t.Errorf("requested paths = %v, want the model endpoint", paths)
	}
}

func TestIDAndPatterns(t *testing.T) {
	s := New()
	if s.ID() != "vrporn" {
		t.Errorf("ID = %q", s.ID())
	}
	if len(s.Patterns()) == 0 {
		t.Error("Patterns is empty")
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
