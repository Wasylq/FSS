package manyvids

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/scraper"
)

// ---- MatchesURL ----

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url   string
		match bool
	}{
		{"https://www.manyvids.com/Profile/590705/bettie-bondage/Store/Videos", true},
		{"http://manyvids.com/Profile/123/some-creator/Store/Videos", true},
		{"https://www.manyvids.com/Profile/590705/bettie-bondage/Store/Videos/", true},
		{"https://www.manyvids.com/Profile/590705/bettie-bondage", false},
		{"https://clips4sale.com/studio/123", false},
		{"https://www.manyvids.com/Video/7342578/fostering-the-bully", false},
		{"", false},
	}
	for _, c := range cases {
		got := s.MatchesURL(c.url)
		if got != c.match {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.match)
		}
	}
}

// ---- creatorID ----

func TestCreatorID(t *testing.T) {
	cases := []struct {
		url     string
		want    string
		wantErr bool
	}{
		{"https://www.manyvids.com/Profile/590705/bettie-bondage/Store/Videos", "590705", false},
		{"https://www.manyvids.com/Profile/999/creator/Store/Videos", "999", false},
		{"https://clips4sale.com/studio/123", "", true},
	}
	for _, c := range cases {
		got, err := creatorID(c.url)
		if (err != nil) != c.wantErr {
			t.Errorf("creatorID(%q) error = %v, wantErr %v", c.url, err, c.wantErr)
			continue
		}
		if got != c.want {
			t.Errorf("creatorID(%q) = %q, want %q", c.url, got, c.want)
		}
	}
}

// ---- parseDuration ----

func TestParseDuration(t *testing.T) {
	cases := []struct {
		input string
		want  int
	}{
		{"33:31", 2011},
		{"0:30", 30},
		{"1:00:00", 3600},
		{"1:05:30", 3930},
		{"15:29", 929},
		{"0:00", 0},
		{"", 0},
	}
	for _, c := range cases {
		got := parseDuration(c.input)
		if got != c.want {
			t.Errorf("parseDuration(%q) = %d, want %d", c.input, got, c.want)
		}
	}
}

// ---- toScene (uses real fixture data from ManyVids API) ----

// fixtureDetail mirrors the real GET /store/video/7342578 response.
var fixtureDetail = detailItem{
	ID:            "7342578",
	Title:         "Fostering the Bully",
	LaunchDate:    "2026-03-07T02:26:19.000Z",
	VideoDuration: "33:31",
	Description:   "You've been bounced around a lot this last year",
	TagList:       []mvTag{{Label: "Bully"}, {Label: "Cougar"}, {Label: "MILF"}, {Label: "POV"}},
	Screenshot:    "https://ods.manyvids.com/590705/58aad4459ab6757ec96dd40457280085-131/screenshots/custom_1_360_69ab8992ea079.jpg",
	Model:         mvModel{DisplayName: "Bettie Bondage"},
	Resolution:    "4K",
	Width:         3840,
	Height:        2160,
	Extension:     "MP4",
	URL:           "/Video/7342578/fostering-the-bully",
	ViewsRaw:      7666,
	LikesRaw:      242,
	Comments:      12,
	Price:         mvPrice{Free: false, OnSale: true, Regular: "29.99", DiscountedPrice: "23.99", PromoRate: 20},
}

const (
	testStudioURL  = "https://www.manyvids.com/Profile/590705/bettie-bondage/Store/Videos"
	testPreviewURL = "https://ods.manyvids.com/590705/preview.mp4"
)

