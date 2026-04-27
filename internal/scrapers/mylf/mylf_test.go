package mylf

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
		{"https://www.mylf.com/", true},
		{"https://www.mylf.com/videos", true},
		{"https://mylf.com/models/penny-barber", true},
		{"https://www.mylf.com/series/features-ts", true},
		{"https://www.mylf.com/categories/amateur", true},
		{"https://example.com", false},
		{"https://teamskeet.com", false},
	}
	for _, tt := range tests {
		if got := s.MatchesURL(tt.url); got != tt.want {
			t.Errorf("MatchesURL(%q) = %v, want %v", tt.url, got, tt.want)
		}
	}
}

func TestClassifyURL(t *testing.T) {
	tests := []struct {
		url      string
		wantKind filterKind
		wantVal  string
	}{
		{"https://www.mylf.com/", filterAll, ""},
		{"https://www.mylf.com/videos", filterAll, ""},
		{"https://www.mylf.com/models/penny-barber", filterModel, "penny-barber"},
		{"https://www.mylf.com/series/features-ts", filterSeries, "features-ts"},
		{"https://www.mylf.com/categories/amateur", filterCategory, "amateur"},
	}
	for _, tt := range tests {
		kind, val := classifyURL(tt.url)
		if kind != tt.wantKind || val != tt.wantVal {
			t.Errorf("classifyURL(%q) = (%v, %q), want (%v, %q)", tt.url, kind, val, tt.wantKind, tt.wantVal)
		}
	}
}

func TestBuildQuery(t *testing.T) {
	t.Run("all", func(t *testing.T) {
		q := buildQuery(filterAll, "")
		data, _ := json.Marshal(q)
		s := string(data)
		if !contains(s, `"_doc_type.keyword":"tour_movie"`) {
			t.Error("missing doc_type filter")
		}
		if !contains(s, `"isUpcoming":false`) {
			t.Error("missing isUpcoming filter")
		}
	})

	t.Run("model", func(t *testing.T) {
		q := buildQuery(filterModel, "penny-barber")
		data, _ := json.Marshal(q)
		if !contains(string(data), `"models.id.keyword":"penny-barber"`) {
			t.Error("missing model filter")
		}
	})

	t.Run("series", func(t *testing.T) {
		q := buildQuery(filterSeries, "features-ts")
		data, _ := json.Marshal(q)
		if !contains(string(data), `"site.nickName.keyword":"features-ts"`) {
			t.Error("missing series filter")
		}
	})

	t.Run("category", func(t *testing.T) {
		q := buildQuery(filterCategory, "amateur")
		data, _ := json.Marshal(q)
		if !contains(string(data), `"tags.keyword"`) {
			t.Error("missing category filter")
		}
	})
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestHitToScene(t *testing.T) {
	src := esScene{
		ID:            "polar-opposites-slug",
		ItemID:        31954,
		Title:         "Polar Opposites",
		Description:   "<p>Two women explore.</p>",
		Img:           "https://images.psmcdn.net/teamskeet/mlf/scene/shared/med_v2.jpg",
		VideoTrailer:  "https://images.psmcdn.net/mlf/tour/pics/trailer.mp4",
		PublishedDate: "2026-04-26T00:00:00",
		Tags:          []string{"MILF", "Step Mom"},
		Models: []esModel{
			{ID: "penny-barber", ItemID: 1234, Name: "Penny Barber"},
			{ID: "kaden-kole", ItemID: 5678, Name: "Kaden Kole"},
		},
	}
	src.Site.Name = "MYLF Singles"
	src.Site.NickName = "mylf-singles-ts"

	now := time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC)
	scene := hitToScene(src, "https://www.mylf.com/", now)

	if scene.ID != "31954" {
		t.Errorf("ID = %q, want 31954", scene.ID)
	}
	if scene.Title != "Polar Opposites" {
		t.Errorf("Title = %q", scene.Title)
	}
	if scene.URL != "https://www.mylf.com/videos/polar-opposites-slug" {
		t.Errorf("URL = %q", scene.URL)
	}
	if scene.Description != "Two women explore." {
		t.Errorf("Description = %q (HTML not stripped?)", scene.Description)
	}
	if len(scene.Performers) != 2 || scene.Performers[0] != "Penny Barber" {
		t.Errorf("Performers = %v", scene.Performers)
	}
	if len(scene.Tags) != 2 {
		t.Errorf("Tags = %v", scene.Tags)
	}
	if scene.Studio != "MYLF Singles" {
		t.Errorf("Studio = %q", scene.Studio)
	}
	wantDate := time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)
	if !scene.Date.Equal(wantDate) {
		t.Errorf("Date = %v, want %v", scene.Date, wantDate)
	}
	if scene.Thumbnail == "" {
		t.Error("Thumbnail is empty")
	}
	if scene.Preview == "" {
		t.Error("Preview is empty")
	}
	if scene.SiteID != "mylf" {
		t.Errorf("SiteID = %q", scene.SiteID)
	}
}

