package wankitnowutil

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
	"github.com/Wasylq/FSS/scraper"
)

var _ scraper.StudioScraper = (*Scraper)(nil)

var testCfg = SiteConfig{
	ID:       "testsite",
	Domain:   "testsite.com",
	Studio:   "Test Studio",
	Patterns: []string{"testsite.com/videos"},
	MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?testsite\.com`),
}

func TestNormalizePerformer(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"JANE DOE", "Jane Doe"},
		{"jane doe", "Jane Doe"},
		{"Jane Doe", "Jane Doe"},
		{"SINGLE", "Single"},
		{"  extra   spaces  ", "Extra Spaces"},
	}
	for _, tt := range tests {
		if got := normalizePerformer(tt.in); got != tt.want {
			t.Errorf("normalizePerformer(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestToScene(t *testing.T) {
	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	sc := sceneJSON{
		ID:              42,
		Title:           "Test Scene",
		Slug:            "test-scene",
		Description:     "A test description",
		PublishDate:     "2026/05/01 10:00:00",
		SecondsDuration: 600,
		Thumb:           "https://cdn.example.com/thumb.jpg",
		Models:          []string{"JANE DOE", "JOHN SMITH"},
		Tags:            []string{"tag1", "tag2"},
	}

	scene := toScene(sc, "testsite", "https://www.testsite.com", "Test Studio", now)

	if scene.ID != "42" {
		t.Errorf("ID = %q, want %q", scene.ID, "42")
	}
	if scene.SiteID != "testsite" {
		t.Errorf("SiteID = %q, want %q", scene.SiteID, "testsite")
	}
	if scene.Title != "Test Scene" {
		t.Errorf("Title = %q, want %q", scene.Title, "Test Scene")
	}
	if scene.URL != "https://www.testsite.com/videos/test-scene" {
		t.Errorf("URL = %q", scene.URL)
	}
	if scene.Date.Format("2006-01-02") != "2026-05-01" {
		t.Errorf("Date = %v, want 2026-05-01", scene.Date)
	}
	if scene.Duration != 600 {
		t.Errorf("Duration = %d, want 600", scene.Duration)
	}
	if len(scene.Performers) != 2 || scene.Performers[0] != "Jane Doe" || scene.Performers[1] != "John Smith" {
		t.Errorf("Performers = %v, want [Jane Doe John Smith]", scene.Performers)
	}
	if len(scene.Tags) != 2 {
		t.Errorf("Tags = %v, want 2 tags", scene.Tags)
	}
	if scene.Studio != "Test Studio" {
		t.Errorf("Studio = %q, want %q", scene.Studio, "Test Studio")
	}
	if scene.Thumbnail != "https://cdn.example.com/thumb.jpg" {
		t.Errorf("Thumbnail = %q", scene.Thumbnail)
	}
}

func TestToSceneEmptyDate(t *testing.T) {
	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	sc := sceneJSON{ID: 1, Title: "No Date"}
	scene := toScene(sc, "testsite", "https://www.testsite.com", "Test Studio", now)
	if !scene.Date.IsZero() {
		t.Errorf("Date should be zero for empty publish_date, got %v", scene.Date)
	}
}

func TestToSceneEmptyModels(t *testing.T) {
	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	sc := sceneJSON{ID: 1, Title: "No Models", Models: []string{"", "  "}}
	scene := toScene(sc, "testsite", "https://www.testsite.com", "Test Studio", now)
	if len(scene.Performers) != 0 {
		t.Errorf("Performers should be empty for blank models, got %v", scene.Performers)
	}
}

func TestBuildIDRegex(t *testing.T) {
	tests := []struct {
		html string
		want string
	}{
		{`"buildId":"abc123xyz"`, "abc123xyz"},
		{`"buildId" : "spaced-id"`, "spaced-id"},
		{`no build id here`, ""},
	}
	for _, tt := range tests {
		m := buildIDRe.FindStringSubmatch(tt.html)
		got := ""
		if m != nil {
			got = m[1]
		}
		if got != tt.want {
			t.Errorf("buildIDRe on %q = %q, want %q", tt.html, got, tt.want)
		}
	}
}

func newTestServer(scenes []sceneJSON, totalPages int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprint(w, `<script>{"buildId":"test-build-123"}</script>`)
		case "/_next/data/test-build-123/videos.json":
			w.Header().Set("Content-Type", "application/json")
			resp := nextDataResponse{}
			resp.PageProps.Contents.Total = len(scenes)
			resp.PageProps.Contents.TotalPages = totalPages
			resp.PageProps.Contents.Data = scenes
			_ = json.NewEncoder(w).Encode(resp)
		default:
			http.NotFound(w, r)
		}
	}))
}

func TestRun(t *testing.T) {
	scenes := []sceneJSON{
		{ID: 1, Title: "Scene One", Slug: "scene-one", Models: []string{"JANE DOE"}, SecondsDuration: 300},
		{ID: 2, Title: "Scene Two", Slug: "scene-two", Models: []string{"JOHN SMITH"}, SecondsDuration: 600},
	}

	srv := newTestServer(scenes, 1)
	defer srv.Close()

	cfg := testCfg
	cfg.BaseURL = srv.URL

	s := New(cfg)
	ch, err := s.ListScenes(context.Background(), srv.URL, scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}

	got := testutil.CollectScenes(t, ch)
	if len(got) != 2 {
		t.Fatalf("got %d scenes, want 2", len(got))
	}
	if got[0].Title != "Scene One" {
		t.Errorf("scenes[0].Title = %q, want %q", got[0].Title, "Scene One")
	}
	if got[0].Performers[0] != "Jane Doe" {
		t.Errorf("scenes[0].Performers[0] = %q, want %q", got[0].Performers[0], "Jane Doe")
	}
	if got[1].Duration != 600 {
		t.Errorf("scenes[1].Duration = %d, want 600", got[1].Duration)
	}
	if got[0].SiteID != "testsite" {
		t.Errorf("scenes[0].SiteID = %q, want %q", got[0].SiteID, "testsite")
	}
}

func TestKnownIDs(t *testing.T) {
	scenes := []sceneJSON{
		{ID: 1, Title: "Scene One", Slug: "scene-one"},
		{ID: 2, Title: "Scene Two", Slug: "scene-two"},
		{ID: 3, Title: "Scene Three", Slug: "scene-three"},
	}

	srv := newTestServer(scenes, 1)
	defer srv.Close()

	cfg := testCfg
	cfg.BaseURL = srv.URL

	s := New(cfg)
	ch, err := s.ListScenes(context.Background(), srv.URL, scraper.ListOpts{
		KnownIDs: map[string]bool{"2": true},
	})
	if err != nil {
		t.Fatal(err)
	}

	got, stopped := testutil.CollectScenesWithStop(t, ch)
	if len(got) != 1 {
		t.Fatalf("got %d scenes, want 1", len(got))
	}
	if got[0].Title != "Scene One" {
		t.Errorf("scenes[0].Title = %q, want %q", got[0].Title, "Scene One")
	}
	if !stopped {
		t.Error("expected StoppedEarly")
	}
}
