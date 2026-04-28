package puretaboo

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
		{"https://www.puretaboo.com", true},
		{"https://puretaboo.com/en/videos", true},
		{"https://www.puretaboo.com/en/video/puretaboo/Under-My-Roof/285913", true},
		{"https://www.manyvids.com/Profile/123/foo", false},
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
		ClipID:      285913,
		Title:       "Under My Roof",
		Description: "A college girl is desperate.</br>She accepts an offer.",
		Length:      3154,
		ReleaseDate: "2026-04-21",
		URLTitle:    "Under-My-Roof",
		StudioName:  "Pure Taboo",
		SerieName:   "Pure Taboo",
		Actors: []gammautil.Actor{
			{ActorID: "50055", Name: "Charles Dera", Gender: "male"},
			{ActorID: "58057", Name: "Lexi Lore", Gender: "female"},
		},
		Directors: []gammautil.Director{
			{Name: "Seth Gamble"},
		},
		Categories: []gammautil.Category{
			{Name: "Blonde"},
			{Name: "Threesome"},
		},
		VideoFormats: []gammautil.VideoFormat{
			{Format: "720p", TrailerURL: "https://trailers.example.com/720p.mp4"},
			{Format: "1080p", TrailerURL: "https://trailers.example.com/1080p.mp4"},
			{Format: "2160p", TrailerURL: "https://trailers.example.com/4k.mp4"},
		},
		Pictures: gammautil.Pictures{
			Full1920: "/160696/160696_01/previews/2/239/top_1_1920x1080/160696_01_01.jpg",
		},
		MasterCategories: []string{"4k"},
		RatingsUp:        315,
	}

	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	scene := gammautil.ToScene(config, "https://www.puretaboo.com", hit, now)

	if scene.ID != "285913" {
		t.Errorf("ID = %q, want 285913", scene.ID)
	}
	if scene.SiteID != "puretaboo" {
		t.Errorf("SiteID = %q", scene.SiteID)
	}
	if scene.Title != "Under My Roof" {
		t.Errorf("Title = %q", scene.Title)
	}
	if scene.URL != "https://www.puretaboo.com/en/video/puretaboo/Under-My-Roof/285913" {
		t.Errorf("URL = %q", scene.URL)
	}
	if scene.Date.Year() != 2026 || scene.Date.Month() != 4 || scene.Date.Day() != 21 {
		t.Errorf("Date = %v", scene.Date)
	}
	if scene.Description != "A college girl is desperate.\nShe accepts an offer." {
		t.Errorf("Description = %q", scene.Description)
	}
	if scene.Thumbnail != gammautil.ImageCDN+"/160696/160696_01/previews/2/239/top_1_1920x1080/160696_01_01.jpg" {
		t.Errorf("Thumbnail = %q", scene.Thumbnail)
	}
	if scene.Preview != "https://trailers.example.com/4k.mp4" {
		t.Errorf("Preview = %q", scene.Preview)
	}
	if len(scene.Performers) != 2 || scene.Performers[0] != "Charles Dera" {
		t.Errorf("Performers = %v", scene.Performers)
	}
	if scene.Director != "Seth Gamble" {
		t.Errorf("Director = %q", scene.Director)
	}
	if scene.Studio != "Pure Taboo" {
		t.Errorf("Studio = %q", scene.Studio)
	}
	if len(scene.Tags) != 2 || scene.Tags[0] != "Blonde" {
		t.Errorf("Tags = %v", scene.Tags)
	}
	if scene.Series != "Pure Taboo" {
		t.Errorf("Series = %q", scene.Series)
	}
	if scene.Duration != 3154 {
		t.Errorf("Duration = %d, want 3154", scene.Duration)
	}
	if scene.Width != 3840 || scene.Height != 2160 {
		t.Errorf("Width=%d Height=%d", scene.Width, scene.Height)
	}
	if scene.Resolution != "2160p" {
		t.Errorf("Resolution = %q", scene.Resolution)
	}
	if scene.Likes != 315 {
		t.Errorf("Likes = %d", scene.Likes)
	}
}