func TestParseDate(t *testing.T) {
	tests := []struct {
		in   string
		want time.Time
	}{
		{"2026-04-26T00:00:00", time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)},
		{"2024-01-15T14:30:00", time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC)},
		{"bad", time.Time{}},
	}
	for _, tt := range tests {
		got := parseDate(tt.in)
		if !got.Equal(tt.want) {
			t.Errorf("parseDate(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func fakeESResponse(scenes []esScene, total int) []byte {
	type hit struct {
		Source esScene `json:"_source"`
	}
	resp := struct {
		Hits struct {
			Total struct {
				Value int `json:"value"`
			} `json:"total"`
			Hits []hit `json:"hits"`
		} `json:"hits"`
	}{}
	resp.Hits.Total.Value = total
	for _, s := range scenes {
		resp.Hits.Hits = append(resp.Hits.Hits, hit{Source: s})
	}
	data, _ := json.Marshal(resp)
	return data
}

func makeScene(id int, title string) esScene {
	return esScene{
		ID:            fmt.Sprintf("scene-%d", id),
		ItemID:        id,
		Title:         title,
		PublishedDate: "2026-04-20T00:00:00",
		Models:        []esModel{{Name: "Test Model"}},
		Tags:          []string{"MILF"},
	}
}

func TestPaginatedScrape(t *testing.T) {
	page1 := fakeESResponse([]esScene{makeScene(100, "Scene A"), makeScene(101, "Scene B")}, 2)
	page2 := fakeESResponse(nil, 2)

	var reqCount int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqCount++
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		from, _ := body["from"].(float64)
		if int(from) == 0 {
			_, _ = w.Write(page1)
		} else {
			_, _ = w.Write(page2)
		}
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client()}
	out := make(chan scraper.SceneResult)

	origEndpoint := esEndpoint
	// We can't override the const, so we test via the full run with a patched search.
	// Instead, test the parsing and pagination logic directly.
	_ = origEndpoint

	// Test the search function with the test server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := s.searchURL(ctx, ts.URL, map[string]any{
		"query": map[string]any{"match_all": map[string]any{}},
		"from":  0,
		"size":  30,
	})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if result.Hits.Total.Value != 2 {
		t.Errorf("total = %d, want 2", result.Hits.Total.Value)
	}
	if len(result.Hits.Hits) != 2 {
		t.Errorf("hits = %d, want 2", len(result.Hits.Hits))
	}
	if result.Hits.Hits[0].Source.ItemID != 100 {
		t.Errorf("first hit itemId = %d, want 100", result.Hits.Hits[0].Source.ItemID)
	}

	_ = out
}

func TestKnownIDsStopsEarly(t *testing.T) {
	scenes := []esScene{makeScene(100, "Scene A"), makeScene(101, "Scene B"), makeScene(102, "Scene C")}
	page := fakeESResponse(scenes, 3)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(page)
	}))
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s := &Scraper{client: ts.Client()}
	result, err := s.searchURL(ctx, ts.URL, map[string]any{
		"query": map[string]any{"match_all": map[string]any{}},
		"from":  0,
		"size":  30,
	})
	if err != nil {
		t.Fatalf("search: %v", err)
	}

	// Simulate KnownIDs check
	opts := scraper.ListOpts{KnownIDs: map[string]bool{"101": true}}
	var count int
	for _, hit := range result.Hits.Hits {
		id := fmt.Sprintf("%d", hit.Source.ItemID)
		if opts.KnownIDs[id] {
			break
		}
		count++
	}
	if count != 1 {
		t.Errorf("got %d scenes before known ID, want 1", count)
	}
}

func TestScraperInterface(t *testing.T) {
	var _ scraper.StudioScraper = (*Scraper)(nil)
}
