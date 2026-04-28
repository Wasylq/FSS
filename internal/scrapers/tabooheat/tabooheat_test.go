package tabooheat

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wasylq/FSS/internal/scrapers/gammautil"
	"github.com/Wasylq/FSS/internal/scrapers/testutil"
	"github.com/Wasylq/FSS/scraper"
)

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url  string
		want bool
	}{
		{"https://www.tabooheat.com", true},
		{"https://tabooheat.com/en/videos", true},
		{"https://www.tabooheat.com/en/video/tabooheat/Some-Scene/12345", true},
		{"https://www.puretaboo.com", false},
		{"https://example.com", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.want {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.want)
		}
	}
}

func TestToScene(t *testing.T) {
	hit := gammautil.AlgoliaHit{
		ClipID:      50001,
		Title:       "Family Therapy",
		Description: "A family seeks help.</br>Things escalate.",
		Length:      2400,
		ReleaseDate: "2026-03-15",
		URLTitle:    "Family-Therapy",
		StudioName:  "Taboo Heat",
		SerieName:   "Taboo Heat",
		Actors: []gammautil.Actor{
			{ActorID: "10001", Name: "Cory Chase", Gender: "female"},
			{ActorID: "10002", Name: "Luke Longly", Gender: "male"},
		},
		Directors: []gammautil.Director{
			{Name: "Cory Chase"},
		},
		Categories: []gammautil.Category{
			{Name: "MILF"},
			{Name: "Taboo"},
		},
		VideoFormats: []gammautil.VideoFormat{
			{Format: "720p", TrailerURL: "https://trailers.example.com/720p.mp4"},
			{Format: "1080p", TrailerURL: "https://trailers.example.com/1080p.mp4"},
		},
		Pictures: gammautil.Pictures{
			Res638: "/img/50001.jpg",
		},
		RatingsUp: 120,
	}

	now := time.Date(2026, 3, 20, 12, 0, 0, 0, time.UTC)
	scene := gammautil.ToScene(config, "https://www.tabooheat.com", hit, now)

	if scene.ID != "50001" {
		t.Errorf("ID = %q, want 50001", scene.ID)
	}
	if scene.SiteID != "tabooheat" {
		t.Errorf("SiteID = %q", scene.SiteID)
	}
	if scene.Title != "Family Therapy" {
		t.Errorf("Title = %q", scene.Title)
	}
	if scene.URL != "https://www.tabooheat.com/en/video/tabooheat/Family-Therapy/50001" {
		t.Errorf("URL = %q", scene.URL)
	}
	if scene.Date.Year() != 2026 || scene.Date.Month() != 3 || scene.Date.Day() != 15 {
		t.Errorf("Date = %v", scene.Date)
	}
	if scene.Description != "A family seeks help.\nThings escalate." {
		t.Errorf("Description = %q", scene.Description)
	}
	if scene.Thumbnail != gammautil.ImageCDN+"/img/50001.jpg" {
		t.Errorf("Thumbnail = %q", scene.Thumbnail)
	}
	if scene.Preview != "https://trailers.example.com/1080p.mp4" {
		t.Errorf("Preview = %q", scene.Preview)
	}
	if len(scene.Performers) != 2 || scene.Performers[0] != "Cory Chase" {
		t.Errorf("Performers = %v", scene.Performers)
	}
	if scene.Director != "Cory Chase" {
		t.Errorf("Director = %q", scene.Director)
	}
	if scene.Studio != "Taboo Heat" {
		t.Errorf("Studio = %q", scene.Studio)
	}
	if len(scene.Tags) != 2 || scene.Tags[0] != "MILF" {
		t.Errorf("Tags = %v", scene.Tags)
	}
	if scene.Series != "Taboo Heat" {
		t.Errorf("Series = %q", scene.Series)
	}
	if scene.Duration != 2400 {
		t.Errorf("Duration = %d, want 2400", scene.Duration)
	}
	if scene.Width != 1920 || scene.Height != 1080 {
		t.Errorf("Width=%d Height=%d", scene.Width, scene.Height)
	}
	if scene.Resolution != "1080p" {
		t.Errorf("Resolution = %q", scene.Resolution)
	}
	if scene.Likes != 120 {
		t.Errorf("Likes = %d", scene.Likes)
	}
}

