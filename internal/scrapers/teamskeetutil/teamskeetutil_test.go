package teamskeetutil

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
		{"https://www.teamskeet.com/series/exxxtrasmall", filterSeries, "exxxtrasmall"},
		{"https://www.teamskeet.com/models/alex-adams", filterModel, "alex-adams"},
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
		if !jsonContains(s, `"_doc_type.keyword":"tour_movie"`) {
			t.Error("missing doc_type filter")
		}
		if !jsonContains(s, `"isUpcoming":false`) {
			t.Error("missing isUpcoming filter")
		}
	})

	t.Run("model", func(t *testing.T) {
		q := buildQuery(filterModel, "penny-barber")
		data, _ := json.Marshal(q)
		if !jsonContains(string(data), `"models.id.keyword":"penny-barber"`) {
			t.Error("missing model filter")
		}
	})

	t.Run("series", func(t *testing.T) {
		q := buildQuery(filterSeries, "exxxtrasmall")
		data, _ := json.Marshal(q)
		if !jsonContains(string(data), `"site.nickName.keyword":"exxxtrasmall"`) {
			t.Error("missing series filter")
		}
	})

	t.Run("category", func(t *testing.T) {
		q := buildQuery(filterCategory, "amateur")
		data, _ := json.Marshal(q)
		if !jsonContains(string(data), `"tags.keyword"`) {
			t.Error("missing category filter")
		}
	})
}

func jsonContains(s, sub string) bool {
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
	scene := hitToScene(src, "https://www.mylf.com/", "https://www.mylf.com", now)

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
	if scene.SiteID != "mylf-singles-ts" {
		t.Errorf("SiteID = %q, want mylf-singles-ts", scene.SiteID)
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
	resp := struct {
		Hits struct {
			Total struct {
				Value int `json:"value"`
			} `json:"total"`
			Hits []esHit `json:"hits"`
		} `json:"hits"`
	}{}
	resp.Hits.Total.Value = total
	for _, s := range scenes {
		dateBytes, _ := json.Marshal(s.PublishedDate)
		idBytes, _ := json.Marshal(s.ItemID)
		resp.Hits.Hits = append(resp.Hits.Hits, esHit{
			Source: s,
			Sort:   []json.RawMessage{dateBytes, idBytes},
		})
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

func TestRunUsesSearchAfter(t *testing.T) {
	var requests []map[string]any

	page1Scenes := make([]esScene, pageSize)
	for i := range page1Scenes {
		page1Scenes[i] = makeScene(1000-i, fmt.Sprintf("Scene %d", i))
		page1Scenes[i].Site.Name = "TestSite"
		page1Scenes[i].Site.NickName = "testsite"
	}
	page2Scenes := []esScene{makeScene(500, "Last Scene")}
	page2Scenes[0].Site.Name = "TestSite"
	page2Scenes[0].Site.NickName = "testsite"

	page1 := fakeESResponse(page1Scenes, pageSize+1)
	page2 := fakeESResponse(page2Scenes, pageSize+1)
	empty := fakeESResponse(nil, pageSize+1)

	call := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		requests = append(requests, body)
		switch call {
		case 0:
			_, _ = w.Write(page1)
		case 1:
			_, _ = w.Write(page2)
		default:
			_, _ = w.Write(empty)
		}
		call++
	}))
	defer ts.Close()

	s := &Scraper{
		client:    ts.Client(),
		Config:    SiteConfig{Index: "test", SiteBase: ts.URL, Domain: "test.com", SiteID: "test"},
		esBaseURL: ts.URL,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	out := make(chan scraper.SceneResult, 100)
	go s.Run(ctx, ts.URL, scraper.ListOpts{Workers: 3}, out)

	var scenes int
	for r := range out {
		if r.Kind == scraper.KindScene {
			scenes++
		}
	}

	if scenes != pageSize+1 {
		t.Errorf("got %d scenes, want %d", scenes, pageSize+1)
	}
	if len(requests) < 2 {
		t.Fatalf("got %d requests, want at least 2", len(requests))
	}

	if _, ok := requests[0]["from"]; ok {
		t.Error("first request should not have 'from' (using search_after)")
	}
	if _, ok := requests[0]["search_after"]; ok {
		t.Error("first request should not have search_after")
	}

	if _, ok := requests[1]["search_after"]; !ok {
		t.Error("second request missing search_after cursor")
	}
	if _, ok := requests[1]["from"]; ok {
		t.Error("second request should not have 'from'")
	}
}

func TestSearchParsesResponse(t *testing.T) {
	page1 := fakeESResponse([]esScene{makeScene(100, "Scene A"), makeScene(101, "Scene B")}, 2)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(page1)
	}))
	defer ts.Close()

	s := &Scraper{
		client: ts.Client(),
		Config: SiteConfig{Index: "test", SiteBase: ts.URL},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	origBase := esBase
	_ = origBase

	result, err := s.searchWithURL(ctx, ts.URL, map[string]any{
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
}
