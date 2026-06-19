package kinkacademy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/Wasylq/FSS/scraper"
)

func newTestScraper(t *testing.T) (*Scraper, *httptest.Server) {
	t.Helper()
	fixture, err := os.ReadFile("testdata/posts_page1.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/wp-json/wp/v2/posts" {
			http.NotFound(w, r)
			return
		}
		page := r.URL.Query().Get("page")
		w.Header().Set("Content-Type", "application/json")
		// Single page of results; the fixture holds 3 posts.
		w.Header().Set("X-WP-Total", "3")
		w.Header().Set("X-WP-TotalPages", "1")
		if page != "" && page != "1" {
			_, _ = w.Write([]byte("[]"))
			return
		}
		_, _ = w.Write(fixture)
	}))
	t.Cleanup(ts.Close)

	s := New()
	s.base = ts.URL
	s.client = ts.Client()
	return s, ts
}

func collect(t *testing.T, s *Scraper) []scraper.SceneResult {
	t.Helper()
	ch, err := s.ListScenes(context.Background(), "https://www.kinkacademy.com/", scraper.ListOpts{})
	if err != nil {
		t.Fatalf("ListScenes: %v", err)
	}
	var results []scraper.SceneResult
	for r := range ch {
		results = append(results, r)
	}
	return results
}

func TestParseScenes(t *testing.T) {
	s, _ := newTestScraper(t)
	results := collect(t, s)

	var scenes []scraper.SceneResult
	var total int
	for _, r := range results {
		switch r.Kind {
		case scraper.KindScene:
			scenes = append(scenes, r)
		case scraper.KindTotal:
			total = r.Total
		case scraper.KindError:
			t.Fatalf("unexpected error result: %v", r.Err)
		}
	}

	if total != 3 {
		t.Errorf("total = %d, want 3", total)
	}
	if len(scenes) != 3 {
		t.Fatalf("got %d scenes, want 3", len(scenes))
	}

	first := scenes[0].Scene
	if first.ID != "19152" {
		t.Errorf("ID = %q, want 19152", first.ID)
	}
	if first.SiteID != "kinkacademy" {
		t.Errorf("SiteID = %q, want kinkacademy", first.SiteID)
	}
	if first.Studio != "Kink Academy" {
		t.Errorf("Studio = %q, want Kink Academy", first.Studio)
	}
	if first.Title != "Control Through Protocol: What It Is and Why It’s Sexy" {
		t.Errorf("Title = %q (HTML entities should be decoded)", first.Title)
	}
	if first.URL == "" {
		t.Error("URL is empty")
	}
	if first.Date.IsZero() {
		t.Error("Date is zero")
	}
	if y := first.Date.Year(); y != 2022 {
		t.Errorf("Date year = %d, want 2022", y)
	}
	if first.Description == "" {
		t.Error("Description is empty")
	}
	if first.Thumbnail == "" {
		t.Error("Thumbnail is empty")
	}
	if first.ScrapedAt.IsZero() {
		t.Error("ScrapedAt is zero")
	}

	// Instructor/expert from embedded author.
	if !contains(first.Performers, "Gray Dancer") {
		t.Errorf("Performers = %v, want to contain Gray Dancer", first.Performers)
	}

	// Topics from post_tag terms.
	for _, want := range []string{"D/s", "dominance", "submission"} {
		if !contains(first.Tags, want) {
			t.Errorf("Tags = %v, want to contain %q", first.Tags, want)
		}
	}
	// The "Video" category and email-style author term must not leak in as tags.
	if contains(first.Tags, "Video") {
		t.Errorf("Tags should not contain category Video: %v", first.Tags)
	}

	if first.Series != "Control Through Protocol" {
		t.Errorf("Series = %q, want Control Through Protocol", first.Series)
	}
}

func TestMatchesURL(t *testing.T) {
	s := New()
	match := []string{
		"https://www.kinkacademy.com/",
		"https://kinkacademy.com/topics/video/",
		"http://www.kinkacademy.com/topics/video/page/3/",
	}
	for _, u := range match {
		if !s.MatchesURL(u) {
			t.Errorf("MatchesURL(%q) = false, want true", u)
		}
	}
	noMatch := []string{
		"https://www.example.com/",
		"https://kinkuniversity.com/",
	}
	for _, u := range noMatch {
		if s.MatchesURL(u) {
			t.Errorf("MatchesURL(%q) = true, want false", u)
		}
	}
}

func TestKnownIDsEarlyStop(t *testing.T) {
	s, _ := newTestScraper(t)
	ch, err := s.ListScenes(context.Background(), "https://www.kinkacademy.com/", scraper.ListOpts{
		KnownIDs: map[string]bool{"19152": true},
	})
	if err != nil {
		t.Fatalf("ListScenes: %v", err)
	}
	var stopped bool
	for r := range ch {
		if r.Kind == scraper.KindStoppedEarly {
			stopped = true
		}
	}
	if !stopped {
		t.Error("expected StoppedEarly when first ID is known")
	}
}

func contains(haystack []string, needle string) bool {
	for _, h := range haystack {
		if h == needle {
			return true
		}
	}
	return false
}
