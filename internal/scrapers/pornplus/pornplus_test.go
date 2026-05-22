package pornplus

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
	"github.com/Wasylq/FSS/scraper"
)

func TestMatchesURL(t *testing.T) {
	s := New()
	tests := []struct {
		url  string
		want bool
	}{
		{"https://pornplus.com/", true},
		{"https://www.pornplus.com/", true},
		{"https://pornplus.com/models/dolly-paige", true},
		{"https://pornplus.com/video/some-scene", true},
		{"https://puremature.com/", false},
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
		slug      string
	}{
		{"https://pornplus.com/", false, ""},
		{"https://pornplus.com/videos", false, ""},
		{"https://pornplus.com/models/dolly-paige", true, "dolly-paige"},
	}
	for _, tt := range tests {
		m := modelRe.FindStringSubmatch(tt.url)
		isModel := m != nil
		if isModel != tt.wantModel {
			t.Errorf("modelRe.Match(%q) = %v, want %v", tt.url, isModel, tt.wantModel)
		}
		if tt.wantModel && m[1] != tt.slug {
			t.Errorf("model slug = %q, want %q", m[1], tt.slug)
		}
	}
}

func makeTestScene(id int, slug, title, sponsorName, sponsorSlug string) apiScene {
	return apiScene{
		ID:         id,
		CachedSlug: slug,
		Title:      title,
		ReleasedAt: "2026-05-21T15:00:00Z",
		PosterURL:  "https://cdn-images.example.com/poster.jpg",
		ThumbURL:   "https://cdn-images.example.com/thumb.jpg",
		TrailerURL: "https://cdn-videos.example.com/trailer.mp4",
		Tags:       []string{"4k", "creampie"},
		Actors: []apiActor{
			{ID: 100, Name: "Dolly Paige", CachedSlug: "dolly-paige"},
		},
		Sponsor: apiSponsor{Name: sponsorName, CachedSlug: sponsorSlug},
		DownloadOptions: []struct {
			Quality string `json:"quality"`
		}{
			{Quality: "2160"},
			{Quality: "1080"},
			{Quality: "720"},
			{Quality: "480"},
		},
	}
}

func TestItemToScene(t *testing.T) {
	item := makeTestScene(76321, "rinse-the-day-away", "Rinse The Day Away", "Shower 4K", "shower-4k")
	now := time.Date(2026, 5, 22, 12, 0, 0, 0, time.UTC)

	scene := itemToScene(item, "https://pornplus.com/", now)

	if scene.ID != "76321" {
		t.Errorf("ID = %q, want 76321", scene.ID)
	}
	if scene.SiteID != "shower-4k" {
		t.Errorf("SiteID = %q, want shower-4k", scene.SiteID)
	}
	if scene.Title != "Rinse The Day Away" {
		t.Errorf("Title = %q", scene.Title)
	}
	if scene.URL != "https://pornplus.com/video/rinse-the-day-away" {
		t.Errorf("URL = %q", scene.URL)
	}
	if len(scene.Performers) != 1 || scene.Performers[0] != "Dolly Paige" {
		t.Errorf("Performers = %v", scene.Performers)
	}
	if len(scene.Tags) != 2 {
		t.Errorf("Tags = %v", scene.Tags)
	}
	if scene.Studio != "Shower 4K" {
		t.Errorf("Studio = %q, want Shower 4K", scene.Studio)
	}
	if scene.Resolution != "4K" {
		t.Errorf("Resolution = %q, want 4K", scene.Resolution)
	}
	if scene.Height != 2160 {
		t.Errorf("Height = %d, want 2160", scene.Height)
	}
	wantDate := time.Date(2026, 5, 21, 15, 0, 0, 0, time.UTC)
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

func TestSiteIDFromSponsor(t *testing.T) {
	tests := []struct {
		sponsorName string
		sponsorSlug string
		wantSiteID  string
		wantStudio  string
	}{
		{"Shower 4K", "shower-4k", "shower-4k", "Shower 4K"},
		{"Creepy PA", "creepy-pa", "creepy-pa", "Creepy PA"},
		{"", "", "pornplus", "Porn+"},
	}
	now := time.Now().UTC()
	for _, tt := range tests {
		item := apiScene{
			ID:         1,
			CachedSlug: "test",
			Sponsor:    apiSponsor{Name: tt.sponsorName, CachedSlug: tt.sponsorSlug},
		}
		scene := itemToScene(item, "https://pornplus.com/", now)
		if scene.SiteID != tt.wantSiteID {
			t.Errorf("sponsor=%q → SiteID=%q, want %q", tt.sponsorName, scene.SiteID, tt.wantSiteID)
		}
		if scene.Studio != tt.wantStudio {
			t.Errorf("sponsor=%q → Studio=%q, want %q", tt.sponsorName, scene.Studio, tt.wantStudio)
		}
	}
}

func TestTagCleaning(t *testing.T) {
	item := apiScene{
		ID:         1,
		CachedSlug: "test",
		Tags:       []string{"step_mom", "big_tits", "creampie"},
	}
	scene := itemToScene(item, "https://pornplus.com/", time.Now())
	want := []string{"step mom", "big tits", "creampie"}
	if fmt.Sprint(scene.Tags) != fmt.Sprint(want) {
		t.Errorf("Tags = %v, want %v", scene.Tags, want)
	}
}

func TestParseDate(t *testing.T) {
	tests := []struct {
		in   string
		want time.Time
	}{
		{"2026-05-21T15:00:00Z", time.Date(2026, 5, 21, 15, 0, 0, 0, time.UTC)},
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
	s1 := makeTestScene(100, "scene-a", "Scene A", "Shower 4K", "shower-4k")
	s2 := makeTestScene(101, "scene-b", "Scene B", "Creepy PA", "creepy-pa")
	page1 := fakeAPIResponse([]apiScene{s1, s2}, 2, false)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(page1)
	}))
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s := &Scraper{client: ts.Client()}
	out := make(chan scraper.SceneResult)
	go func() {
		defer close(out)
		s.paginate(ctx, ts.URL+"/api/releases?sort=latest", ts.URL, scraper.ListOpts{}, out)
	}()

	scenes := testutil.CollectScenes(t, out)
	if len(scenes) != 2 {
		t.Errorf("got %d scenes, want 2", len(scenes))
	}
}

func TestKnownIDsStopsEarly(t *testing.T) {
	apiScenes := []apiScene{
		makeTestScene(100, "a", "A", "Shower 4K", "shower-4k"),
		makeTestScene(101, "b", "B", "Creepy PA", "creepy-pa"),
		makeTestScene(102, "c", "C", "Sexercise", "sexercise"),
	}
	page := fakeAPIResponse(apiScenes, 3, false)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(page)
	}))
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s := &Scraper{client: ts.Client()}
	out := make(chan scraper.SceneResult)
	opts := scraper.ListOpts{KnownIDs: map[string]bool{"101": true}}
	go func() {
		defer close(out)
		s.paginate(ctx, ts.URL+"/api/releases?sort=latest", ts.URL, opts, out)
	}()

	scenes, stoppedEarly := testutil.CollectScenesWithStop(t, out)
	if len(scenes) != 1 {
		t.Errorf("got %d scenes before known ID, want 1", len(scenes))
	}
	if !stoppedEarly {
		t.Error("expected StoppedEarly")
	}
}

func TestScraperInterface(t *testing.T) {
	var _ scraper.StudioScraper = (*Scraper)(nil)
}