func TestBestResolution(t *testing.T) {
	cases := []struct {
		name       string
		formats    []gammautil.VideoFormat
		masterCats []string
		wantW      int
		wantH      int
		wantRes    string
	}{
		{
			name:       "4k master category",
			formats:    []gammautil.VideoFormat{{Format: "1080p"}},
			masterCats: []string{"4k"},
			wantW:      3840, wantH: 2160, wantRes: "2160p",
		},
		{
			name:       "1080p from formats",
			formats:    []gammautil.VideoFormat{{Format: "720p"}, {Format: "1080p"}},
			masterCats: nil,
			wantW:      1920, wantH: 1080, wantRes: "1080p",
		},
		{
			name:       "720p only",
			formats:    []gammautil.VideoFormat{{Format: "720p"}},
			masterCats: nil,
			wantW:      1280, wantH: 720, wantRes: "720p",
		},
		{
			name:       "no formats",
			formats:    nil,
			masterCats: nil,
			wantW:      0, wantH: 0, wantRes: "",
		},
	}
	for _, c := range cases {
		w, h, res := gammautil.BestResolution(c.formats, c.masterCats)
		if w != c.wantW || h != c.wantH || res != c.wantRes {
			t.Errorf("%s: got %dx%d %q, want %dx%d %q", c.name, w, h, res, c.wantW, c.wantH, c.wantRes)
		}
	}
}

func TestParseDate(t *testing.T) {
	cases := []struct {
		input string
		year  int
		month time.Month
		day   int
	}{
		{"2026-04-21", 2026, 4, 21},
		{"2020-01-01", 2020, 1, 1},
		{"", 1, 1, 1},
	}
	for _, c := range cases {
		d := gammautil.ParseDate(c.input)
		if c.input == "" {
			if !d.IsZero() {
				t.Errorf("parseDate(%q) should be zero", c.input)
			}
			continue
		}
		if d.Year() != c.year || d.Month() != c.month || d.Day() != c.day {
			t.Errorf("parseDate(%q) = %v", c.input, d)
		}
	}
}

func TestThumbnailURL(t *testing.T) {
	cases := []struct {
		name string
		pics gammautil.Pictures
		want string
	}{
		{
			name: "full 1920",
			pics: gammautil.Pictures{Full1920: "/path/to/img.jpg"},
			want: gammautil.ImageCDN + "/path/to/img.jpg",
		},
		{
			name: "fallback to 638",
			pics: gammautil.Pictures{Res638: "/path/to/small.jpg"},
			want: gammautil.ImageCDN + "/path/to/small.jpg",
		},
		{
			name: "empty",
			pics: gammautil.Pictures{},
			want: "",
		},
	}
	for _, c := range cases {
		if got := gammautil.ThumbnailURL(c.pics); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

func TestDescriptionCleaning(t *testing.T) {
	hit := gammautil.AlgoliaHit{
		ClipID:      999,
		Title:       "Test",
		Description: "Line one.</br>Line two.<br>Line three.<br/>Line four.",
		ReleaseDate: "2026-01-01",
		URLTitle:    "Test",
	}
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	scene := gammautil.ToScene(config, "https://www.puretaboo.com", hit, now)

	want := "Line one.\nLine two.\nLine three.\nLine four."
	if scene.Description != want {
		t.Errorf("Description = %q, want %q", scene.Description, want)
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
			ClipID:      1001,
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
			ClipID:      1002,
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
		SiteID: "puretaboo", SiteBase: ts.URL, StudioName: "Pure Taboo", SiteName: "puretaboo",
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
		{ClipID: 2001, Title: "New Scene", ReleaseDate: "2026-02-01", URLTitle: "New-Scene"},
		{ClipID: 2002, Title: "Known Scene", ReleaseDate: "2026-01-01", URLTitle: "Known-Scene"},
	}

	ts := newTestServer(hits)
	defer ts.Close()

	cfg := gammautil.SiteConfig{
		SiteID: "puretaboo", SiteBase: ts.URL, StudioName: "Pure Taboo", SiteName: "puretaboo",
	}
	s := &Scraper{gamma: &gammautil.Scraper{Client: ts.Client(), Config: cfg, AlgoliaHost: ts.URL}}

	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{
		KnownIDs: map[string]bool{"2002": true},
	})
	if err != nil {
		t.Fatal(err)
	}

	scenes, stoppedEarly := testutil.CollectScenesWithStop(t, ch)

	if !stoppedEarly {
		t.Error("expected StoppedEarly signal")
	}
	if len(scenes) != 1 || scenes[0].ID != "2001" {
		t.Errorf("got %d scenes, want [2001] (known ID 2002 should stop)", len(scenes))
	}
}

func TestFetchAPIKey(t *testing.T) {
	ts := newTestServer(nil)
	defer ts.Close()

	cfg := gammautil.SiteConfig{
		SiteID: "puretaboo", SiteBase: ts.URL, StudioName: "Pure Taboo", SiteName: "puretaboo",
	}
	g := &gammautil.Scraper{Client: ts.Client(), Config: cfg, AlgoliaHost: ts.URL}

	key, err := g.FetchAPIKey(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if key != "testkey" {
		t.Errorf("apiKey = %q, want testkey", key)
	}
}