func TestToScene(t *testing.T) {
	now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
	scene, err := toScene(testStudioURL, defaultSiteBase, fixtureDetail, testPreviewURL, now)
	if err != nil {
		t.Fatalf("toScene: %v", err)
	}

	str := func(field, got, want string) {
		t.Helper()
		if got != want {
			t.Errorf("%s = %q, want %q", field, got, want)
		}
	}
	num := func(field string, got, want int) {
		t.Helper()
		if got != want {
			t.Errorf("%s = %d, want %d", field, got, want)
		}
	}

	str("ID", scene.ID, "7342578")
	str("SiteID", scene.SiteID, "manyvids")
	str("StudioURL", scene.StudioURL, testStudioURL)
	str("Title", scene.Title, "Fostering the Bully")
	str("URL", scene.URL, "https://www.manyvids.com/Video/7342578/fostering-the-bully")
	str("Thumbnail", scene.Thumbnail, fixtureDetail.Screenshot)
	str("Preview", scene.Preview, testPreviewURL)
	str("Studio", scene.Studio, "Bettie Bondage")
	str("Resolution", scene.Resolution, "4K")
	str("Format", scene.Format, "MP4")
	num("Width", scene.Width, 3840)
	num("Height", scene.Height, 2160)
	num("Duration", scene.Duration, 2011)
	num("Views", scene.Views, 7666)
	num("Likes", scene.Likes, 242)
	num("Comments", scene.Comments, 12)

	if len(scene.Performers) != 1 || scene.Performers[0] != "Bettie Bondage" {
		t.Errorf("Performers = %v, want [Bettie Bondage]", scene.Performers)
	}

	wantTags := []string{"Bully", "Cougar", "MILF", "POV"}
	if len(scene.Tags) != len(wantTags) {
		t.Errorf("Tags = %v, want %v", scene.Tags, wantTags)
	} else {
		for i, tag := range wantTags {
			if scene.Tags[i] != tag {
				t.Errorf("Tags[%d] = %q, want %q", i, scene.Tags[i], tag)
			}
		}
	}

	wantDate := time.Date(2026, 3, 7, 2, 26, 19, 0, time.UTC)
	if !scene.Date.Equal(wantDate) {
		t.Errorf("Date = %v, want %v", scene.Date, wantDate)
	}

	if len(scene.PriceHistory) != 1 {
		t.Fatalf("PriceHistory len = %d, want 1", len(scene.PriceHistory))
	}
	p := scene.PriceHistory[0]
	if p.Regular != 29.99 {
		t.Errorf("Price.Regular = %v, want 29.99", p.Regular)
	}
	if p.Discounted != 23.99 {
		t.Errorf("Price.Discounted = %v, want 23.99", p.Discounted)
	}
	if !p.IsOnSale {
		t.Error("Price.IsOnSale = false, want true")
	}
	if p.DiscountPercent != 20 {
		t.Errorf("Price.DiscountPercent = %d, want 20", p.DiscountPercent)
	}
	if scene.LowestPrice != 23.99 {
		t.Errorf("LowestPrice = %v, want 23.99", scene.LowestPrice)
	}
}

// ---- AddPrice lowest-price tracking ----

func TestAddPriceLowestTracking(t *testing.T) {
	now := time.Now().UTC()
	scene, _ := toScene(testStudioURL, defaultSiteBase, fixtureDetail, testPreviewURL, now)
	// Effective price after first scrape is 23.99 (on sale).

	// Second scrape: price goes up, lowest should not change.
	scene.AddPrice(models.PriceSnapshot{Date: now, Regular: 35.00, IsFree: false, IsOnSale: false})
	if scene.LowestPrice != 23.99 {
		t.Errorf("LowestPrice after price increase = %v, want 23.99", scene.LowestPrice)
	}

	// Third scrape: drops to a new low.
	lower := now.Add(time.Hour)
	scene.AddPrice(models.PriceSnapshot{Date: lower, Regular: 29.99, Discounted: 9.99, IsOnSale: true})
	if scene.LowestPrice != 9.99 {
		t.Errorf("LowestPrice after new low = %v, want 9.99", scene.LowestPrice)
	}
	if scene.LowestPriceDate == nil || !scene.LowestPriceDate.Equal(lower) {
		t.Errorf("LowestPriceDate not updated correctly, got %v", scene.LowestPriceDate)
	}
	if len(scene.PriceHistory) != 3 {
		t.Errorf("PriceHistory len = %d, want 3", len(scene.PriceHistory))
	}
}

// ---- fetchPage via httptest ----

func TestFetchPage(t *testing.T) {
	want := listResponse{
		StatusCode: 200,
		Data: []listItem{
			{ID: "111", Preview: struct {
				URL string `json:"url"`
			}{URL: "https://cdn.example.com/111.mp4"}},
			{ID: "222", Preview: struct {
				URL string `json:"url"`
			}{URL: "https://cdn.example.com/222.mp4"}},
		},
		Pagination: pagination{TotalPages: 2, CurrentPage: 1, NextPage: 2},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(want)
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client(), apiBase: ts.URL, siteBase: defaultSiteBase}
	entries, totalPages, err := s.fetchPage(context.Background(), "590705", 1)
	if err != nil {
		t.Fatalf("fetchPage: %v", err)
	}
	if totalPages != 2 {
		t.Errorf("totalPages = %d, want 2", totalPages)
	}
	if len(entries) != 2 {
		t.Fatalf("entries len = %d, want 2", len(entries))
	}
	if entries[0].id != "111" || entries[1].id != "222" {
		t.Errorf("entry IDs = %q %q", entries[0].id, entries[1].id)
	}
	if entries[0].previewURL != "https://cdn.example.com/111.mp4" {
		t.Errorf("previewURL = %q", entries[0].previewURL)
	}
}

// ---- ListScenes end-to-end via httptest ----

