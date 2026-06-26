package hustler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wasylq/FSS/scraper"
)

// videosJSON returns a fixture for the /wp-json/wp/v2/videos endpoint with a
// single fully-populated video and a second minimal one.
const videosJSON = `[
  {
    "id": 1234,
    "date": "2026-01-23T08:30:00",
    "link": "https://hustlerunlimited.com/videos/worshipping-princess/",
    "title": { "rendered": "Worshipping &amp; Princess" },
    "_embedded": {
      "wp:term": [
        [
          { "name": "Dee Williams", "taxonomy": "hu_actors" },
          { "name": "Natasha Ty", "taxonomy": "hu_actors" }
        ],
        [
          { "name": "Anal", "taxonomy": "video_tags" },
          { "name": "Blonde", "taxonomy": "video_tags" }
        ],
        [
          { "name": "Barely Legal", "taxonomy": "video_channels" }
        ],
        [
          { "name": "Hustler Video", "taxonomy": "video_studio" }
        ],
        [
          { "name": "Some Director", "taxonomy": "video_director" }
        ]
      ]
    }
  },
  {
    "id": 5678,
    "date": "2025-12-15T00:00:00",
    "link": "https://hustlerunlimited.com/videos/second/",
    "title": { "rendered": "Second Scene" },
    "_embedded": { "wp:term": [] }
  }
]`

func TestToScene_mapsTaxonomies(t *testing.T) {
	var vids []video
	if err := json.Unmarshal([]byte(videosJSON), &vids); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	if len(vids) != 2 {
		t.Fatalf("got %d videos, want 2", len(vids))
	}

	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	scene := toScene("https://hustlerunlimited.com", vids[0], now)

	if scene.ID != "1234" {
		t.Errorf("ID = %q, want 1234", scene.ID)
	}
	if scene.SiteID != "hustler" {
		t.Errorf("SiteID = %q", scene.SiteID)
	}
	if scene.Title != "Worshipping & Princess" {
		t.Errorf("Title = %q (want HTML-unescaped)", scene.Title)
	}
	if scene.URL != "https://hustlerunlimited.com/videos/worshipping-princess/" {
		t.Errorf("URL = %q", scene.URL)
	}
	wantDate := time.Date(2026, 1, 23, 8, 30, 0, 0, time.UTC)
	if !scene.Date.Equal(wantDate) {
		t.Errorf("Date = %v, want %v", scene.Date, wantDate)
	}
	if len(scene.Performers) != 2 || scene.Performers[0] != "Dee Williams" || scene.Performers[1] != "Natasha Ty" {
		t.Errorf("Performers = %v", scene.Performers)
	}
	if len(scene.Tags) != 2 || scene.Tags[0] != "Anal" {
		t.Errorf("Tags = %v", scene.Tags)
	}
	if scene.Series != "Barely Legal" {
		t.Errorf("Series = %q, want Barely Legal", scene.Series)
	}
	if len(scene.Categories) != 1 || scene.Categories[0] != "Barely Legal" {
		t.Errorf("Categories = %v", scene.Categories)
	}
	if scene.Studio != "Hustler Video" {
		t.Errorf("Studio = %q, want Hustler Video", scene.Studio)
	}
	if scene.Director != "Some Director" {
		t.Errorf("Director = %q", scene.Director)
	}
	if !scene.ScrapedAt.Equal(now) {
		t.Errorf("ScrapedAt = %v, want %v", scene.ScrapedAt, now)
	}
}

func TestToScene_defaultStudioWhenNoTaxonomy(t *testing.T) {
	var vids []video
	if err := json.Unmarshal([]byte(videosJSON), &vids); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	scene := toScene("https://hustlerunlimited.com", vids[1], time.Now())
	if scene.Studio != "Hustler" {
		t.Errorf("Studio = %q, want default Hustler", scene.Studio)
	}
	if len(scene.Performers) != 0 {
		t.Errorf("Performers = %v, want none", scene.Performers)
	}
}

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url   string
		match bool
	}{
		{"https://hustlerunlimited.com/", true},
		{"https://www.hustlerunlimited.com/videos/foo/", true},
		{"http://hustlerunlimited.com", true},
		{"https://example.com/", false},
		{"", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.match {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.match)
		}
	}
}

func TestListScenes_endToEnd(t *testing.T) {
	orig := siteBase
	defer func() { siteBase = orig }()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/wp-json/wp/v2/videos":
			w.Header().Set("X-WP-Total", "2")
			if r.URL.Query().Get("page") == "1" {
				_, _ = fmt.Fprint(w, videosJSON)
				return
			}
			_, _ = fmt.Fprint(w, "[]")
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()
	siteBase = ts.URL

	s := New()
	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{})
	if err != nil {
		t.Fatalf("ListScenes: %v", err)
	}

	got := map[string]string{}
	var total int
	for r := range ch {
		switch r.Kind {
		case scraper.KindScene:
			got[r.Scene.ID] = r.Scene.Title
		case scraper.KindTotal:
			total = r.Total
		case scraper.KindError:
			t.Errorf("unexpected error: %v", r.Err)
		}
	}
	if len(got) != 2 {
		t.Fatalf("got %d scenes, want 2: %v", len(got), got)
	}
	if got["1234"] != "Worshipping & Princess" {
		t.Errorf("scene 1234 title = %q", got["1234"])
	}
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
}
