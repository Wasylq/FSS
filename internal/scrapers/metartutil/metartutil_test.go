package metartutil

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wasylq/FSS/scraper"
)

const fixturePage1 = `{
  "total": 3,
  "galleries": [
    {
      "UUID": "AAA111",
      "name": "Scene One",
      "publishedAt": "2025-03-15T07:00:00.000Z",
      "type": "MOVIE",
      "siteUUID": "SITE1",
      "models": [
        {"UUID": "M1", "name": "Alice", "path": "/model/alice"},
        {"UUID": "M2", "name": "Bob", "path": "/model/bob"}
      ],
      "photographers": [
        {"UUID": "P1", "name": "John Doe"}
      ],
      "categories": ["Glamour", "Solo"],
      "runtime": 720,
      "path": "/model/alice/movie/20250315/SCENE_ONE",
      "coverImagePath": "/media/AAA111/cover_AAA111.jpg"
    },
    {
      "UUID": "BBB222",
      "name": "Gallery Only",
      "publishedAt": "2025-03-14T07:00:00.000Z",
      "type": "GALLERY",
      "siteUUID": "SITE1",
      "models": [{"UUID": "M3", "name": "Carol", "path": "/model/carol"}],
      "photographers": [],
      "categories": [],
      "runtime": -1,
      "path": "/model/carol/gallery/20250314/GALLERY_ONLY",
      "coverImagePath": "/media/BBB222/cover_BBB222.jpg"
    },
    {
      "UUID": "CCC333",
      "name": "Scene Two",
      "publishedAt": "2025-03-13T07:00:00.000Z",
      "type": "MOVIE",
      "siteUUID": "SITE1",
      "models": [{"UUID": "M4", "name": "Diana", "path": "/model/diana"}],
      "photographers": [{"UUID": "P2", "name": "Jane Smith"}],
      "categories": ["Art"],
      "runtime": 1200,
      "path": "/model/diana/movie/20250313/SCENE_TWO",
      "coverImagePath": ""
    }
  ]
}`

const fixtureEmpty = `{"total": 0, "galleries": []}`

func newTestServer(pages map[int]string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")
		for p, body := range pages {
			if fmt.Sprintf("%d", p) == page {
				w.Header().Set("Content-Type", "application/json")
				_, _ = fmt.Fprint(w, body)
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, fixtureEmpty)
	}))
}

func newTestScraper(ts *httptest.Server) *Scraper {
	return &Scraper{
		client: ts.Client(),
		base:   ts.URL,
		Config: SiteConfig{SiteID: "testsite", Domain: "test.com", StudioName: "Test Studio"},
	}
}

func collect(ch <-chan scraper.SceneResult) []scraper.SceneResult {
	var results []scraper.SceneResult
	for r := range ch {
		results = append(results, r)
	}
	return results
}

func TestListScenes(t *testing.T) {
	ts := newTestServer(map[int]string{1: fixturePage1})
	defer ts.Close()

	s := newTestScraper(ts)
	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}

	results := collect(ch)

	var scenes, progress int
	for _, r := range results {
		switch r.Kind {
		case scraper.KindScene:
			scenes++
		case scraper.KindTotal:
			progress++
		}
	}

	if scenes != 2 {
		t.Errorf("got %d scenes, want 2 (galleries should be filtered)", scenes)
	}
	if progress != 1 {
		t.Errorf("got %d progress messages, want 1", progress)
	}

	// Verify first scene
	first := results[1] // index 0 is progress
	if first.Scene.ID != "AAA111" {
		t.Errorf("first scene ID = %q, want AAA111", first.Scene.ID)
	}
	if first.Scene.Title != "Scene One" {
		t.Errorf("title = %q, want %q", first.Scene.Title, "Scene One")
	}
	if first.Scene.Duration != 720 {
		t.Errorf("duration = %d, want 720", first.Scene.Duration)
	}
	if len(first.Scene.Performers) != 2 || first.Scene.Performers[0] != "Alice" {
		t.Errorf("performers = %v, want [Alice Bob]", first.Scene.Performers)
	}
	if first.Scene.Director != "John Doe" {
		t.Errorf("director = %q, want %q", first.Scene.Director, "John Doe")
	}
	if len(first.Scene.Tags) != 2 || first.Scene.Tags[0] != "Glamour" {
		t.Errorf("tags = %v, want [Glamour Solo]", first.Scene.Tags)
	}
	wantThumb := cdnBase + "/media/AAA111/cover_AAA111.jpg"
	if first.Scene.Thumbnail != wantThumb {
		t.Errorf("thumbnail = %q, want %q", first.Scene.Thumbnail, wantThumb)
	}
	if first.Scene.Date.Format("2006-01-02") != "2025-03-15" {
		t.Errorf("date = %v, want 2025-03-15", first.Scene.Date)
	}
	if first.Scene.SiteID != "testsite" {
		t.Errorf("siteID = %q, want testsite", first.Scene.SiteID)
	}
	if first.Scene.Studio != "Test Studio" {
		t.Errorf("studio = %q, want %q", first.Scene.Studio, "Test Studio")
	}

	// Second scene has no photographer → no director, no thumbnail
	second := results[2]
	if second.Scene.ID != "CCC333" {
		t.Errorf("second scene ID = %q, want CCC333", second.Scene.ID)
	}
	if second.Scene.Director != "Jane Smith" {
		t.Errorf("director = %q, want %q", second.Scene.Director, "Jane Smith")
	}
	if second.Scene.Thumbnail != "" {
		t.Errorf("thumbnail = %q, want empty (no cover)", second.Scene.Thumbnail)
	}
}