func newTestServer(hits []gammautil.AlgoliaHit) *httptest.Server {
	homePage := `<html><body>
<script>window.env={"api":{"algolia":{"applicationID":"TEST","apiKey":"testkey"}}}</script>
</body></html>`

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/en/videos":
			_, _ = w.Write([]byte(homePage))
		case "/1/indexes/all_scenes_latest_desc/query":
			resp := gammautil.AlgoliaResponse{Hits: hits, NbHits: len(hits), NbPages: 1}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		default:
			http.NotFound(w, r)
		}
	}))
}

func TestListScenes(t *testing.T) {
	hits := []gammautil.AlgoliaHit{
		{
			ClipID:      3001,
			Title:       "Scene One",
			Description: "First scene",
			Length:      600,
			ReleaseDate: "2026-01-15",
			URLTitle:    "Scene-One",
			Actors:      []gammautil.Actor{{Name: "Actor A"}},
			Categories:  []gammautil.Category{{Name: "Tag1"}},
			Pictures:    gammautil.Pictures{Res638: "/img/1.jpg"},
		},
		{
			ClipID:      3002,
			Title:       "Scene Two",
			Description: "Second scene",
			Length:      900,
			ReleaseDate: "2026-01-10",
			URLTitle:    "Scene-Two",
			Actors:      []gammautil.Actor{{Name: "Actor B"}},
			Categories:  []gammautil.Category{{Name: "Tag2"}},
			Pictures:    gammautil.Pictures{Res638: "/img/2.jpg"},
		},
	}

	ts := newTestServer(hits)
	defer ts.Close()

	cfg := gammautil.SiteConfig{
		SiteID: "tabooheat", SiteBase: ts.URL, StudioName: "Taboo Heat", SiteName: "tabooheat",
	}
	s := &Scraper{gamma: &gammautil.Scraper{Client: ts.Client(), Config: cfg, AlgoliaHost: ts.URL}}

	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}

	scenes := testutil.CollectScenes(t, ch)

	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2", len(scenes))
	}
	if scenes[0].Title != "Scene One" || scenes[1].Title != "Scene Two" {
		t.Errorf("scenes = %v %v", scenes[0].Title, scenes[1].Title)
	}
}

func TestListScenesKnownIDs(t *testing.T) {
	hits := []gammautil.AlgoliaHit{
		{ClipID: 4001, Title: "New Scene", ReleaseDate: "2026-02-01", URLTitle: "New-Scene"},
		{ClipID: 4002, Title: "Known Scene", ReleaseDate: "2026-01-01", URLTitle: "Known-Scene"},
	}

	ts := newTestServer(hits)
	defer ts.Close()

	cfg := gammautil.SiteConfig{
		SiteID: "tabooheat", SiteBase: ts.URL, StudioName: "Taboo Heat", SiteName: "tabooheat",
	}
	s := &Scraper{gamma: &gammautil.Scraper{Client: ts.Client(), Config: cfg, AlgoliaHost: ts.URL}}

	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{
		KnownIDs: map[string]bool{"4002": true},
	})
	if err != nil {
		t.Fatal(err)
	}

	scenes, stoppedEarly := testutil.CollectScenesWithStop(t, ch)

	if !stoppedEarly {
		t.Error("expected StoppedEarly signal")
	}
	if len(scenes) != 1 || scenes[0].ID != "4001" {
		t.Errorf("got %d scenes, want [4001] (known ID 4002 should stop)", len(scenes))
	}
}
