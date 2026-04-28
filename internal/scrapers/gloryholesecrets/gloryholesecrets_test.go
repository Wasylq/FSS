package gloryholesecrets

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
		{"https://www.gloryholesecrets.com", true},
		{"https://gloryholesecrets.com/en/videos", true},
		{"https://www.gloryholesecrets.com/en/video/gloryholesecrets/Serena-Hill-First-Gloryhole/287153", true},
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
		ClipID:      287153,
		Title:       "Serena Hill First Gloryhole",
		Description: "Serena Hill is here for her first gloryhole.</br>She loves it.",
		Length:      2690,
		ReleaseDate: "2026-04-22",
		URLTitle:    "Serena-Hill-First-Gloryhole",
		StudioName:  "Gloryhole Secrets",
		SerieName:   "First Glory Hole",
		Actors: []gammautil.Actor{
			{ActorID: "12345", Name: "Serena Hill", Gender: "female"},
		},
		Categories: []gammautil.Category{
			{Name: "Blowjob"},
			{Name: "Brunette"},
		},
		VideoFormats: []gammautil.VideoFormat{
			{Format: "720p"},
			{Format: "1080p"},
			{Format: "2160p", TrailerURL: "https://trailers.example.com/4k.mp4"},
		},
		Pictures: gammautil.Pictures{
			Full1920: "/161595/161595_01/previews/2/655/top_1_1920x1080/161595_01_01.jpg",
		},
		MasterCategories: []string{"4k"},
		RatingsUp:        42,
	}

	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	scene := gammautil.ToScene(config, "https://www.gloryholesecrets.com", hit, now)

	if scene.ID != "287153" {
		t.Errorf("ID = %q", scene.ID)
	}
	if scene.SiteID != "gloryholesecrets" {
		t.Errorf("SiteID = %q", scene.SiteID)
	}
	if scene.Title != "Serena Hill First Gloryhole" {
		t.Errorf("Title = %q", scene.Title)
	}
	if scene.URL != "https://www.gloryholesecrets.com/en/video/gloryholesecrets/Serena-Hill-First-Gloryhole/287153" {
		t.Errorf("URL = %q", scene.URL)
	}
	if scene.Date.Year() != 2026 || scene.Date.Month() != 4 || scene.Date.Day() != 22 {
		t.Errorf("Date = %v", scene.Date)
	}
	if scene.Description != "Serena Hill is here for her first gloryhole.\nShe loves it." {
		t.Errorf("Description = %q", scene.Description)
	}
	if scene.Thumbnail != gammautil.ImageCDN+"/161595/161595_01/previews/2/655/top_1_1920x1080/161595_01_01.jpg" {
		t.Errorf("Thumbnail = %q", scene.Thumbnail)
	}
	if len(scene.Performers) != 1 || scene.Performers[0] != "Serena Hill" {
		t.Errorf("Performers = %v", scene.Performers)
	}
	if scene.Studio != "Gloryhole Secrets" {
		t.Errorf("Studio = %q", scene.Studio)
	}
	if len(scene.Tags) != 2 || scene.Tags[0] != "Blowjob" {
		t.Errorf("Tags = %v", scene.Tags)
	}
	if scene.Series != "First Glory Hole" {
		t.Errorf("Series = %q", scene.Series)
	}
	if scene.Duration != 2690 {
		t.Errorf("Duration = %d", scene.Duration)
	}
	if scene.Width != 3840 || scene.Height != 2160 {
		t.Errorf("Width=%d Height=%d", scene.Width, scene.Height)
	}
	if scene.Likes != 42 {
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
		SiteID: "gloryholesecrets", SiteBase: ts.URL, StudioName: "Gloryhole Secrets", SiteName: "gloryholesecrets",
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
		SiteID: "gloryholesecrets", SiteBase: ts.URL, StudioName: "Gloryhole Secrets", SiteName: "gloryholesecrets",
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
		t.Errorf("got %d scenes, want 1 with ID 4001", len(scenes))
	}
}
