package pinupfiles

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Wasylq/FSS/scraper"
)

func loadFixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return string(b)
}

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	listing := loadFixture(t, "listing.html")
	detail := loadFixture(t, "detail.html")

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/categories/movies/1/"):
			_, _ = fmt.Fprint(w, listing)
		case strings.HasPrefix(r.URL.Path, "/categories/movies/"):
			// Any page beyond 1 has no cards -> stops pagination.
			_, _ = fmt.Fprint(w, "<html><body></body></html>")
		case strings.HasPrefix(r.URL.Path, "/trailers/"):
			_, _ = fmt.Fprint(w, detail)
		case strings.HasPrefix(r.URL.Path, "/models/"):
			_, _ = fmt.Fprint(w, listing)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(ts.Close)
	return ts
}

func collect(t *testing.T, s *Scraper, url string, opts scraper.ListOpts) []scraper.SceneResult {
	t.Helper()
	ch, err := s.ListScenes(context.Background(), url, opts)
	if err != nil {
		t.Fatalf("ListScenes: %v", err)
	}
	var results []scraper.SceneResult
	for r := range ch {
		results = append(results, r)
	}
	return results
}

func TestParseListingFiltersPhotos(t *testing.T) {
	body := []byte(loadFixture(t, "listing.html"))
	items := parseListingPage(body)
	if len(items) != 2 {
		t.Fatalf("expected 2 video items (photos excluded), got %d", len(items))
	}
	for _, it := range items {
		if strings.Contains(it.url, "/scenes/") {
			t.Errorf("photo entry leaked into results: %q", it.url)
		}
		if !strings.HasPrefix(it.url, "/trailers/") {
			t.Errorf("unexpected url %q", it.url)
		}
	}

	first := items[0]
	if first.id != "staceypoole-blackvinyl02-4K" {
		t.Errorf("id = %q", first.id)
	}
	if first.title != "Stacey Poole - Black Vinyl 2 - 4K" {
		t.Errorf("title = %q", first.title)
	}
	if first.duration != 13*60+52 {
		t.Errorf("duration = %d, want %d", first.duration, 13*60+52)
	}
	if first.likes != 5 {
		t.Errorf("likes = %d, want 5", first.likes)
	}
	if !strings.Contains(first.thumbnail, "mjedge.net") {
		t.Errorf("thumbnail = %q", first.thumbnail)
	}
	if got := first.date.Format("2006-01-02"); got != "2026-06-17" {
		t.Errorf("date = %q, want 2026-06-17", got)
	}
	if len(first.performers) != 1 || first.performers[0] != "Stacey Poole" {
		t.Errorf("performers = %v", first.performers)
	}
}

func TestParseDetailPage(t *testing.T) {
	d := parseDetailPage([]byte(loadFixture(t, "detail.html")))
	if d.title != "Stacey Poole - Black Vinyl 2 - 4K" {
		t.Errorf("title = %q", d.title)
	}
	if d.duration != 13*60+52 {
		t.Errorf("duration = %d", d.duration)
	}
	if !d.hasDate || d.date.Format("2006-01-02") != "2026-06-17" {
		t.Errorf("date hasDate=%v date=%v", d.hasDate, d.date)
	}
	if !strings.Contains(d.description, "All-natural glam goddess") {
		t.Errorf("description = %q", d.description)
	}
	if strings.Contains(d.description, "<") {
		t.Errorf("description still contains HTML: %q", d.description)
	}
	wantTags := []string{"Bra", "Brunettes", "Busty", "strip tease"}
	if len(d.tags) != len(wantTags) {
		t.Fatalf("tags = %v, want %v", d.tags, wantTags)
	}
	for i, tag := range wantTags {
		if d.tags[i] != tag {
			t.Errorf("tag[%d] = %q, want %q", i, d.tags[i], tag)
		}
	}
}

func TestRunPaginatedEndToEnd(t *testing.T) {
	ts := newTestServer(t)
	s := New()
	s.base = ts.URL
	s.Client = ts.Client()

	results := collect(t, s, ts.URL+"/", scraper.ListOpts{})

	var scenes []scraper.SceneResult
	for _, r := range results {
		if r.Kind == scraper.KindScene {
			scenes = append(scenes, r)
		}
		if r.Kind == scraper.KindError {
			t.Fatalf("unexpected error result: %v", r.Err)
		}
	}
	if len(scenes) != 2 {
		t.Fatalf("expected 2 scenes, got %d", len(scenes))
	}

	sc := scenes[0].Scene
	if sc.SiteID != "pinupfiles" {
		t.Errorf("SiteID = %q", sc.SiteID)
	}
	if sc.Studio != "Pinup Files" {
		t.Errorf("Studio = %q", sc.Studio)
	}
	if sc.Title != "Stacey Poole - Black Vinyl 2 - 4K" {
		t.Errorf("Title = %q", sc.Title)
	}
	if !strings.HasPrefix(sc.URL, ts.URL+"/trailers/") {
		t.Errorf("URL = %q", sc.URL)
	}
	if sc.Description == "" || sc.Duration == 0 || len(sc.Tags) == 0 {
		t.Errorf("detail enrichment missing: desc=%q dur=%d tags=%v", sc.Description, sc.Duration, sc.Tags)
	}
	if sc.ScrapedAt.IsZero() {
		t.Error("ScrapedAt not set")
	}
}

func TestKnownIDsEarlyStop(t *testing.T) {
	ts := newTestServer(t)
	s := New()
	s.base = ts.URL
	s.Client = ts.Client()

	opts := scraper.ListOpts{KnownIDs: map[string]bool{"staceypoole-blackvinyl02-4K": true}}
	results := collect(t, s, ts.URL+"/", opts)

	stopped := false
	for _, r := range results {
		if r.Kind == scraper.KindStoppedEarly {
			stopped = true
		}
	}
	if !stopped {
		t.Error("expected StoppedEarly when first scene is a known ID")
	}
}

func TestModelPageMode(t *testing.T) {
	ts := newTestServer(t)
	s := New()
	s.base = ts.URL
	s.Client = ts.Client()

	results := collect(t, s, ts.URL+"/models/stacey-poole.html", scraper.ListOpts{})
	scenes := 0
	for _, r := range results {
		if r.Kind == scraper.KindScene {
			scenes++
		}
	}
	if scenes != 2 {
		t.Errorf("model page: expected 2 video scenes, got %d", scenes)
	}
}

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := map[string]bool{
		"https://www.pinupfiles.com/":                         true,
		"https://pinupfiles.com/categories/movies/1/latest/":  true,
		"https://www.pinupfiles.com/models/stacey-poole.html": true,
		"https://example.com/":                                false,
	}
	for u, want := range cases {
		if got := s.MatchesURL(u); got != want {
			t.Errorf("MatchesURL(%q) = %v, want %v", u, got, want)
		}
	}
}
