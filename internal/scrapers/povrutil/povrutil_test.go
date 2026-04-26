package povrutil

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

	"github.com/Wasylq/FSS/scraper"
)

func testScraper(siteBase string) *Scraper {
	return &Scraper{
		Cfg: SiteConfig{
			ID:       "milfvr",
			Studio:   "MilfVR",
			SiteBase: siteBase,
			MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?milfvr\.com(/|$)`),
		},
	}
}

func TestMatchesURL(t *testing.T) {
	s := testScraper("https://www.milfvr.com")
	cases := []struct {
		url  string
		want bool
	}{
		{"https://www.milfvr.com", true},
		{"https://milfvr.com/", true},
		{"https://www.milfvr.com/some-scene-123", true},
		{"https://www.wankzvr.com", false},
		{"https://example.com", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.want {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.want)
		}
	}
}

func TestExtractID(t *testing.T) {
	cases := []struct {
		url, want string
	}{
		{"https://www.milfvr.com/house-of-the-rising-bum-6367985", "6367985"},
		{"https://www.wankzvr.com/working-her-fingers-to-the-moans-6367027", "6367027"},
		{"/scene-slug-12345", "12345"},
		{"/scene-slug-12345/", "12345"},
		{"https://www.milfvr.com/", ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := extractID(c.url); got != c.want {
			t.Errorf("extractID(%q) = %q, want %q", c.url, got, c.want)
		}
	}
}

func TestURLPath(t *testing.T) {
	cases := []struct {
		url, want string
	}{
		{"https://www.milfvr.com/some-scene-123", "/some-scene-123"},
		{"https://www.wankzvr.com/slug-456", "/slug-456"},
		{"https://example.com", ""},
		{"no-scheme", ""},
	}
	for _, c := range cases {
		if got := urlPath(c.url); got != c.want {
			t.Errorf("urlPath(%q) = %q, want %q", c.url, got, c.want)
		}
	}
}

func TestParseListingCards(t *testing.T) {
	html := []byte(`<div class="cards-list__item card "><div class="card__body"><a href="/house-of-the-rising-bum-6367985" class="card__video video"><div class="video__body"></div></a><div class="card__footer"><div class="card__h">House Of The Rising Bum</div><div class="card__inf"><div class="card__links"><a href="/sarah-jessie">Sarah Jessie</a></div><div class="card__date"><svg><use xlink:href="#date"></use></svg> 2 April, 2026</div></div></div></div>`)

	cards := parseListingCards(html)
	if len(cards) != 1 {
		t.Fatalf("got %d cards, want 1", len(cards))
	}
	c := cards[0]
	if c.path != "/house-of-the-rising-bum-6367985" {
		t.Errorf("path = %q", c.path)
	}
	if c.title != "House Of The Rising Bum" {
		t.Errorf("title = %q", c.title)
	}
	if len(c.performers) != 1 || c.performers[0] != "Sarah Jessie" {
		t.Errorf("performers = %v", c.performers)
	}
	if c.date.Year() != 2026 || c.date.Month() != 4 || c.date.Day() != 2 {
		t.Errorf("date = %v", c.date)
	}
}

func TestParseListingCardsMultiplePerformers(t *testing.T) {
	html := []byte(`<div class="cards-list__item card "><div class="card__body"><a href="/scene-999" class="card__video video"><div class="video__body"></div></a><div class="card__footer"><div class="card__h">A Scene</div><div class="card__inf"><div class="card__links"><a href="/actor-a">Actor A</a>, <a href="/actor-b">Actor B</a></div><div class="card__date"><svg><use xlink:href="#date"></use></svg> 15 January, 2026</div></div></div></div>`)

	cards := parseListingCards(html)
	if len(cards) != 1 {
		t.Fatalf("got %d cards", len(cards))
	}
	if len(cards[0].performers) != 2 || cards[0].performers[0] != "Actor A" || cards[0].performers[1] != "Actor B" {
		t.Errorf("performers = %v", cards[0].performers)
	}
}

func TestBuildScene(t *testing.T) {
	s := testScraper("https://www.milfvr.com")
	c := listingCard{
		path:       "/house-of-the-rising-bum-6367985",
		title:      "House Of The Rising Bum",
		performers: []string{"Sarah Jessie"},
		date:       time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC),
	}
	ev := exportVideo{
		URL:    "https://www.milfvr.com/house-of-the-rising-bum-6367985",
		Length: 2940,
		Tags:   []string{"Anal Sex", "Big Ass"},
		Thumb:  "https://cdns-i.milfvr.com/6/6367/6367985/hero/medium.jpg",
		Actors: []string{"Sarah Jessie"},
	}
	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)

	scene := s.buildScene(c, ev, "6367985", now)

	if scene.ID != "6367985" {
		t.Errorf("ID = %q", scene.ID)
	}
	if scene.SiteID != "milfvr" {
		t.Errorf("SiteID = %q", scene.SiteID)
	}
	if scene.Title != "House Of The Rising Bum" {
		t.Errorf("Title = %q", scene.Title)
	}
	if scene.URL != "https://www.milfvr.com/house-of-the-rising-bum-6367985" {
		t.Errorf("URL = %q", scene.URL)
	}
	if scene.Duration != 2940 {
		t.Errorf("Duration = %d", scene.Duration)
	}
	if len(scene.Tags) != 2 || scene.Tags[0] != "Anal Sex" {
		t.Errorf("Tags = %v", scene.Tags)
	}
	if scene.Thumbnail != "https://cdns-i.milfvr.com/6/6367/6367985/hero/medium.jpg" {
		t.Errorf("Thumbnail = %q", scene.Thumbnail)
	}
	if len(scene.Performers) != 1 || scene.Performers[0] != "Sarah Jessie" {
		t.Errorf("Performers = %v", scene.Performers)
	}
	if scene.Studio != "MilfVR" {
		t.Errorf("Studio = %q", scene.Studio)
	}
	if scene.Date.Year() != 2026 || scene.Date.Month() != 4 || scene.Date.Day() != 2 {
		t.Errorf("Date = %v", scene.Date)
	}
}

func TestBuildSceneNoExport(t *testing.T) {
	s := testScraper("https://www.milfvr.com")
	c := listingCard{
		path:       "/some-scene-123",
		title:      "Some Scene",
		performers: []string{"Actor"},
		date:       time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)

	scene := s.buildScene(c, exportVideo{}, "123", now)
	if scene.Duration != 0 {
		t.Errorf("Duration = %d, want 0", scene.Duration)
	}
	if len(scene.Tags) != 0 {
		t.Errorf("Tags = %v, want empty", scene.Tags)
	}
}

func TestListScenes(t *testing.T) {
	export := []exportVideo{
		{
			URL:    "%s/scene-one-100",
			Length: 600,
			Tags:   []string{"Tag1"},
			Thumb:  "https://cdn.example.com/thumb1.jpg",
			Actors: []string{"Actor A"},
		},
	}
	listingPage := `<div class="cards-list__item card "><div class="card__body"><a href="/scene-one-100" class="card__video video"><div class="video__body"></div></a><div class="card__footer"><div class="card__h">Scene One</div><div class="card__inf"><div class="card__links"><a href="/actor-a">Actor A</a></div><div class="card__date"><svg><use xlink:href="#date"></use></svg> 20 April, 2026</div></div></div></div>`

	var ts *httptest.Server
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/export/videos.json":
			vids := make([]exportVideo, len(export))
			copy(vids, export)
			vids[0].URL = fmt.Sprintf(export[0].URL, ts.URL)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(vids)
		default:
			if r.URL.Query().Get("p") == "2" {
				_, _ = w.Write([]byte(`<html><body></body></html>`))
				return
			}
			_, _ = w.Write([]byte(listingPage))
		}
	}))
	defer ts.Close()

	s := &Scraper{
		Cfg: SiteConfig{
			ID: "milfvr", Studio: "MilfVR", SiteBase: ts.URL,
			MatchRe: regexp.MustCompile(`.*`),
		},
		Client: ts.Client(),
	}

	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}

	var scenes []string
	for r := range ch {
		if r.StoppedEarly {
			continue
		}
		if r.Err != nil {
			t.Errorf("unexpected error: %v", r.Err)
			continue
		}
		scenes = append(scenes, r.Scene.Title)
		if r.Scene.Duration != 600 {
			t.Errorf("Duration = %d, want 600", r.Scene.Duration)
		}
		if len(r.Scene.Tags) != 1 || r.Scene.Tags[0] != "Tag1" {
			t.Errorf("Tags = %v", r.Scene.Tags)
		}
	}

	if len(scenes) != 1 || scenes[0] != "Scene One" {
		t.Errorf("scenes = %v, want [Scene One]", scenes)
	}
}

func TestListScenesKnownIDs(t *testing.T) {
	listingPage := `
<div class="cards-list__item card "><div class="card__body"><a href="/new-scene-200" class="card__video video"><div class="video__body"></div></a><div class="card__footer"><div class="card__h">New Scene</div><div class="card__inf"><div class="card__links"><a href="/x">X</a></div><div class="card__date"><svg><use xlink:href="#date"></use></svg> 20 April, 2026</div></div></div></div>
<div class="cards-list__item card "><div class="card__body"><a href="/known-scene-199" class="card__video video"><div class="video__body"></div></a><div class="card__footer"><div class="card__h">Known Scene</div><div class="card__inf"><div class="card__links"><a href="/y">Y</a></div><div class="card__date"><svg><use xlink:href="#date"></use></svg> 19 April, 2026</div></div></div></div>`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/export/videos.json":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[]`))
		default:
			_, _ = w.Write([]byte(listingPage))
		}
	}))
	defer ts.Close()

	s := &Scraper{
		Cfg: SiteConfig{
			ID: "milfvr", Studio: "MilfVR", SiteBase: ts.URL,
			MatchRe: regexp.MustCompile(`.*`),
		},
		Client: ts.Client(),
	}

	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{
		KnownIDs: map[string]bool{"199": true},
	})
	if err != nil {
		t.Fatal(err)
	}

	var titles []string
	stoppedEarly := false
	for r := range ch {
		if r.StoppedEarly {
			stoppedEarly = true
			continue
		}
		if r.Err != nil {
			t.Errorf("error: %v", r.Err)
			continue
		}
		titles = append(titles, r.Scene.Title)
	}

	if !stoppedEarly {
		t.Error("expected StoppedEarly")
	}
	if len(titles) != 1 || titles[0] != "New Scene" {
		t.Errorf("titles = %v, want [New Scene]", titles)
	}
}
