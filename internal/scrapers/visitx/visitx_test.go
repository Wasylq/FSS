package visitx

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
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
		{"https://www.visit-x.net/en/amateur/dirtytina/videos/", true},
		{"https://visit-x.net/en/amateur/dirtytina/videos/", true},
		{"https://www.visit-x.net/de/amateur/dirtytina/videos/", true},
		{"https://www.visit-x.net/es/amateur/someone/videos/", true},
		{"https://www.visit-x.net/en/amateur/dirtytina/videos/?page=2", true},
		{"https://www.visit-x.net/en/amateur/dirtytina/", false},
		{"https://www.visit-x.net/en/amateur/", false},
		{"https://example.com/en/amateur/x/videos/", false},
		{"", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.want {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.want)
		}
	}
}

func TestModelFromURL(t *testing.T) {
	cases := []struct {
		url  string
		want string
	}{
		{"https://www.visit-x.net/en/amateur/dirtytina/videos/", "dirtytina"},
		{"https://visit-x.net/de/amateur/SomeModel/videos/", "SomeModel"},
		{"https://www.visit-x.net/es/amateur/test-model/videos/?page=2", "test-model"},
	}
	for _, c := range cases {
		if got := modelFromURL(c.url); got != c.want {
			t.Errorf("modelFromURL(%q) = %q, want %q", c.url, got, c.want)
		}
	}
}

func TestToScene(t *testing.T) {
	v := gqlVideo{
		ID:          12345,
		Title:       "Test Video",
		Description: "A description",
		Duration:    "600",
		Released:    "2026-04-20T10:00:00+00:00",
		Free:        false,
		Slug:        "12345-test-video",
		LinkVX:      "https://www.visit-x.net/en/amateur/tester/videos/12345-test-video/",
		ViewCount:   42,
		Price:       &gqlPrice{Value: 15, Currency: "VXC"},
		BasePrice:   &gqlPrice{Value: 20, Currency: "VXC"},
		Preview:     &gqlPreview{Images: []gqlImage{{URL: "https://cdn.example.com/thumb.jpg"}}},
		TagList:     []gqlTag{{Label: "milf"}, {Label: "pov"}},
		Rating:      &gqlRating{Likes: 10, Dislikes: 2},
		Model:       &gqlVideoModel{Name: "Tester"},
	}

	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	sc := toScene(v, "https://www.visit-x.net/en/amateur/tester/videos/", now)

	if sc.ID != "12345" {
		t.Errorf("ID = %q", sc.ID)
	}
	if sc.SiteID != "visitx" {
		t.Errorf("SiteID = %q", sc.SiteID)
	}
	if sc.URL != "https://www.visit-x.net/en/amateur/tester/videos/12345-test-video/" {
		t.Errorf("URL = %q", sc.URL)
	}
	if sc.Title != "Test Video" {
		t.Errorf("Title = %q", sc.Title)
	}
	if sc.Duration != 600 {
		t.Errorf("Duration = %d", sc.Duration)
	}
	if sc.Date.Month() != 4 || sc.Date.Day() != 20 {
		t.Errorf("Date = %v", sc.Date)
	}
	if sc.Description != "A description" {
		t.Errorf("Description = %q", sc.Description)
	}
	if sc.Studio != "Tester" {
		t.Errorf("Studio = %q", sc.Studio)
	}
	if len(sc.Performers) != 1 || sc.Performers[0] != "Tester" {
		t.Errorf("Performers = %v", sc.Performers)
	}
	if sc.Thumbnail != "https://cdn.example.com/thumb.jpg" {
		t.Errorf("Thumbnail = %q", sc.Thumbnail)
	}
	if len(sc.Tags) != 2 || sc.Tags[0] != "milf" || sc.Tags[1] != "pov" {
		t.Errorf("Tags = %v", sc.Tags)
	}
	if sc.Views != 42 {
		t.Errorf("Views = %d", sc.Views)
	}
	if sc.Likes != 10 {
		t.Errorf("Likes = %d", sc.Likes)
	}
	if len(sc.PriceHistory) != 1 {
		t.Fatalf("PriceHistory len = %d", len(sc.PriceHistory))
	}
	ph := sc.PriceHistory[0]
	if ph.Regular != 20 {
		t.Errorf("Regular = %f", ph.Regular)
	}
	if !ph.IsOnSale {
		t.Error("expected IsOnSale")
	}
	if ph.Discounted != 15 {
		t.Errorf("Discounted = %f", ph.Discounted)
	}
	if ph.DiscountPercent != 25 {
		t.Errorf("DiscountPercent = %d", ph.DiscountPercent)
	}
}