func TestKnownIDsStopEarly(t *testing.T) {
	ts := newTestServer(map[int]string{1: fixturePage1})
	defer ts.Close()

	s := newTestScraper(ts)
	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{
		KnownIDs: map[string]bool{"CCC333": true},
	})
	if err != nil {
		t.Fatal(err)
	}

	results := collect(ch)

	var scenes, stoppedEarly int
	for _, r := range results {
		switch r.Kind {
		case scraper.KindScene:
			scenes++
		case scraper.KindStoppedEarly:
			stoppedEarly++
		}
	}

	if scenes != 1 {
		t.Errorf("got %d scenes, want 1 (should stop at CCC333)", scenes)
	}
	if stoppedEarly != 1 {
		t.Errorf("got %d stoppedEarly, want 1", stoppedEarly)
	}
}

func TestGalleryFiltering(t *testing.T) {
	allGalleries := `{
		"total": 2,
		"galleries": [
			{"UUID": "G1", "name": "Photo Set", "publishedAt": "2025-01-01T00:00:00.000Z", "type": "GALLERY", "models": [], "photographers": [], "categories": [], "runtime": -1, "path": "/g1", "coverImagePath": ""},
			{"UUID": "G2", "name": "Another Set", "publishedAt": "2025-01-02T00:00:00.000Z", "type": "GALLERY", "models": [], "photographers": [], "categories": [], "runtime": -1, "path": "/g2", "coverImagePath": ""}
		]
	}`

	ts := newTestServer(map[int]string{1: allGalleries})
	defer ts.Close()

	s := newTestScraper(ts)
	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}

	results := collect(ch)
	for _, r := range results {
		if r.Kind == scraper.KindScene {
			t.Error("should not emit scenes for GALLERY entries")
		}
	}
}

func TestMatchesURL(t *testing.T) {
	s := NewScraper(SiteConfig{SiteID: "metart", Domain: "metart.com", StudioName: "MetArt"})

	tests := []struct {
		url  string
		want bool
	}{
		{"https://www.metart.com", true},
		{"https://metart.com", true},
		{"https://www.metart.com/model/alice", true},
		{"https://www.sexart.com", false},
		{"https://notmetart.com", false},
	}
	for _, tt := range tests {
		if got := s.MatchesURL(tt.url); got != tt.want {
			t.Errorf("MatchesURL(%q) = %v, want %v", tt.url, got, tt.want)
		}
	}
}

func TestToScene(t *testing.T) {
	cfg := SiteConfig{SiteID: "metart", Domain: "metart.com", StudioName: "MetArt"}
	base := "https://www.metart.com"

	g := gallery{
		UUID:           "ABC123",
		Name:           "Test Scene",
		PublishedAt:    "2025-06-15T07:00:00.000Z",
		Type:           "MOVIE",
		Models:         []apiModel{{Name: "Model A"}, {Name: "Model B"}},
		Photographers:  []apiPerson{{Name: "Director X"}},
		Categories:     []string{"Tag1", "Tag2"},
		Runtime:        600,
		Path:           "/model/model-a/movie/20250615/TEST_SCENE",
		CoverImagePath: "/media/ABC123/cover_ABC123.jpg",
	}

	sc := toScene(cfg, base, g, fixedTime())
	if sc.URL != "https://www.metart.com/model/model-a/movie/20250615/TEST_SCENE" {
		t.Errorf("URL = %q", sc.URL)
	}
	if sc.Thumbnail != "https://gccdn.metartnetwork.com/media/ABC123/cover_ABC123.jpg" {
		t.Errorf("thumbnail = %q", sc.Thumbnail)
	}
}

func fixedTime() time.Time {
	return time.Date(2025, 6, 20, 12, 0, 0, 0, time.UTC)
}
