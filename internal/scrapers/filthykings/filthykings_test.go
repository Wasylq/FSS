package filthykings

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
		{"https://www.filthykings.com", true},
		{"https://filthykings.com/en/videos", true},
		{"https://www.filthykings.com/en/video/filthykings/Some-Scene/12345", true},
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
		StudioName:  "Filthy Kings",
		SerieName:   "",
		Actors: []gammautil.Actor{
			{ActorID: "1", Name: "Performer A", Gender: "female"},
		},
		Directors: []gammautil.Director{
			{Name: "Director X"},
		},
		Categories: []gammautil.Category{
			{Name: "Taboo"},
		},
		VideoFormats: []gammautil.VideoFormat{
			{Format: "1080p", TrailerURL: "https://trailers.example.com/1080p.mp4"},
		},
		Pictures: gammautil.Pictures{
			Res638: "/img/100001.jpg",
		},
		RatingsUp: 200,
	}

	now := time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC)
	scene := gammautil.ToScene(config, "https://www.filthykings.com", hit, now)

	if scene.ID != "100001" {
		t.Errorf("ID = %q", scene.ID)
	}
	if scene.SiteID != "filthykings" {
		t.Errorf("SiteID = %q", scene.SiteID)
	}
	if scene.URL != "https://www.filthykings.com/en/video/filthykings/Test-Scene/100001" {
		t.Errorf("URL = %q", scene.URL)
	}
	if scene.Studio != "Filthy Kings" {
		t.Errorf("Studio = %q", scene.Studio)
	}
	if scene.Width != 1920 || scene.Height != 1080 {
		t.Errorf("Width=%d Height=%d", scene.Width, scene.Height)
	}
	if scene.Resolution != "1080p" {
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
			Categories:  []gammautil.Category{{Name: "Taboo"}},
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
		SiteID: "filthykings", SiteBase: ts.URL, StudioName: "Filthy Kings", SiteName: "filthykings",
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
}

func TestListScenesKnownIDs(t *testing.T) {
	hits := []gammautil.AlgoliaHit{
		{ClipID: 6001, Title: "New", ReleaseDate: "2026-02-01", URLTitle: "New"},
		{ClipID: 6002, Title: "Known", ReleaseDate: "2026-01-01", URLTitle: "Known"},
	}

	ts := newTestServer(hits)
	defer ts.Close()

	cfg := gammautil.SiteConfig{
		SiteID: "filthykings", SiteBase: ts.URL, StudioName: "Filthy Kings", SiteName: "filthykings",
	}
	s := &Scraper{gamma: &gammautil.Scraper{Client: ts.Client(), Config: cfg, AlgoliaHost: ts.URL}}

	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{
		KnownIDs: map[string]bool{"6002": true},
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
		t.Error("expected StoppedEarly")
	}
	if len(scenes) != 1 || scenes[0] != "6001" {
		t.Errorf("got scenes %v, want [6001]", scenes)
	}
}

func TestScraperInterface(t *testing.T) {
	var _ scraper.StudioScraper = New()
}
