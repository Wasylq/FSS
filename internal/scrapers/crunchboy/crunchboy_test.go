package crunchboy

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wasylq/FSS/scraper"
)

const testListingHTML = `<!DOCTYPE html>
<html>
<head><title>Videos</title></head>
<body>
<script type="application/ld+json">
{
  "@type": "ItemList",
  "itemListElement": [
    {
      "@type": "ListItem",
      "position": 1,
      "item": {
        "@type": "VideoObject",
        "url": "BASE/en/videos/detail/12345-test-scene-one",
        "name": "Test Scene One",
        "description": "First test scene description",
        "thumbnailUrl": "https://gcs.pornsitemanager.com/store/1/2/3/thumb1.jpg",
        "datePublished": "2025-03-15",
        "actor": [
          {"@type": "Person", "name": "Actor One"},
          {"@type": "Person", "name": "Actor Two"}
        ]
      }
    },
    {
      "@type": "ListItem",
      "position": 2,
      "item": {
        "@type": "VideoObject",
        "url": "BASE/en/videos/detail/12346-test-scene-two",
        "name": "Test Scene Two",
        "description": "Second test scene with HTML &amp; entities",
        "thumbnailUrl": "https://gcs.pornsitemanager.com/store/4/5/6/thumb2.jpg",
        "datePublished": "2025-03-14",
        "actor": [
          {"@type": "Person", "name": "Actor Three"}
        ]
      }
    }
  ]
}
</script>
<div>
<a href="/en/videos/detail/12345-test-scene-one">
  <span class="h90"><i class="fal fa-clock"></i></span>
  <span>22 min</span>
  <span class="h90 ms-1"><i class="far fa-video"></i></span>
  <a href="/en/videos/crunchboy"><span class="h95 text-uppercase">Crunchboy</span></a>
</a>
<a href="/en/videos/detail/12346-test-scene-two">
  <span class="h90"><i class="fal fa-clock"></i></span>
  <span>15 min</span>
  <span class="h90 ms-1"><i class="far fa-video"></i></span>
  <a href="/en/videos/rafaboys"><span class="h95 text-uppercase">Rafaboys</span></a>
</a>
</div>
<a href="?page=1">1</a><a href="?page=2">2</a>
</body>
</html>`

func TestScraperInterface(t *testing.T) {
	var _ scraper.StudioScraper = &Scraper{}
}

func TestMatchesURL(t *testing.T) {
	s := New()
	tests := []struct {
		url  string
		want bool
	}{
		{"https://www.crunchboy.com", true},
		{"https://crunchboy.com/en/videos", true},
		{"https://www.crunchboy.com/en/videos/crunchboy", true},
		{"http://crunchboy.com/en/videos?page=2", true},
		{"https://www.example.com", false},
	}
	for _, tt := range tests {
		if got := s.MatchesURL(tt.url); got != tt.want {
			t.Errorf("MatchesURL(%q) = %v, want %v", tt.url, got, tt.want)
		}
	}
}

func TestParseListing(t *testing.T) {
	html := strings.ReplaceAll(testListingHTML, "BASE", "https://www.crunchboy.com")
	items, totalPages := parseListing([]byte(html))

	if totalPages != 2 {
		t.Errorf("totalPages = %d, want 2", totalPages)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}

	first := items[0]
	if first.id != "12345" {
		t.Errorf("id = %q, want %q", first.id, "12345")
	}
	if first.title != "Test Scene One" {
		t.Errorf("title = %q, want %q", first.title, "Test Scene One")
	}
	if first.description != "First test scene description" {
		t.Errorf("description = %q", first.description)
	}
	if first.date.Format("2006-01-02") != "2025-03-15" {
		t.Errorf("date = %v", first.date)
	}
	if len(first.performers) != 2 || first.performers[0] != "Actor One" {
		t.Errorf("performers = %v", first.performers)
	}
	if first.duration != 22*60 {
		t.Errorf("duration = %d, want %d", first.duration, 22*60)
	}
	if first.studio != "Crunchboy" {
		t.Errorf("studio = %q, want %q", first.studio, "Crunchboy")
	}

	second := items[1]
	if second.id != "12346" {
		t.Errorf("second id = %q", second.id)
	}
	if second.description != "Second test scene with HTML & entities" {
		t.Errorf("second description = %q", second.description)
	}
	if second.studio != "Rafaboys" {
		t.Errorf("second studio = %q, want %q", second.studio, "Rafaboys")
	}
}

func TestListScenes(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("page") {
		case "", "1":
			body := strings.ReplaceAll(testListingHTML, "BASE", "https://www.crunchboy.com")
			body = strings.ReplaceAll(body, `href="?page=2"`, "")
			_, _ = fmt.Fprint(w, body)
		default:
			_, _ = fmt.Fprint(w, `<html><script type="application/ld+json">{"@type":"ItemList","itemListElement":[]}</script></html>`)
		}
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client(), base: ts.URL}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/en/videos", scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}

	var scenes []scraper.SceneResult
	for r := range ch {
		scenes = append(scenes, r)
	}

	sceneCount := 0
	for _, r := range scenes {
		if r.Kind == scraper.KindScene {
			sceneCount++
		}
	}
	if sceneCount != 2 {
		t.Errorf("got %d scenes, want 2", sceneCount)
	}
}

func TestKnownIDsStopEarly(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := strings.ReplaceAll(testListingHTML, "BASE", "https://www.crunchboy.com")
		_, _ = fmt.Fprint(w, body)
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client(), base: ts.URL}
	known := map[string]bool{"12346": true}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/en/videos", scraper.ListOpts{KnownIDs: known})
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

func TestListingPath(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"https://www.crunchboy.com", "/en/videos"},
		{"https://www.crunchboy.com/en/videos", "/en/videos"},
		{"https://www.crunchboy.com/en/videos/crunchboy", "/en/videos/crunchboy"},
		{"https://www.crunchboy.com/en/videos/rafaboys", "/en/videos/rafaboys"},
	}
	for _, tt := range tests {
		if got := listingPath(tt.url); got != tt.want {
			t.Errorf("listingPath(%q) = %q, want %q", tt.url, got, tt.want)
		}
	}
}
