package indiebucks

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wasylq/FSS/scraper"
)

const listingHTML = `<html><body>
<div class="one-video">
  <a href="https://example.com/videos/test0001_jane">
    <img src="//cdn.example.com/test0001_jane-c900x598.jpg" class="img-responsive" />
    <span class="description-wrapper">
      <span class="description-inner-media">
        <p class="media">Jane Solo Scene</p>
      </span>
  </a>
</div>
<script type="application/ld+json">
  {
    "@context": "http://schema.org",
    "@type": "Movie",
    "name": "Jane Solo Scene",
    "description": "Jane does a great solo performance.",
    "image": "http://cdn.example.com/test0001_jane-c900x598.jpg",
    "datePublished": "2026-05-15 00:00:00",
    "actors": [
      {
        "@type": "Person",
        "name": "Jane Doe",
        "url": "https://example.com/models/jane-doe_1234"
      }
    ],
    "url": "https://example.com/videos/test0001_jane"
  }
</script>
<div class="one-video">
  <a href="https://example.com/videos/test0002_alice_bob">
    <img src="//cdn.example.com/test0002-c900x598.jpg" class="img-responsive" />
    <span class="description-wrapper">
      <span class="description-inner-media">
        <p class="media">Alice and Bob</p>
      </span>
  </a>
</div>
<script type="application/ld+json">
  {
    "@context": "http://schema.org",
    "@type": "Movie",
    "name": "Alice and Bob",
    "description": "A duo scene.",
    "image": "//cdn.example.com/test0002-c900x598.jpg",
    "datePublished": "2026-05-10 00:00:00",
    "actors": [
      {"@type": "Person", "name": "Alice", "url": "https://example.com/models/alice_5678"},
      {"@type": "Person", "name": "Bob", "url": "https://example.com/models/bob_9012"}
    ],
    "url": "https://example.com/videos/test0002_alice_bob"
  }
</script>
<div class="pagination">
  <a href="#" class="one-page-link">First</a>
  <a href="#" class="one-page-link active">1</a>
  <a href="https://example.com/videos?page=2" class="one-page-link">2</a>
  <a href="https://example.com/videos?page=1" class="one-page-link">Next</a>
  <a href="https://example.com/videos?page=1" class="one-page-link">Last</a>
</div>
</body></html>`

func newTestServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, listingHTML)
	}))
}

func TestFetchListing(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	s := &siteScraper{cfg: sites[0], client: ts.Client()}

	movies, lastPage, err := s.fetchListing(context.Background(), ts.URL+"/videos?page=1&sort=newest")
	if err != nil {
		t.Fatalf("fetchListing: %v", err)
	}
	if lastPage != 1 {
		t.Errorf("lastPage = %d, want 1", lastPage)
	}
	if len(movies) != 2 {
		t.Fatalf("got %d movies, want 2", len(movies))
	}

	if movies[0].Name != "Jane Solo Scene" {
		t.Errorf("movies[0].Name = %q", movies[0].Name)
	}
	if movies[0].DatePublished != "2026-05-15 00:00:00" {
		t.Errorf("movies[0].DatePublished = %q", movies[0].DatePublished)
	}
	if len(movies[0].Actors) != 1 || movies[0].Actors[0].Name != "Jane Doe" {
		t.Errorf("movies[0].Actors = %v", movies[0].Actors)
	}
	if movies[1].Name != "Alice and Bob" {
		t.Errorf("movies[1].Name = %q", movies[1].Name)
	}
	if len(movies[1].Actors) != 2 {
		t.Errorf("movies[1] has %d actors, want 2", len(movies[1].Actors))
	}
}

func TestToScene(t *testing.T) {
	s := &siteScraper{cfg: siteConfig{id: "testsite", studio: "Test Studio"}}
	m := movieLD{
		Name:          "Jane Solo Scene",
		Description:   "A great scene.",
		Image:         "//cdn.example.com/thumb.jpg",
		DatePublished: "2026-05-15 00:00:00",
		URL:           "https://example.com/videos/test0001_jane",
		Actors:        []actorLD{{Name: "Jane Doe"}},
	}
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	scene := s.toScene(m, "https://example.com", now)

	if scene.ID != "test0001_jane" {
		t.Errorf("ID = %q", scene.ID)
	}
	if scene.Title != "Jane Solo Scene" {
		t.Errorf("Title = %q", scene.Title)
	}
	wantDate := time.Date(2026, 5, 15, 0, 0, 0, 0, time.UTC)
	if !scene.Date.Equal(wantDate) {
		t.Errorf("Date = %v, want %v", scene.Date, wantDate)
	}
	if scene.Thumbnail != "https://cdn.example.com/thumb.jpg" {
		t.Errorf("Thumbnail = %q", scene.Thumbnail)
	}
	if len(scene.Performers) != 1 || scene.Performers[0] != "Jane Doe" {
		t.Errorf("Performers = %v", scene.Performers)
	}
}

func TestListScenes(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	s := &siteScraper{cfg: sites[0], client: ts.Client()}

	ch, err := s.ListScenes(context.Background(), ts.URL+"/videos", scraper.ListOpts{})
	if err != nil {
		t.Fatalf("ListScenes: %v", err)
	}

	var sceneCount int
	for r := range ch {
		switch r.Kind {
		case scraper.KindScene:
			sceneCount++
		case scraper.KindError:
			t.Errorf("unexpected error: %v", r.Err)
		}
	}
	if sceneCount != 2 {
		t.Errorf("got %d scenes, want 2", sceneCount)
	}
}

func TestMatchesURL(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"https://boys-smoking.com/videos", "boyssmoking"},
		{"https://www.boys-pissing.com/", "boyspissing"},
		{"https://boundmusclejocks.com", "boundmusclejocks"},
	}
	for _, tt := range tests {
		found := false
		for _, cfg := range sites {
			s := newScraper(cfg)
			if s.MatchesURL(tt.url) {
				if s.ID() != tt.want {
					t.Errorf("MatchesURL(%q) matched %q, want %q", tt.url, s.ID(), tt.want)
				}
				found = true
				break
			}
		}
		if !found {
			t.Errorf("no scraper matched %q", tt.url)
		}
	}
}
