package visitx

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
		{"https://www.visit-x.net/en/amateur/dirtytina/", true},
		{"https://www.visit-x.net/de/amateur/someone/", true},
		{"https://visit-x.net/en/amateur/dirtytina", true},
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
		{"https://www.visit-x.net/en/amateur/dirtytina/", "dirtytina"},
		{"https://visit-x.net/en/amateur/someone", "someone"},
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

// --- golden fixture ----------------------------------------------------------
//
// The other tests build gqlVideo values in Go, so encode and decode share the
// struct tag and a renamed one round-trips unnoticed. This is a live
// POST https://www.visit-x.net/vxql response, kept whole and unedited (3KB).
//
// Two steps are needed to reach it: GET the model page and scrape the
// `vxqlAccessToken` out of its HTML, then send it as a Bearer token. That token
// is a JWT and travels in a *request* header, so it never appears in the
// response — TestGoldenVideosCarriesNoToken asserts that rather than trusting it.
// No account or credential is involved.
//
// Shapes a hand-written fixture would have got wrong:
//   - **`duration` is a string ("148") even though the query asks for
//     `duration(format:sec)`.** gqlVideo.Duration is a string for that reason;
//     writing it as a JSON number passes a hand-built fixture and fails live.
//   - **`price.currency` is "VXC"**, VisitX's own token currency — not EUR or
//     USD. Scene prices from this site are therefore not comparable with other
//     sites' prices, and the value is small (13) because of it.
//   - `tagList` is an array of `{"label": …}` objects, not strings.
//   - `price` / `basePrice` / `preview` / `rating` / `model` are all **pointers**;
//     GraphQL returns null for absent ones and toScene nil-checks each.
//   - `released` is RFC3339 with an explicit +00:00 offset.
func TestGoldenVideosGQL(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("testdata", "videos_gql.json"))
	if err != nil {
		t.Fatal(err)
	}

	var resp gqlResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("decoding captured payload: %v", err)
	}
	if len(resp.Errors) != 0 {
		t.Fatalf("captured payload carries GraphQL errors: %+v", resp.Errors)
	}
	if resp.Data.Model == nil {
		t.Fatal("data.model is nil — the whole walk hangs off it")
	}
	m := resp.Data.Model
	if m.Name != "DirtyTina" {
		t.Errorf("model Name = %q (data.model.name)", m.Name)
	}
	if m.VideosV2.Total != 1294 {
		t.Errorf("Total = %d (videos_v2.total), want 1294 — pagination depends on it", m.VideosV2.Total)
	}
	if len(m.VideosV2.Items) != 2 {
		t.Fatalf("decoded %d items, want 2", len(m.VideosV2.Items))
	}

	v := m.VideosV2.Items[0]
	if v.ID != 28044121 {
		t.Errorf("ID = %d (id)", v.ID)
	}
	if v.Title == "" || v.Description == "" {
		t.Errorf("title/description empty: %q / %q", v.Title, v.Description)
	}
	if v.Slug == "" || v.LinkVX == "" {
		t.Errorf("slug/linkVX empty: %q / %q", v.Slug, v.LinkVX)
	}

	// duration as a numeric-looking *string*.
	if v.Duration != "148" {
		t.Errorf("Duration = %q (duration), want the string \"148\" — it is a string despite format:sec", v.Duration)
	}
	if _, err := strconv.Atoi(v.Duration); err != nil {
		t.Errorf("duration %q is no longer an integer string: %v", v.Duration, err)
	}

	if v.Released != "2026-07-15T07:00:00+00:00" {
		t.Errorf("Released = %q (released), want RFC3339 with an explicit offset", v.Released)
	}
	if _, err := time.Parse(time.RFC3339, v.Released); err != nil {
		t.Errorf("released %q does not parse as RFC3339: %v", v.Released, err)
	}

	// The nullable blocks, and the site's own currency.
	if v.Price == nil || v.BasePrice == nil {
		t.Fatalf("price/basePrice nil: %+v / %+v — both are pointers and toScene nil-checks them",
			v.Price, v.BasePrice)
	}
	if v.Price.Currency != "VXC" {
		t.Errorf("price.currency = %q, want VXC — VisitX prices are in its own token "+
			"currency, so they are not comparable with other sites'", v.Price.Currency)
	}
	if v.Price.Value == 0 {
		t.Error("price.value is 0")
	}
	if v.Preview == nil || len(v.Preview.Images) == 0 {
		t.Error("preview.images is empty — every scene loses its thumbnail")
	}
	if v.Rating == nil {
		t.Error("rating is nil")
	}
	if len(v.TagList) == 0 || v.TagList[0].Label == "" {
		t.Errorf("tagList = %+v — it is an array of {label} objects, not strings", v.TagList)
	}
}

func TestGoldenVideosCarriesNoToken(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("testdata", "videos_gql.json"))
	if err != nil {
		t.Fatal(err)
	}
	// The Bearer token is a JWT ("eyJ…") sent as a request header; it must never
	// land in a committed fixture.
	for _, marker := range []string{"eyJ0eXAiOi", "vxqlAccessToken", "Authorization", "Bearer "} {
		if bytes.Contains(body, []byte(marker)) {
			t.Errorf("fixture contains %q — re-capture without the credential", marker)
		}
	}
	if !bytes.Contains(body, []byte(`"duration":"148"`)) {
		t.Error(`fixture lost the quoted "duration":"148" — a re-encode may have made it numeric`)
	}
}
