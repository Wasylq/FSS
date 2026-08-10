package auntjudys

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
	"github.com/Anastylosis/FSS/scraper"
)

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url  string
		want bool
	}{
		{"https://www.auntjudysxxx.com/tour/categories/movies.html", true},
		{"https://auntjudysxxx.com/tour/categories/movies.html", true},
		{"https://www.auntjudysxxx.com/tour/models/andi-james.html", true},
		{"https://www.auntjudysxxx.com/", true},
		{"https://www.auntjudys.com/tour/categories/movies.html", true},
		{"https://auntjudys.com/tour/categories/movies.html", true},
		{"https://www.auntjudys.com/", true},
		{"http://www.auntjudys.com/tour/models/andi-james.html", true},
		{"https://example.com/auntjudys", false},
		{"", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.want {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.want)
		}
	}
}

func TestResolveBase(t *testing.T) {
	cases := []struct {
		url  string
		want string
	}{
		{"https://www.auntjudysxxx.com/tour/categories/movies.html", "https://www.auntjudysxxx.com"},
		{"https://auntjudysxxx.com/", "https://auntjudysxxx.com"},
		{"https://www.auntjudys.com/tour/categories/movies.html", "https://www.auntjudys.com"},
		{"http://auntjudys.com/tour/models/andi.html", "http://auntjudys.com"},
		{"https://www.auntjudys.com", "https://www.auntjudys.com"},
	}
	for _, c := range cases {
		if got := resolveBase(c.url); got != c.want {
			t.Errorf("resolveBase(%q) = %q, want %q", c.url, got, c.want)
		}
	}
}

// listingHTML mirrors the 2026 tour: an update-grid of update-item cards, each
// repeated in a carousel, mixing videos with photo sets.
const listingHTML = `<html><body>
<div class="update-grid">
	<div class="update-item group">
		<div class="img-wrap">
			<img src="/tour/content//contentthumbs/86/75/18675-3x.jpg" alt="Alt Title One" />
			<span class="update-type"><span class="fas fa-video"></span> 20:41</span>
			<span class="update-date"><span class="fad fa-calendar"></span> 08/08/2026</span>
		</div>
		<div class="flex-1 bg-white">
			<a href="https://test.local/tour/join.php" class="update-title">
				<span class="absolute inset-0 z-10"></span>
				Curvy MILF Ameliya&#39;s Stepson			</a>
		</div>
		<div class="update-sub-details">
			<div><span class="update_models">
	Ameliya
	</span></div>
		</div>
	</div>
	<div class="update-item group">
		<div class="img-wrap">
			<img src="/tour/content//contentthumbs/86/71/18671-3x.jpg" alt="Alt Title Two" />
			<span class="update-type"><span class="fas fa-images"></span> 191 Photos</span>
			<span class="update-date"><span class="fad fa-calendar"></span> 08/07/2026</span>
		</div>
		<div class="flex-1 bg-white">
			<a href="https://test.local/tour/join.php" class="update-title">A Photo Set</a>
		</div>
	</div>
	<div class="update-item group">
		<div class="img-wrap">
			<img src="/tour/content//contentthumbs/86/72/18672-3x.jpg" alt="Alt Title Three" />
			<span class="update-type"><span class="fas fa-video"></span> 1:02:03</span>
			<span class="update-date"><span class="fad fa-calendar"></span> 08/06/2026</span>
		</div>
		<div class="flex-1 bg-white">
			<a href="https://test.local/tour/join.php" class="update-title">Two Models Scene</a>
		</div>
		<div class="update-sub-details">
			<div><span class="update_models">Taylor Vixxen, Other Model</span></div>
		</div>
	</div>
	<div class="update-item inner carousel-item">
		<div class="img-wrap">
			<img src="/tour/content//contentthumbs/86/75/18675-3x.jpg" alt="Alt Title One" />
			<span class="update-type"><span class="fas fa-video"></span> 20:41</span>
			<span class="update-date"><span class="fad fa-calendar"></span> 08/08/2026</span>
		</div>
	</div>
</div>
</body></html>`

