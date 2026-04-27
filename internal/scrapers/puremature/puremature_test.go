package puremature

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wasylq/FSS/scraper"
)

func TestMatchesURL(t *testing.T) {
	s := New()
	tests := []struct {
		url  string
		want bool
	}{
		{"https://puremature.com/", true},
		{"https://www.puremature.com/", true},
		{"https://puremature.com/models/laura-bentley", true},
		{"https://puremature.com/video/some-scene", true},
		{"https://example.com", false},
	}
	for _, tt := range tests {
		if got := s.MatchesURL(tt.url); got != tt.want {
			t.Errorf("MatchesURL(%q) = %v, want %v", tt.url, got, tt.want)
		}
	}
}

func TestURLClassification(t *testing.T) {
	tests := []struct {
		url       string
		wantModel bool
	}{
		{"https://puremature.com/", false},
		{"https://puremature.com/videos", false},
		{"https://puremature.com/models/laura-bentley", true},
	}
	for _, tt := range tests {
		m := modelRe.FindStringSubmatch(tt.url)
		isModel := m != nil
		if isModel != tt.wantModel {
			t.Errorf("modelRe.Match(%q) = %v, want %v", tt.url, isModel, tt.wantModel)
		}
		if tt.wantModel && m[1] != "laura-bentley" {
			t.Errorf("model slug = %q, want laura-bentley", m[1])
		}
	}
}

func makeTestScene(id int, slug, title string) apiScene {
	return apiScene{
		ID:         id,
		CachedSlug: slug,
		Title:      title,
		ReleasedAt: "2026-04-01T15:00:00Z",
		PosterURL:  "https://cdn-images.example.com/poster.jpg",
		ThumbURL:   "https://cdn-images.example.com/thumb.jpg",
		TrailerURL: "https://cdn-videos.example.com/trailer.mp4",
		Tags:       []string{"milf", "creampie"},
		Actors: []apiActor{
			{ID: 100, Name: "Laura Bentley", CachedSlug: "laura-bentley", Gender: "girl"},
		},
		Sponsor: apiSponsor{Name: "Pure Mature", CachedSlug: "pure-mature"},
		DownloadOptions: []struct {
			Quality string `json:"quality"`
		}{
			{Quality: "2160"},
			{Quality: "1080"},
			{Quality: "720"},
		},
	}
}

func TestItemToScene(t *testing.T) {
	item := makeTestScene(76232, "my-daughters-boyfriend", "My Daughter's Boyfriend")
	now := time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC)

	scene := itemToScene(item, "https://puremature.com/", now)

	if scene.ID != "76232" {
		t.Errorf("ID = %q, want 76232", scene.ID)
	}
	if scene.Title != "My Daughter's Boyfriend" {
		t.Errorf("Title = %q", scene.Title)
	}
	if scene.URL != "https://puremature.com/video/my-daughters-boyfriend" {
		t.Errorf("URL = %q", scene.URL)
	}
	if len(scene.Performers) != 1 || scene.Performers[0] != "Laura Bentley" {
		t.Errorf("Performers = %v", scene.Performers)
	}
	if len(scene.Tags) != 2 {
		t.Errorf("Tags = %v", scene.Tags)
	}
	if scene.Studio != "Pure Mature" {
		t.Errorf("Studio = %q", scene.Studio)
	}
	if scene.Resolution != "4K" {
		t.Errorf("Resolution = %q, want 4K", scene.Resolution)
	}
	if scene.Height != 2160 {
		t.Errorf("Height = %d, want 2160", scene.Height)
	}
	wantDate := time.Date(2026, 4, 1, 15, 0, 0, 0, time.UTC)
	if !scene.Date.Equal(wantDate) {
		t.Errorf("Date = %v, want %v", scene.Date, wantDate)
	}
	if scene.Thumbnail == "" {
		t.Error("Thumbnail is empty")
	}
	if scene.Preview == "" {
		t.Error("Preview is empty")
	}
}

