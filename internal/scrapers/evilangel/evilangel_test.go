package evilangel

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
		{"https://www.evilangel.com", true},
		{"https://evilangel.com/en/videos", true},
		{"https://www.evilangel.com/en/video/evilangel/Some-Scene/12345", true},
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
		ClipID:      100001,
		Title:       "Test Scene",
		Description: "A test.</br>More text.",
		Length:      2400,
		ReleaseDate: "2026-04-20",
		URLTitle:    "Test-Scene",
		StudioName:  "Evil Angel",
		SerieName:   "",
		Actors: []gammautil.Actor{
			{ActorID: "1", Name: "Performer A", Gender: "female"},
		},
		Directors: []gammautil.Director{
			{Name: "Director X"},
		},
		Categories: []gammautil.Category{
			{Name: "Anal"},
		},
		VideoFormats: []gammautil.VideoFormat{
			{Format: "2160p", TrailerURL: "https://trailers.example.com/4k.mp4"},
		},
		Pictures: gammautil.Pictures{
			Res638: "/img/100001.jpg",
		},
		MasterCategories: []string{"4k"},
		RatingsUp:        200,
	}

	now := time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC)
	scene := gammautil.ToScene(config, "https://www.evilangel.com", hit, now)

	if scene.ID != "100001" {
		t.Errorf("ID = %q", scene.ID)
	}
	if scene.SiteID != "evilangel" {
		t.Errorf("SiteID = %q", scene.SiteID)
	}
	if scene.URL != "https://www.evilangel.com/en/video/evilangel/Test-Scene/100001" {
		t.Errorf("URL = %q", scene.URL)
	}
	if scene.Studio != "Evil Angel" {
		t.Errorf("Studio = %q", scene.Studio)
	}
	if scene.Width != 3840 || scene.Height != 2160 {
		t.Errorf("Width=%d Height=%d", scene.Width, scene.Height)
	}
	if scene.Resolution != "2160p" {
		t.Errorf("Resolution = %q", scene.Resolution)
	}
	if scene.Director != "Director X" {
		t.Errorf("Director = %q", scene.Director)
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
			ClipID:      5001,
			Title:       "Scene One",
			Length:      600,
			ReleaseDate: "2026-01-15",
			URLTitle:    "Scene-One",
			Actors:      []gammautil.Actor{{Name: "Actor A"}},
			Categories:  []gammautil.Category{{Name: "Anal"}},
			Pictures:    gammautil.Pictures{Res638: "/img/1.jpg"},
		},
		{
			ClipID:      5002,
			Title:       "Scene Two",
			Length:      900,
			ReleaseDate: "2026-01-10",
			URLTitle:    "Scene-Two",
			Actors:      []gammautil.Actor{{Name: "Actor B"}},
			Pictures:    gammautil.Pictures{Res638: "/img/2.jpg"},
		},
	}

	ts := newTestServer(hits)
	defer ts.Close()

	cfg := gammautil.SiteConfig{
		SiteID: "evilangel", SiteBase: ts.URL, StudioName: "Evil Angel", SiteName: "evilangel",
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
}

func TestListScenesKnownIDs(t *testing.T) {
	hits := []gammautil.AlgoliaHit{
		{ClipID: 6001, Title: "New", ReleaseDate: "2026-02-01", URLTitle: "New"},
		{ClipID: 6002, Title: "Known", ReleaseDate: "2026-01-01", URLTitle: "Known"},
	}

	ts := newTestServer(hits)
	defer ts.Close()

	cfg := gammautil.SiteConfig{
		SiteID: "evilangel", SiteBase: ts.URL, StudioName: "Evil Angel", SiteName: "evilangel",
	}
	s := &Scraper{gamma: &gammautil.Scraper{Client: ts.Client(), Config: cfg, AlgoliaHost: ts.URL}}

	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{
		KnownIDs: map[string]bool{"6002": true},
	})
	if err != nil {
		t.Fatal(err)
	}

	scenes, stoppedEarly := testutil.CollectScenesWithStop(t, ch)
	if !stoppedEarly {
		t.Error("expected StoppedEarly")
	}
	if len(scenes) != 1 || scenes[0].ID != "6001" {
		t.Errorf("got %d scenes, want 1 with ID 6001", len(scenes))
	}
}

func TestScraperInterface(t *testing.T) {
	var _ scraper.StudioScraper = New()
}
