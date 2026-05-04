package lucasentertainment

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wasylq/FSS/scraper"
)

const testAPIResponse = `[
  {
    "id": 92943,
    "date_gmt": "2026-05-01T07:10:14",
    "slug": "pablo-pixx-tops-mik-ayden",
    "link": "https://www.lucasentertainment.com/pablo-pixx-tops-mik-ayden/",
    "title": {"rendered": "Pablo Pixx Tops Mik Ayden"},
    "content": {"rendered": "<p>Test description for the scene.</p>"},
    "featured_media": 0,
    "yoast_head": "<meta property=\"og:image\" content=\"https://example.com/thumb.jpg\" />",
    "_embedded": {
      "wp:featuredmedia": [],
      "wp:term": [
        [{"name": "Scenes", "taxonomy": "category"}],
        [{"name": "LVP495-01", "taxonomy": "post_tag"}]
      ]
    }
  },
  {
    "id": 92940,
    "date_gmt": "2026-04-27T07:40:02",
    "slug": "igor-lucios-sucks-and-rides-derek-kage",
    "link": "https://www.lucasentertainment.com/igor-lucios-sucks-and-rides-derek-kage/",
    "title": {"rendered": "Igor Lucios Sucks And Rides Derek Kage"},
    "content": {"rendered": "<p class=\"p1\">Igor Lucios arrived from Brazil.</p>"},
    "featured_media": 92229,
    "yoast_head": "",
    "_embedded": {
      "wp:featuredmedia": [{"source_url": "https://example.com/igor.jpg"}],
      "wp:term": [
        [{"name": "Scenes", "taxonomy": "category"}],
        [{"name": "LVP494-04", "taxonomy": "post_tag"}]
      ]
    }
  }
]`

func TestScraperInterface(t *testing.T) {
	var _ scraper.StudioScraper = &Scraper{}
}

func TestMatchesURL(t *testing.T) {
	s := New()
	tests := []struct {
		url  string
		want bool
	}{
		{"https://www.lucasentertainment.com", true},
		{"https://lucasentertainment.com/scenes/", true},
		{"http://www.lucasentertainment.com/pablo-pixx-tops-mik-ayden/", true},
		{"https://www.example.com", false},
	}
	for _, tt := range tests {
		if got := s.MatchesURL(tt.url); got != tt.want {
			t.Errorf("MatchesURL(%q) = %v, want %v", tt.url, got, tt.want)
		}
	}
}

func TestExtractPerformers(t *testing.T) {
	tests := []struct {
		title string
		want  []string
	}{
		{"Pablo Pixx Tops Mik Ayden", []string{"Pablo Pixx", "Mik Ayden"}},
		{"Igor Lucios Sucks And Rides Derek Kage", []string{"Igor Lucios", "Derek Kage"}},
		{"Brian Bonds Rides Harold Lopez", []string{"Brian Bonds", "Harold Lopez"}},
		{"Omikink Has Andrea Suarez Service His 9-Inch Uncut Cock", []string{"Omikink", "Andrea Suarez"}},
		{"Apolo Adrii Pounds Alfonso Osnaya", []string{"Apolo Adrii", "Alfonso Osnaya"}},
	}
	for _, tt := range tests {
		got := extractPerformers(tt.title)
		if len(got) != len(tt.want) {
			t.Errorf("extractPerformers(%q) = %v, want %v", tt.title, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("extractPerformers(%q)[%d] = %q, want %q", tt.title, i, got[i], tt.want[i])
			}
		}
	}
}

func TestListScenes(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-WP-Total", "2")
		w.Header().Set("X-WP-TotalPages", "1")
		_, _ = fmt.Fprint(w, testAPIResponse)
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client(), base: ts.URL}
	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}

	var scenes []scraper.SceneResult
	for r := range ch {
		scenes = append(scenes, r)
	}

	sceneCount := 0
	for _, r := range scenes {
		switch r.Kind {
		case scraper.KindScene:
			sceneCount++
		case scraper.KindError:
			t.Fatalf("unexpected error: %v", r.Err)
		}
	}
	if sceneCount != 2 {
		t.Errorf("got %d scenes, want 2", sceneCount)
	}
}

func TestToScene(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-WP-Total", "2")
		_, _ = fmt.Fprint(w, testAPIResponse)
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client(), base: ts.URL}
	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}

	var first, second *scraper.SceneResult
	for r := range ch {
		if r.Kind == scraper.KindScene {
			r := r
			if first == nil {
				first = &r
			} else {
				second = &r
			}
		}
	}

	if first == nil {
		t.Fatal("no scenes")
	}
	if first.Scene.Title != "Pablo Pixx Tops Mik Ayden" {
		t.Errorf("title = %q", first.Scene.Title)
	}
	if first.Scene.Description != "Test description for the scene." {
		t.Errorf("description = %q", first.Scene.Description)
	}
	if first.Scene.Date.Format("2006-01-02") != "2026-05-01" {
		t.Errorf("date = %v", first.Scene.Date)
	}
	if first.Scene.Thumbnail != "https://example.com/thumb.jpg" {
		t.Errorf("thumbnail = %q (expected OG fallback)", first.Scene.Thumbnail)
	}
	if len(first.Scene.Tags) != 1 || first.Scene.Tags[0] != "LVP495-01" {
		t.Errorf("tags = %v", first.Scene.Tags)
	}

	if second == nil {
		t.Fatal("expected second scene")
	}
	if second.Scene.Thumbnail != "https://example.com/igor.jpg" {
		t.Errorf("second thumbnail = %q (expected embedded media)", second.Scene.Thumbnail)
	}
}

func TestKnownIDsStopEarly(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-WP-Total", "2")
		_, _ = fmt.Fprint(w, testAPIResponse)
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client(), base: ts.URL}
	known := map[string]bool{"92940": true}
	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{KnownIDs: known})
	if err != nil {
		t.Fatal(err)
	}

	var gotScene, gotStop bool
	for r := range ch {
		switch r.Kind {
		case scraper.KindScene:
			gotScene = true
		case scraper.KindStoppedEarly:
			gotStop = true
		}
	}
	if !gotScene {
		t.Error("expected at least one scene before stop")
	}
	if !gotStop {
		t.Error("expected StoppedEarly")
	}
}