func TestListScenes(t *testing.T) {
	detailFixtures := map[string]detailItem{
		"111": {ID: "111", Title: "Scene One", VideoDuration: "10:00",
			Model: mvModel{DisplayName: "Alice"}, Price: mvPrice{Regular: "9.99"},
			LaunchDate: "2026-01-01T00:00:00Z"},
		"222": {ID: "222", Title: "Scene Two", VideoDuration: "20:00",
			Model: mvModel{DisplayName: "Alice"}, Price: mvPrice{Regular: "14.99"},
			LaunchDate: "2026-01-02T00:00:00Z"},
		"333": {ID: "333", Title: "Scene Three", VideoDuration: "30:00",
			Model: mvModel{DisplayName: "Alice"}, Price: mvPrice{Regular: "19.99"},
			LaunchDate: "2025-12-01T00:00:00Z"},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if strings.Contains(r.URL.Path, "/store/videos/") {
			var resp listResponse
			switch r.URL.Query().Get("page") {
			case "", "1":
				resp = listResponse{
					StatusCode: 200,
					Data:       []listItem{{ID: "111"}, {ID: "222"}},
					Pagination: pagination{TotalPages: 2, CurrentPage: 1, NextPage: 2},
				}
			case "2":
				resp = listResponse{
					StatusCode: 200,
					Data:       []listItem{{ID: "333"}},
					Pagination: pagination{TotalPages: 2, CurrentPage: 2},
				}
			default:
				http.NotFound(w, r)
				return
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		// /store/video/{id}
		parts := strings.Split(strings.TrimSuffix(r.URL.Path, "/"), "/")
		id := parts[len(parts)-1]
		item, ok := detailFixtures[id]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(detailResponse{StatusCode: 200, Data: item})
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client(), apiBase: ts.URL, siteBase: defaultSiteBase}
	ch, err := s.ListScenes(context.Background(), testStudioURL, scraper.ListOpts{Workers: 2})
	if err != nil {
		t.Fatalf("ListScenes: %v", err)
	}

	got := map[string]string{}
	for result := range ch {
		if result.Err != nil {
			t.Errorf("unexpected error: %v", result.Err)
			continue
		}
		got[result.Scene.ID] = result.Scene.Title
	}

	if len(got) != 3 {
		t.Fatalf("got %d scenes, want 3: %v", len(got), got)
	}
	want := map[string]string{"111": "Scene One", "222": "Scene Two", "333": "Scene Three"}
	for id, title := range want {
		if got[id] != title {
			t.Errorf("scene %s title = %q, want %q", id, got[id], title)
		}
	}
}

// ---- ListScenes early-stop on KnownIDs ----

func TestListScenesKnownIDs(t *testing.T) {
	detailFixtures := map[string]detailItem{
		"111": {ID: "111", Title: "Fresh", VideoDuration: "10:00",
			Model: mvModel{DisplayName: "Alice"}, Price: mvPrice{Regular: "9.99"},
			LaunchDate: "2026-01-03T00:00:00Z"},
		"222": {ID: "222", Title: "Known", VideoDuration: "20:00",
			Model: mvModel{DisplayName: "Alice"}, Price: mvPrice{Regular: "14.99"},
			LaunchDate: "2026-01-02T00:00:00Z"},
		"333": {ID: "333", Title: "Older known", VideoDuration: "30:00",
			Model: mvModel{DisplayName: "Alice"}, Price: mvPrice{Regular: "19.99"},
			LaunchDate: "2026-01-01T00:00:00Z"},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "/store/videos/") {
			resp := listResponse{
				StatusCode: 200,
				Data:       []listItem{{ID: "111"}, {ID: "222"}, {ID: "333"}},
				Pagination: pagination{TotalPages: 1, CurrentPage: 1},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		parts := strings.Split(strings.TrimSuffix(r.URL.Path, "/"), "/")
		id := parts[len(parts)-1]
		item, ok := detailFixtures[id]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(detailResponse{StatusCode: 200, Data: item})
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client(), apiBase: ts.URL, siteBase: defaultSiteBase}
	// Workers: 1 keeps detail-fetch order deterministic so we can verify that
	// only the fresh scene was fetched, not the ones past the known marker.
	ch, err := s.ListScenes(context.Background(), testStudioURL, scraper.ListOpts{
		Workers:  1,
		KnownIDs: map[string]bool{"222": true},
	})
	if err != nil {
		t.Fatalf("ListScenes: %v", err)
	}

	var scenes []scraper.SceneResult
	sawStoppedEarly := false
	for r := range ch {
		if r.StoppedEarly {
			sawStoppedEarly = true
			continue
		}
		if r.Err != nil {
			t.Errorf("unexpected error: %v", r.Err)
			continue
		}
		scenes = append(scenes, r)
	}

	if len(scenes) != 1 {
		t.Fatalf("got %d scenes, want 1 (early stop at known ID)", len(scenes))
	}
	if scenes[0].Scene.ID != "111" {
		t.Errorf("scene ID = %q, want %q", scenes[0].Scene.ID, "111")
	}
	if !sawStoppedEarly {
		t.Error("expected StoppedEarly signal, got none")
	}
}