func TestParseDate(t *testing.T) {
	tests := []struct {
		in   string
		want time.Time
	}{
		{"2026-04-01T15:00:00Z", time.Date(2026, 4, 1, 15, 0, 0, 0, time.UTC)},
		{"2024-01-15T00:00:00Z", time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)},
		{"bad", time.Time{}},
	}
	for _, tt := range tests {
		got := parseDate(tt.in)
		if !got.Equal(tt.want) {
			t.Errorf("parseDate(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestQuerySep(t *testing.T) {
	if querySep("https://example.com/api?sort=latest") != "&" {
		t.Error("expected & for URL with existing query")
	}
	if querySep("https://example.com/api/releases") != "?" {
		t.Error("expected ? for URL without query")
	}
}

func fakeAPIResponse(scenes []apiScene, total int, hasNext bool) []byte {
	resp := apiResponse{}
	resp.Items = scenes
	resp.Pagination.TotalItems = total
	resp.Pagination.TotalPages = (total + pageSize - 1) / pageSize
	if hasNext {
		next := "/api/releases?page=2"
		resp.Pagination.NextPage = &next
	}
	data, _ := json.Marshal(resp)
	return data
}

func TestPaginatedScrape(t *testing.T) {
	s1 := makeTestScene(100, "scene-a", "Scene A")
	s2 := makeTestScene(101, "scene-b", "Scene B")
	page1 := fakeAPIResponse([]apiScene{s1, s2}, 2, false)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(page1)
	}))
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s := &Scraper{client: ts.Client()}
	out := make(chan scraper.SceneResult)
	go func() {
		defer close(out)
		s.runWithBase(ctx, ts.URL+"/api/releases?sort=latest", ts.URL, scraper.ListOpts{}, out)
	}()

	var scenes []string
	for r := range out {
		if r.Err != nil {
			t.Fatalf("unexpected error: %v", r.Err)
		}
		if r.Total > 0 || r.StoppedEarly {
			continue
		}
		scenes = append(scenes, r.Scene.ID)
	}
	if len(scenes) != 2 {
		t.Errorf("got %d scenes, want 2", len(scenes))
	}
}

func TestKnownIDsStopsEarly(t *testing.T) {
	scenes := []apiScene{
		makeTestScene(100, "a", "A"),
		makeTestScene(101, "b", "B"),
		makeTestScene(102, "c", "C"),
	}
	page := fakeAPIResponse(scenes, 3, false)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(page)
	}))
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s := &Scraper{client: ts.Client()}
	out := make(chan scraper.SceneResult)
	opts := scraper.ListOpts{KnownIDs: map[string]bool{"101": true}}
	go func() {
		defer close(out)
		s.runWithBase(ctx, ts.URL+"/api/releases?sort=latest", ts.URL, opts, out)
	}()

	var gotScenes int
	var stoppedEarly bool
	for r := range out {
		if r.StoppedEarly {
			stoppedEarly = true
			continue
		}
		if r.Total > 0 || r.Err != nil {
			continue
		}
		gotScenes++
	}
	if gotScenes != 1 {
		t.Errorf("got %d scenes before known ID, want 1", gotScenes)
	}
	if !stoppedEarly {
		t.Error("expected StoppedEarly")
	}
}

func TestTagCleaning(t *testing.T) {
	item := apiScene{
		ID:         1,
		CachedSlug: "test",
		Tags:       []string{"step_mom", "big_tits", "creampie"},
	}
	scene := itemToScene(item, "https://puremature.com/", time.Now())
	want := []string{"step mom", "big tits", "creampie"}
	if fmt.Sprint(scene.Tags) != fmt.Sprint(want) {
		t.Errorf("Tags = %v, want %v", scene.Tags, want)
	}
}

func TestScraperInterface(t *testing.T) {
	var _ scraper.StudioScraper = (*Scraper)(nil)
}
