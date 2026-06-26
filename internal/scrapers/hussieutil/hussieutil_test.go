package hussieutil

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

	"github.com/Wasylq/FSS/scraper"
)

// Fixture derived from povporncash NATS tour markup. Two listing cards, each
// carrying full per-scene metadata so no detail fetch is needed.
const listingHTML = `<html><body>
<div class="row">

<div class="item-video">
  <div class="img-div">
    <a href="/trailers/HussieSceneOne.html" title="Hussie Scene One">
      <img class="thumbs" src0_1x="/content//contentthumbs/65/62/16562-1x.jpg" />
    </a>
  </div>
  <div class="time">30:35</div>
  <div class="date">2026-06-23</div>
</div>

<div class="item-video">
  <div class="img-div">
    <a href="/trailers/HussieSceneTwo.html" title="Hussie Scene Two">
      <img class="thumbs" src0_1x="/content//contentthumbs/65/63/16563-1x.jpg" />
    </a>
  </div>
  <div class="time">25:10</div>
  <div class="date">2026-06-20</div>
</div>

</div>
</body></html>`

const emptyHTML = `<html><body><div class="row"></div></body></html>`

func testConfig(base, tourPrefix string) SiteConfig {
	return SiteConfig{
		ID:         "hussiepass",
		Studio:     "Hussie Pass",
		SiteBase:   base,
		TourPrefix: tourPrefix,
		MatchRe:    regexp.MustCompile(`^https?://(?:www\.)?hussiepass\.com`),
	}
}

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		switch r.URL.Path {
		case "/categories/movies/1/latest/", "/tour/categories/movies/1/latest/":
			_, _ = fmt.Fprint(w, listingHTML)
		default:
			_, _ = fmt.Fprint(w, emptyHTML)
		}
	}))
}

func drainScenes(t *testing.T, ch <-chan scraper.SceneResult) []scraper.SceneResult {
	t.Helper()
	var scenes []scraper.SceneResult
	for r := range ch {
		switch r.Kind {
		case scraper.KindScene:
			scenes = append(scenes, r)
		case scraper.KindError:
			t.Errorf("unexpected error: %v", r.Err)
		}
	}
	return scenes
}

func TestListScenes_endToEnd(t *testing.T) {
	for _, tourPrefix := range []string{"", "/tour"} {
		t.Run("tour="+tourPrefix, func(t *testing.T) {
			ts := newTestServer(t)
			defer ts.Close()

			s := New(testConfig(ts.URL, tourPrefix))
			ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{})
			if err != nil {
				t.Fatal(err)
			}
			scenes := drainScenes(t, ch)
			if len(scenes) != 2 {
				t.Fatalf("got %d scenes, want 2", len(scenes))
			}

			first := scenes[0].Scene
			if first.ID != "16562" {
				t.Errorf("ID = %q, want content id 16562", first.ID)
			}
			if first.Title != "Hussie Scene One" {
				t.Errorf("Title = %q", first.Title)
			}
			if first.URL != ts.URL+"/trailers/HussieSceneOne.html" {
				t.Errorf("URL = %q", first.URL)
			}
			if first.Thumbnail != ts.URL+"/content//contentthumbs/65/62/16562-1x.jpg" {
				t.Errorf("Thumbnail = %q", first.Thumbnail)
			}
			if first.Duration != 1835 {
				t.Errorf("Duration = %d, want 1835", first.Duration)
			}
			if first.Date.Year() != 2026 || first.Date.Month() != 6 || first.Date.Day() != 23 {
				t.Errorf("Date = %v, want 2026-06-23", first.Date)
			}
			if first.Studio != "Hussie Pass" {
				t.Errorf("Studio = %q", first.Studio)
			}

			second := scenes[1].Scene
			if second.ID != "16563" {
				t.Errorf("Second ID = %q", second.ID)
			}
			if second.Duration != 1510 {
				t.Errorf("Second Duration = %d, want 1510", second.Duration)
			}
		})
	}
}

func TestToScene(t *testing.T) {
	card := `"> <a href="/trailers/HussieSceneOne.html" title="Hussie &amp; Friends">
		<img src0_1x="/content//contentthumbs/65/62/16562-1x.jpg" />
		<div class="time">30:35</div>
		<div class="date">2026-06-23</div>`
	s := New(testConfig("https://hussiepass.com", ""))
	now := time.Now().UTC()
	scene, ok := s.toScene("https://hussiepass.com", card, now)
	if !ok {
		t.Fatal("toScene returned ok=false")
	}
	if scene.ID != "16562" {
		t.Errorf("ID = %q", scene.ID)
	}
	if scene.Title != "Hussie & Friends" {
		t.Errorf("Title = %q (want HTML-unescaped)", scene.Title)
	}
	if scene.URL != "https://hussiepass.com/trailers/HussieSceneOne.html" {
		t.Errorf("URL = %q", scene.URL)
	}
	if scene.Thumbnail != "https://hussiepass.com/content//contentthumbs/65/62/16562-1x.jpg" {
		t.Errorf("Thumbnail = %q", scene.Thumbnail)
	}
	if scene.Duration != 1835 {
		t.Errorf("Duration = %d", scene.Duration)
	}
	if scene.Date.Year() != 2026 || scene.Date.Month() != 6 || scene.Date.Day() != 23 {
		t.Errorf("Date = %v", scene.Date)
	}
}

func TestToScene_noMatch(t *testing.T) {
	s := New(testConfig("https://hussiepass.com", ""))
	if _, ok := s.toScene("https://hussiepass.com", "<div>no trailer link here</div>", time.Now()); ok {
		t.Error("expected ok=false for card without trailer link")
	}
}

func TestMatchesURL(t *testing.T) {
	s := New(testConfig("https://hussiepass.com", ""))
	cases := []struct {
		url   string
		match bool
	}{
		{"https://hussiepass.com/", true},
		{"http://www.hussiepass.com/categories/movies/2/latest/", true},
		{"https://example.com/", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.match {
			t.Errorf("MatchesURL(%q) = %v", c.url, got)
		}
	}
}

func TestListScenes_knownIDsStopsEarly(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	s := New(testConfig(ts.URL, ""))
	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{
		KnownIDs: map[string]bool{"16563": true},
	})
	if err != nil {
		t.Fatal(err)
	}
	var (
		scenes       int
		stoppedEarly bool
	)
	for r := range ch {
		switch r.Kind {
		case scraper.KindScene:
			scenes++
		case scraper.KindStoppedEarly:
			stoppedEarly = true
		}
	}
	if scenes != 1 {
		t.Errorf("got %d scenes, want 1", scenes)
	}
	if !stoppedEarly {
		t.Error("expected StoppedEarly")
	}
}