func TestToSceneFree(t *testing.T) {
	v := gqlVideo{
		ID:       1,
		Title:    "Free Video",
		Duration: "120",
		Free:     true,
		LinkVX:   "https://www.visit-x.net/en/amateur/x/videos/1-free/",
	}
	sc := toScene(v, "https://www.visit-x.net/en/amateur/x/videos/", fixedTime())
	if len(sc.PriceHistory) != 1 || !sc.PriceHistory[0].IsFree {
		t.Errorf("expected IsFree, got %+v", sc.PriceHistory)
	}
}

func TestToSceneNoDiscount(t *testing.T) {
	v := gqlVideo{
		ID:        1,
		Title:     "Full Price",
		Duration:  "300",
		Free:      false,
		LinkVX:    "https://www.visit-x.net/en/amateur/x/videos/1-full/",
		Price:     &gqlPrice{Value: 20, Currency: "VXC"},
		BasePrice: &gqlPrice{Value: 20, Currency: "VXC"},
	}
	sc := toScene(v, "https://www.visit-x.net/en/amateur/x/videos/", fixedTime())
	ph := sc.PriceHistory[0]
	if ph.IsOnSale {
		t.Error("should not be on sale when price == basePrice")
	}
	if ph.Regular != 20 {
		t.Errorf("Regular = %f", ph.Regular)
	}
}

type testVideo struct {
	id       int
	title    string
	duration int
	date     string
	free     bool
	price    float64
}

func tokenPage() string {
	return `<html><head><script>window.VXConfig={"vxqlAccessToken":"test-jwt-token","accessTokenTTL":21600}</script></head><body></body></html>`
}

func gqlVideosResponse(videos []testVideo, total int, modelName string) []byte {
	items := make([]map[string]any, len(videos))
	for i, v := range videos {
		items[i] = map[string]any{
			"id":          v.id,
			"title":       v.title,
			"description": fmt.Sprintf("Desc for %s", v.title),
			"duration":    strconv.Itoa(v.duration),
			"released":    v.date,
			"free":        v.free,
			"slug":        fmt.Sprintf("%d-slug", v.id),
			"linkVX":      fmt.Sprintf("https://www.visit-x.net/en/amateur/%s/videos/%d-slug/", modelName, v.id),
			"viewCount":   10,
			"price":       map[string]any{"value": v.price, "currency": "VXC"},
			"basePrice":   map[string]any{"value": v.price, "currency": "VXC"},
			"preview":     map[string]any{"images": []map[string]string{{"url": "https://cdn/thumb.jpg"}}},
			"tagList":     []map[string]string{{"label": "tag1"}},
			"rating":      map[string]int{"likes": 5, "dislikes": 0},
			"model":       map[string]string{"name": modelName},
		}
	}
	resp := map[string]any{
		"data": map[string]any{
			"model": map[string]any{
				"id":   1,
				"name": modelName,
				"videos_v2": map[string]any{
					"total": total,
					"items": items,
				},
			},
		},
	}
	b, _ := json.Marshal(resp)
	return b
}

