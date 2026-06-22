package finishesthejob

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

func mustRead(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return b
}

func TestMatchesURL(t *testing.T) {
	s := newFor("manojob")
	cases := map[string]bool{
		"https://www.manojob.com/":                  true,
		"https://manojob.com/updates/manojob/2":     true,
		"http://www.manojob.com/scene/manojob/best": true,
		"https://www.mrpov.com/":                    false,
		"https://www.finishesthejob.com/":           false,
		"https://example.com/manojob.com":           false,
	}
	for u, want := range cases {
		if got := s.MatchesURL(u); got != want {
			t.Errorf("MatchesURL(%q) = %v, want %v", u, got, want)
		}
	}
}

func TestParseListing(t *testing.T) {
	items := parseListing(mustRead(t, "listing_page1.html"))
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	first := items[0]
	if first.id != "manojob/best-roomate-ever" {
		t.Errorf("id = %q", first.id)
	}
	if first.brand != "manojob" {
		t.Errorf("brand = %q", first.brand)
	}
	if first.title != "Best Roomate Ever" {
		t.Errorf("title = %q", first.title)
	}
	if first.url != "/scene/manojob/best-roomate-ever" {
		t.Errorf("url = %q", first.url)
	}
	if first.thumbnail != "https://www.manojob.com/tour/scenes/manojob/mj0019_ashlyn_peaks/featured.jpg" {
		t.Errorf("thumbnail = %q", first.thumbnail)
	}
	if len(first.performers) != 1 || first.performers[0] != "Ashlyn Peaks" {
		t.Errorf("performers = %v", first.performers)
	}
	if len(items[1].performers) != 2 {
		t.Errorf("second item performers = %v", items[1].performers)
	}
}

func TestParseDetail(t *testing.T) {
	d := parseDetail(mustRead(t, "detail.html"))
	if d.title != "Best Roomate Ever" {
		t.Errorf("title = %q", d.title)
	}
	if !strings.HasPrefix(d.description, "You're a certified") {
		t.Errorf("description = %q", d.description)
	}
	if d.date.Format("2006-01-02") != "2026-05-20" {
		t.Errorf("date = %v", d.date)
	}
	wantTags := []string{"Big Tits", "Shaved", "Ebony"}
	if len(d.tags) != len(wantTags) {
		t.Fatalf("tags = %v, want %v", d.tags, wantTags)
	}
	for i, tg := range wantTags {
		if d.tags[i] != tg {
			t.Errorf("tags[%d] = %q, want %q", i, d.tags[i], tg)
		}
	}
	if len(d.performers) != 1 || d.performers[0] != "Ashlyn Peaks" {
		t.Errorf("performers = %v", d.performers)
	}
	if d.thumbnail == "" {
		t.Errorf("thumbnail empty")
	}
}

func TestMaxPageNum(t *testing.T) {
	if n := maxPageNum(mustRead(t, "listing_page1.html")); n != 36 {
		t.Errorf("maxPageNum = %d, want 36", n)
	}
}

// newTestServer serves the fixtures from a fake Finishes The Job site.
func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/updates/manojob/1"):
			_, _ = fmt.Fprint(w, string(mustRead(t, "listing_page1.html")))
		case strings.HasPrefix(r.URL.Path, "/updates/manojob/2"):
			_, _ = fmt.Fprint(w, string(mustRead(t, "listing_page2.html")))
		case strings.HasPrefix(r.URL.Path, "/updates/manojob/"):
			_, _ = fmt.Fprint(w, string(mustRead(t, "listing_empty.html")))
		case strings.HasPrefix(r.URL.Path, "/scene/"):
			_, _ = fmt.Fprint(w, string(mustRead(t, "detail.html")))
		default:
			http.NotFound(w, r)
		}
	}))
}

func collect(t *testing.T, s *Scraper, opts scraper.ListOpts) []scraper.SceneResult {
	t.Helper()
	ch, err := s.ListScenes(context.Background(), s.base, opts)
	if err != nil {
		t.Fatalf("ListScenes: %v", err)
	}
	var got []scraper.SceneResult
	for r := range ch {
		got = append(got, r)
	}
	return got
}

func TestRunFullListing(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	s := newFor("manojob")
	s.Client = ts.Client()
	s.base = ts.URL

	results := collect(t, s, scraper.ListOpts{})

	var scenes int
	for _, r := range results {
		if r.Kind == scraper.KindScene {
			scenes++
			sc := r.Scene
			if sc.SiteID != "manojob" {
				t.Errorf("SiteID = %q", sc.SiteID)
			}
			if sc.Studio != "Mano Job" {
				t.Errorf("Studio = %q", sc.Studio)
			}
			if sc.Title == "" {
				t.Errorf("empty title for %s", sc.ID)
			}
			if !strings.HasPrefix(sc.URL, ts.URL+"/scene/manojob/") {
				t.Errorf("URL = %q", sc.URL)
			}
			if sc.Date.IsZero() {
				t.Errorf("zero date for %s", sc.ID)
			}
		}
	}
	// 2 from page1 + 1 from page2 = 3 scenes.
	if scenes != 3 {
		t.Errorf("got %d scenes, want 3", scenes)
	}
}

func TestRunKnownIDsEarlyStop(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	s := newFor("manojob")
	s.Client = ts.Client()
	s.base = ts.URL

	// Mark the second page-1 scene as known: pagination should stop after page 1.
	opts := scraper.ListOpts{KnownIDs: map[string]bool{
		"manojob/another-trip-to-the-sperm-bank": true,
	}}
	results := collect(t, s, opts)

	var emitted, stopped int
	for _, r := range results {
		switch r.Kind {
		case scraper.KindScene:
			emitted++
			if r.Scene.ID == "manojob/another-trip-to-the-sperm-bank" {
				t.Errorf("emitted a known scene")
			}
		case scraper.KindStoppedEarly:
			stopped++
		}
	}
	if stopped == 0 {
		t.Errorf("expected an early-stop signal")
	}
	if emitted != 1 {
		t.Errorf("emitted %d scenes, want 1 (the one before the known id)", emitted)
	}
}