func TestParseListingPage(t *testing.T) {
	now := time.Date(2026, 8, 9, 0, 0, 0, 0, time.UTC)
	scenes := parseListingPage([]byte(listingHTML), "https://test.local", "https://test.local"+listingPath, now)

	// The photo set is dropped and the carousel repeat is deduped.
	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2: %+v", len(scenes), scenes)
	}

	s := scenes[0]
	if s.ID != "18675" {
		t.Errorf("ID = %q, want 18675", s.ID)
	}
	if s.Title != "Curvy MILF Ameliya's Stepson" {
		t.Errorf("Title = %q", s.Title)
	}
	if s.Duration != 20*60+41 {
		t.Errorf("Duration = %d, want %d", s.Duration, 20*60+41)
	}
	if s.Thumbnail != "https://test.local/tour/content//contentthumbs/86/75/18675-3x.jpg" {
		t.Errorf("Thumbnail = %q", s.Thumbnail)
	}
	if want := time.Date(2026, 8, 8, 0, 0, 0, 0, time.UTC); !s.Date.Equal(want) {
		t.Errorf("Date = %v, want %v", s.Date, want)
	}
	if len(s.Performers) != 1 || s.Performers[0] != "Ameliya" {
		t.Errorf("Performers = %v", s.Performers)
	}
	if s.URL != "https://test.local"+listingPath {
		t.Errorf("URL = %q, want the listing page (no public scene page)", s.URL)
	}

	s2 := scenes[1]
	if s2.ID != "18672" {
		t.Errorf("ID = %q, want 18672", s2.ID)
	}
	if s2.Duration != 3723 {
		t.Errorf("Duration = %d, want 3723 (HH:MM:SS)", s2.Duration)
	}
	if len(s2.Performers) != 2 || s2.Performers[1] != "Other Model" {
		t.Errorf("Performers = %v", s2.Performers)
	}
}

func TestParseListingPageEmpty(t *testing.T) {
	got := parseListingPage([]byte("<html><body>nothing</body></html>"), "https://test.local", "https://test.local", time.Now())
	if len(got) != 0 {
		t.Errorf("got %d scenes, want 0", len(got))
	}
}

func newTestServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		if r.URL.Path == listingPath {
			_, _ = fmt.Fprint(w, body)
			return
		}
		http.NotFound(w, r)
	}))
}

func collect(t *testing.T, ch <-chan scraper.SceneResult) (scenes []string, errs int, stopped bool) {
	t.Helper()
	for r := range ch {
		switch r.Kind {
		case scraper.KindScene:
			scenes = append(scenes, r.Scene.ID)
		case scraper.KindError:
			errs++
		case scraper.KindStoppedEarly:
			stopped = true
		}
	}
	return
}

func TestListScenes(t *testing.T) {
	ts := newTestServer(t, listingHTML)
	defer ts.Close()

	s := New()
	s.client = ts.Client()

	ch, err := s.ListScenes(context.Background(), ts.URL+listingPath, scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}
	scenes, errs, _ := collect(t, ch)
	if errs != 0 {
		t.Errorf("got %d errors, want 0", errs)
	}
	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2: %v", len(scenes), scenes)
	}
}

func TestListScenesKnownIDs(t *testing.T) {
	ts := newTestServer(t, listingHTML)
	defer ts.Close()

	s := New()
	s.client = ts.Client()

	opts := scraper.ListOpts{KnownIDs: map[string]bool{"18675": true}}
	ch, err := s.ListScenes(context.Background(), ts.URL+listingPath, opts)
	if err != nil {
		t.Fatal(err)
	}
	scenes, _, stopped := collect(t, ch)
	if !stopped {
		t.Error("expected StoppedEarly")
	}
	if len(scenes) != 0 {
		t.Errorf("got %v, want no scenes (first is known)", scenes)
	}
}

// TestListScenesReportsEmptyListing pins that a tour that parses to nothing
// raises a parse error rather than reporting a clean empty scrape.
func TestListScenesReportsEmptyListing(t *testing.T) {
	ts := newTestServer(t, "<html><body>redesigned again</body></html>")
	defer ts.Close()

	s := New()
	s.client = ts.Client()

	ch, err := s.ListScenes(context.Background(), ts.URL+listingPath, scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}
	var errs int
	for r := range ch {
		if r.Kind == scraper.KindError {
			errs++
			if got := scraper.Classify(r.Err); got != scraper.FailureParse {
				t.Errorf("Classify = %v, want FailureParse", got)
			}
		}
	}
	if errs != 1 {
		t.Errorf("got %d errors, want 1", errs)
	}
}

func TestCancellable(t *testing.T) {
	ts := newTestServer(t, listingHTML)
	defer ts.Close()

	s := New()
	s.client = ts.Client()
	testutil.AssertCancellable(t, s, ts.URL+listingPath, scraper.ListOpts{})
}