func newTestServer(modelName string, pages [][]testVideo, total int) *httptest.Server {
	pageIdx := 0
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Token page: any GET request (not to /vxql).
		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprint(w, tokenPage())
			return
		}

		// GraphQL endpoint.
		if r.URL.Path == "/vxql" && r.Method == http.MethodPost {
			w.Header().Set("Content-Type", "application/json")
			if pageIdx >= len(pages) {
				_, _ = w.Write(gqlVideosResponse(nil, total, modelName))
				return
			}
			vids := pages[pageIdx]
			pageIdx++
			_, _ = w.Write(gqlVideosResponse(vids, total, modelName))
			return
		}

		http.NotFound(w, r)
	}))
}

func TestListScenes(t *testing.T) {
	videos := []testVideo{
		{id: 100, title: "Scene One", duration: 600, date: "2026-04-20T10:00:00+00:00", price: 20},
		{id: 200, title: "Scene Two", duration: 900, date: "2026-04-15T10:00:00+00:00", price: 15},
	}

	ts := newTestServer("tester", [][]testVideo{videos}, 2)
	defer ts.Close()

	s := &Scraper{client: ts.Client(), base: ts.URL}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/en/amateur/tester/videos/", scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}

	results := testutil.CollectScenes(t, ch)
	if len(results) != 2 {
		t.Fatalf("got %d scenes, want 2", len(results))
	}
	if results[0].Title != "Scene One" {
		t.Errorf("first title = %q", results[0].Title)
	}
	if results[0].Duration != 600 {
		t.Errorf("first duration = %d", results[0].Duration)
	}
	if results[1].Title != "Scene Two" {
		t.Errorf("second title = %q", results[1].Title)
	}
}

func TestListScenesPagination(t *testing.T) {
	page1 := make([]testVideo, perPage)
	for i := range page1 {
		page1[i] = testVideo{
			id: i + 1, title: fmt.Sprintf("Scene %d", i+1),
			duration: 300, date: "2026-01-01T00:00:00+00:00", price: 10,
		}
	}
	page2 := []testVideo{
		{id: 101, title: "Scene 101", duration: 300, date: "2026-01-01T00:00:00+00:00", price: 10},
		{id: 102, title: "Scene 102", duration: 300, date: "2026-01-01T00:00:00+00:00", price: 10},
	}

	ts := newTestServer("tester", [][]testVideo{page1, page2}, 102)
	defer ts.Close()

	s := &Scraper{client: ts.Client(), base: ts.URL}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/en/amateur/tester/videos/", scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}

	results := testutil.CollectScenes(t, ch)
	if len(results) != 102 {
		t.Fatalf("got %d scenes, want 102", len(results))
	}
}

func TestListScenesKnownIDs(t *testing.T) {
	videos := []testVideo{
		{id: 1, title: "New", duration: 300, date: "2026-04-20T10:00:00+00:00", price: 10},
		{id: 2, title: "Also New", duration: 300, date: "2026-04-19T10:00:00+00:00", price: 10},
		{id: 3, title: "Known", duration: 300, date: "2026-04-18T10:00:00+00:00", price: 10},
		{id: 4, title: "Old", duration: 300, date: "2026-04-17T10:00:00+00:00", price: 10},
	}

	ts := newTestServer("tester", [][]testVideo{videos}, 4)
	defer ts.Close()

	s := &Scraper{client: ts.Client(), base: ts.URL}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/en/amateur/tester/videos/", scraper.ListOpts{
		KnownIDs: map[string]bool{"3": true},
	})
	if err != nil {
		t.Fatal(err)
	}

	results, stoppedEarly := testutil.CollectScenesWithStop(t, ch)
	if !stoppedEarly {
		t.Error("expected StoppedEarly signal")
	}
	if len(results) != 2 {
		t.Fatalf("got %d scenes, want 2", len(results))
	}
	if results[0].ID != "1" || results[1].ID != "2" {
		t.Errorf("scenes = %v, %v", results[0].ID, results[1].ID)
	}
}

func fixedTime() time.Time {
	return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
}
