package mommyblowsbest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wasylq/FSS/internal/scrapers/gammautil"
	"github.com/Wasylq/FSS/scraper"
)

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url  string
		want bool
	}{
		{"https://www.mommyblowsbest.com", true},
		{"https://mommyblowsbest.com/en/videos", true},
		{"https://www.mommyblowsbest.com/en/video/mommyblowsbest/Some-Scene/12345", true},
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
		ClipID:      287040,
		Title:       "Nurse's Special Care",
		Description: "Nurse Harper notices tension.</br>She helps.",
		Length:      1683,
		ReleaseDate: "2026-04-21",
		URLTitle:    "Nurses-Special-Care",
		StudioName:  "MommyBlowsBest",
		SerieName:   "MommyBlowsBest",
		Actors: []gammautil.Actor{
			{ActorID: "1", Name: "Sophia Locke", Gender: "female"},
			{ActorID: "2", Name: "Lucky Fate", Gender: "male"},
		},
		Directors: []gammautil.Director{
			{Name: "David Lord"},
		},
		Categories: []gammautil.Category{
			{Name: "Blowjob"},
			{Name: "MILF"},
		},
		VideoFormats: []gammautil.VideoFormat{
			{Format: "720p", TrailerURL: "https://trailers.example.com/720p.mp4"},
			{Format: "1080p", TrailerURL: "https://trailers.example.com/1080p.mp4"},
		},
		Pictures: gammautil.Pictures{
			Res638: "/img/287040.jpg",
		},
		RatingsUp: 50,
	}

	now := time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC)
	scene := gammautil.ToScene(config, "https://www.mommyblowsbest.com", hit, now)

	if scene.ID != "287040" {
		t.Errorf("ID = %q", scene.ID)
	}
	if scene.SiteID != "mommyblowsbest" {
		t.Errorf("SiteID = %q", scene.SiteID)
	}
	if scene.URL != "https://www.mommyblowsbest.com/en/video/mommyblowsbest/Nurses-Special-Care/287040" {
		t.Errorf("URL = %q", scene.URL)
	}
	if scene.Description != "Nurse Harper notices tension.\nShe helps." {
		t.Errorf("Description = %q", scene.Description)
	}
	if scene.Studio != "Mommy Blows Best" {
		t.Errorf("Studio = %q", scene.Studio)
	}
	if len(scene.Performers) != 2 || scene.Performers[0] != "Sophia Locke" {
		t.Errorf("Performers = %v", scene.Performers)
	}
	if scene.Director != "David Lord" {
		t.Errorf("Director = %q", scene.Director)
	}
	if scene.Duration != 1683 {
		t.Errorf("Duration = %d", scene.Duration)
	}
	if scene.Width != 1920 || scene.Height != 1080 {
		t.Errorf("Width=%d Height=%d", scene.Width, scene.Height)
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
			Categories:  []gammautil.Category{{Name: "Blowjob"}},
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
			Categories:  []gammautil.Category{{Name: "MILF"}},
			Pictures:    gammautil.Pictures{Res638: "/img/2.jpg"},
		},
	}

	ts := newTestServer(hits)
	defer ts.Close()

	cfg := gammautil.SiteConfig{
		SiteID: "mommyblowsbest", SiteBase: ts.URL, StudioName: "Mommy Blows Best", SiteName: "mommyblowsbest",
	}
	s := &Scraper{gamma: &gammautil.Scraper{Client: ts.Client(), Config: cfg, AlgoliaHost: ts.URL}}

	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}

	var scenes []string
	for r := range ch {
		if r.Kind == scraper.KindTotal || r.Kind == scraper.KindStoppedEarly {
			continue
		}
		if r.Err != nil {
			t.Errorf("unexpected error: %v", r.Err)
			continue
		}
		scenes = append(scenes, r.Scene.Title)
	}

	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2", len(scenes))
	}
	if scenes[0] != "Scene One" || scenes[1] != "Scene Two" {
		t.Errorf("scenes = %v", scenes)
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
		SiteID: "mommyblowsbest", SiteBase: ts.URL, StudioName: "Mommy Blows Best", SiteName: "mommyblowsbest",
	}
	s := &Scraper{gamma: &gammautil.Scraper{Client: ts.Client(), Config: cfg, AlgoliaHost: ts.URL}}

	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{
		KnownIDs: map[string]bool{"4002": true},
	})
	if err != nil {
		t.Fatal(err)
	}

	var scenes []string
	var stoppedEarly bool
	for r := range ch {
		if r.Total > 0 {
			continue
		}
		if r.Kind == scraper.KindStoppedEarly {
			stoppedEarly = true
			continue
		}
		if r.Err != nil {
			t.Errorf("unexpected error: %v", r.Err)
			continue
		}
		scenes = append(scenes, r.Scene.ID)
	}

	if !stoppedEarly {
		t.Error("expected StoppedEarly signal")
	}
	if len(scenes) != 1 || scenes[0] != "4001" {
		t.Errorf("got scenes %v, want [4001]", scenes)
	}
}

func TestScraperInterface(t *testing.T) {
	var _ scraper.StudioScraper = New()
}
