package veutil

import (
	"context"
	"encoding/json"
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
			ID:             "mypervmom",
			Studio:         "PervMom",
			SiteBase:       siteBase,
			MainCategoryID: 1,
			MatchRe:        regexp.MustCompile(`^https?://(?:www\.)?mypervmom\.com(/|$)`),
		},
	}
}

func TestMatchesURL(t *testing.T) {
	s := testScraper("https://mypervmom.com")
	cases := []struct {
		url  string
		want bool
	}{
		{"https://mypervmom.com", true},
		{"https://mypervmom.com/", true},
		{"https://www.mypervmom.com/some-scene", true},
		{"https://example.com", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.want {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.want)
		}
	}
}

func TestExtractPoster(t *testing.T) {
	cases := []struct {
		content, want string
	}{
		{`<video poster="https://cdn.example.com/thumb.jpg"><source src="video.mp4"></video>`, "https://cdn.example.com/thumb.jpg"},
		{`<p>no video here</p>`, ""},
	}
	for _, c := range cases {
		if got := extractPoster(c.content); got != c.want {
			t.Errorf("extractPoster(%q) = %q, want %q", c.content[:20], got, c.want)
		}
	}
}

func TestPostToScene(t *testing.T) {
	s := testScraper("https://mypervmom.com")
	tagMap := map[int]string{10: "Daisy Stone", 20: "Joshua Lewis"}
	p := wpPost{
		ID:      2319,
		DateGMT: "2026-04-26T11:37:34",
		Link:    "https://mypervmom.com/sex-therapy-at-home/",
		Title:   wpRendered{Rendered: "Sex Therapy At Home &#8211; S3:E2"},
		Content: wpRendered{Rendered: `<p><video poster="https://cdn.example.com/thumb.jpg"><source src="video.mp4"></video></p>`},
		Tags:    []int{10, 20},
	}

	scene := s.postToScene(p, tagMap, fixedTime())

	if scene.ID != "2319" {
		t.Errorf("ID = %q", scene.ID)
	}
	if scene.Title != "Sex Therapy At Home – S3:E2" {
		t.Errorf("Title = %q", scene.Title)
	}
	if scene.URL != "https://mypervmom.com/sex-therapy-at-home/" {
		t.Errorf("URL = %q", scene.URL)
	}
	if scene.Thumbnail != "https://cdn.example.com/thumb.jpg" {
		t.Errorf("Thumbnail = %q", scene.Thumbnail)
	}
	if scene.Date.Year() != 2026 || scene.Date.Month() != 4 || scene.Date.Day() != 26 {
		t.Errorf("Date = %v", scene.Date)
	}
	if len(scene.Performers) != 2 || scene.Performers[0] != "Daisy Stone" {
		t.Errorf("Performers = %v", scene.Performers)
	}
	if scene.Studio != "PervMom" {
		t.Errorf("Studio = %q", scene.Studio)
	}
}

func newTestServer(tags []wpTag, posts []wpPost) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/wp-json/wp/v2/tags":
			w.Header().Set("X-WP-Total", "2")
			_ = json.NewEncoder(w).Encode(tags)
		case "/wp-json/wp/v2/posts":
			page := r.URL.Query().Get("page")
			if page == "2" {
				w.Header().Set("X-WP-Total", "0")
				_, _ = w.Write([]byte("[]"))
				return
			}
			w.Header().Set("X-WP-Total", itoa(len(posts)))
			_ = json.NewEncoder(w).Encode(posts)
		default:
			http.NotFound(w, r)
		}
	}))
}

func itoa(n int) string {
	return string(rune('0' + n))
}

func TestListScenes(t *testing.T) {
	tags := []wpTag{{ID: 10, Name: "Daisy Stone"}}
	posts := []wpPost{
		{
			ID: 100, DateGMT: "2026-04-26T11:00:00",
			Link:    "https://mypervmom.com/scene-one/",
			Title:   wpRendered{Rendered: "Scene One"},
			Content: wpRendered{Rendered: `<video poster="https://cdn.example.com/1.jpg"></video>`},
			Tags:    []int{10},
		},
		{
			ID: 99, DateGMT: "2026-04-25T10:00:00",
			Link:    "https://mypervmom.com/scene-two/",
			Title:   wpRendered{Rendered: "Scene Two"},
			Content: wpRendered{Rendered: `<video poster="https://cdn.example.com/2.jpg"></video>`},
			Tags:    []int{10},
		},
	}

	ts := newTestServer(tags, posts)
	defer ts.Close()

	s := &Scraper{
		Cfg: SiteConfig{
			ID: "mypervmom", Studio: "PervMom", SiteBase: ts.URL,
			MainCategoryID: 1, MatchRe: regexp.MustCompile(`.*`),
		},
		Client: ts.Client(),
	}

	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}

	var scenes []string
	for r := range ch {
		if r.Total > 0 || r.StoppedEarly {
			continue
		}
		if r.Err != nil {
			t.Errorf("unexpected error: %v", r.Err)
			continue
		}
		scenes = append(scenes, r.Scene.Title)
	}

	if len(scenes) != 2 || scenes[0] != "Scene One" || scenes[1] != "Scene Two" {
		t.Errorf("scenes = %v", scenes)
	}
}

func TestListScenesKnownIDs(t *testing.T) {
	tags := []wpTag{{ID: 10, Name: "Actor"}}
	posts := []wpPost{
		{ID: 200, DateGMT: "2026-04-26T11:00:00", Link: "https://mypervmom.com/new/", Title: wpRendered{Rendered: "New"}, Tags: []int{10}},
		{ID: 199, DateGMT: "2026-04-25T10:00:00", Link: "https://mypervmom.com/known/", Title: wpRendered{Rendered: "Known"}, Tags: []int{10}},
	}

	ts := newTestServer(tags, posts)
	defer ts.Close()

	s := &Scraper{
		Cfg: SiteConfig{
			ID: "mypervmom", Studio: "PervMom", SiteBase: ts.URL,
			MainCategoryID: 1, MatchRe: regexp.MustCompile(`.*`),
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
		if r.Total > 0 {
			continue
		}
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
	if len(titles) != 1 || titles[0] != "New" {
		t.Errorf("titles = %v, want [New]", titles)
	}
}

func fixedTime() time.Time {
	return time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
}
