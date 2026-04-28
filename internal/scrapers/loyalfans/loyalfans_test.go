package loyalfans

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
	"github.com/Wasylq/FSS/scraper"
)

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url  string
		want bool
	}{
		{"https://www.loyalfans.com/bettie_bondage", true},
		{"https://loyalfans.com/bettie_bondage", true},
		{"https://www.loyalfans.com/some-creator", true},
		{"https://www.loyalfans.com/bettie_bondage/video/some-slug", false},
		{"https://www.loyalfans.com", false},
		{"https://www.manyvids.com/Profile/123", false},
		{"", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.want {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.want)
		}
	}
}

func TestToScene(t *testing.T) {
	v := video{
		Slug:     "my-scene-123456",
		Title:    "My Scene",
		Content:  "A description<br />with linebreaks #tag1 #tag2",
		Hashtags: []string{"#tag1", "#tag2"},
	}
	v.Owner.Slug = "creator1"
	v.Owner.DisplayName = "Creator One"
	v.CreatedAt.Date = "2026-03-15 12:30:00"
	v.VideoObject.Duration = 900
	v.VideoObject.Poster = "https://cdn.example.com/poster.jpg"
	v.Reactions.Total = 42

	sc := toScene("https://www.loyalfans.com/creator1", "creator1", v)

	if sc.ID != "my-scene-123456" {
		t.Errorf("ID = %q", sc.ID)
	}
	if sc.SiteID != "loyalfans" {
		t.Errorf("SiteID = %q", sc.SiteID)
	}
	if sc.URL != "https://www.loyalfans.com/creator1/video/my-scene-123456" {
		t.Errorf("URL = %q", sc.URL)
	}
	if sc.Title != "My Scene" {
		t.Errorf("Title = %q", sc.Title)
	}
	if sc.Duration != 900 {
		t.Errorf("Duration = %d", sc.Duration)
	}
	if sc.Thumbnail != "https://cdn.example.com/poster.jpg" {
		t.Errorf("Thumbnail = %q", sc.Thumbnail)
	}
	if sc.Studio != "Creator One" {
		t.Errorf("Studio = %q", sc.Studio)
	}
	if sc.Likes != 42 {
		t.Errorf("Likes = %d", sc.Likes)
	}
	if len(sc.Performers) != 1 || sc.Performers[0] != "Creator One" {
		t.Errorf("Performers = %v", sc.Performers)
	}
	if len(sc.Tags) != 2 || sc.Tags[0] != "tag1" || sc.Tags[1] != "tag2" {
		t.Errorf("Tags = %v", sc.Tags)
	}
	if sc.Date.Year() != 2026 || sc.Date.Month() != 3 || sc.Date.Day() != 15 {
		t.Errorf("Date = %v", sc.Date)
	}
	if sc.Description != "A description\nwith linebreaks" {
		t.Errorf("Description = %q", sc.Description)
	}
}

func TestStripHashtags(t *testing.T) {
	cases := []struct {
		input, want string
	}{
		{"hello #tag1 world #tag2", "hello  world "},
		{"no tags here", "no tags here"},
		{"#only #tags", " "},
		{"", ""},
	}
	for _, c := range cases {
		if got := stripHashtags(c.input); got != c.want {
			t.Errorf("stripHashtags(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

func makeVideos(slug string, n int) []video {
	videos := make([]video, n)
	for i := range n {
		v := video{
			Slug:     fmt.Sprintf("scene-%d", i+1),
			Title:    fmt.Sprintf("Scene %d", i+1),
			Hashtags: []string{},
		}
		v.Owner.Slug = slug
		v.Owner.DisplayName = "Test Creator"
		v.CreatedAt.Date = "2026-01-15 12:00:00"
		v.VideoObject.Duration = 600
		videos[i] = v
	}
	return videos
}

func newTestServer(pages [][]video) *httptest.Server {
	pageIdx := 0
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Path == "/api/v2/system-status" {
			http.SetCookie(w, &http.Cookie{Name: "session", Value: "test"})
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
			return
		}

		if r.URL.Path == "/api/v2/advanced-search" {
			if pageIdx >= len(pages) {
				_ = json.NewEncoder(w).Encode(searchResponse{Success: true})
				return
			}
			data := pages[pageIdx]
			pageIdx++

			var nextToken *string
			if pageIdx < len(pages) {
				tok := "next-page-token"
				nextToken = &tok
			}
			_ = json.NewEncoder(w).Encode(searchResponse{
				Success:   true,
				Data:      data,
				PageToken: nextToken,
			})
			return
		}

		http.NotFound(w, r)
	}))
}

func TestListScenes(t *testing.T) {
	slug := "test_creator"
	page1 := makeVideos(slug, 3)

	ts := newTestServer([][]video{page1})
	defer ts.Close()

	s := &Scraper{client: ts.Client(), base: ts.URL}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/test_creator", scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}

	scenes := testutil.CollectScenes(t, ch)
	if len(scenes) != 3 {
		t.Fatalf("got %d scenes, want 3", len(scenes))
	}
	if scenes[0].Title != "Scene 1" {
		t.Errorf("first scene title = %q", scenes[0].Title)
	}
	if scenes[2].Title != "Scene 3" {
		t.Errorf("last scene title = %q", scenes[2].Title)
	}
}

func TestListScenesPagination(t *testing.T) {
	slug := "test_creator"
	page1 := makeVideos(slug, 20)
	page2 := makeVideos(slug, 5)
	// Offset page2 slugs so they don't duplicate page1.
	for i := range page2 {
		page2[i].Slug = fmt.Sprintf("scene-%d", 21+i)
		page2[i].Title = fmt.Sprintf("Scene %d", 21+i)
	}

	ts := newTestServer([][]video{page1, page2})
	defer ts.Close()

	s := &Scraper{client: ts.Client(), base: ts.URL}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/test_creator", scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}

	scenes := testutil.CollectScenes(t, ch)
	if len(scenes) != 25 {
		t.Fatalf("got %d scenes, want 25", len(scenes))
	}
}

func TestListScenesKnownIDs(t *testing.T) {
	slug := "test_creator"
	page1 := makeVideos(slug, 5)

	ts := newTestServer([][]video{page1})
	defer ts.Close()

	s := &Scraper{client: ts.Client(), base: ts.URL}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/test_creator", scraper.ListOpts{
		KnownIDs: map[string]bool{"scene-3": true},
	})
	if err != nil {
		t.Fatal(err)
	}

	scenes, stoppedEarly := testutil.CollectScenesWithStop(t, ch)
	if !stoppedEarly {
		t.Error("expected StoppedEarly signal")
	}
	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2 (early stop at known ID)", len(scenes))
	}
	if scenes[0].ID != "scene-1" || scenes[1].ID != "scene-2" {
		t.Errorf("scenes = %v", scenes)
	}
}

func TestListScenesFiltersOwner(t *testing.T) {
	slug := "test_creator"
	videos := makeVideos(slug, 2)
	// Add a video from a different creator.
	other := video{Slug: "other-1", Title: "Other Scene"}
	other.Owner.Slug = "someone_else"
	other.Owner.DisplayName = "Someone Else"
	videos = append(videos, other)

	ts := newTestServer([][]video{videos})
	defer ts.Close()

	s := &Scraper{client: ts.Client(), base: ts.URL}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/test_creator", scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}

	scenes := testutil.CollectScenes(t, ch)
	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2 (other creator filtered out)", len(scenes))
	}
}
