package premiumbukkake

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Wasylq/FSS/scraper"
)

func mustFixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return string(b)
}

// newTestScraper wires a Scraper to a server that serves the listing/detail
// fixtures under the /tour2/ paths the scraper requests.
func newTestScraper(t *testing.T) (*Scraper, *httptest.Server) {
	t.Helper()
	listing := mustFixture(t, "listing.html")
	listingLast := mustFixture(t, "listing_last.html")
	detail := mustFixture(t, "detail.html")

	mux := http.NewServeMux()
	mux.HandleFunc("/tour2/updates/page_1.html", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, listing)
	})
	mux.HandleFunc("/tour2/updates/page_2.html", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, listingLast)
	})
	// Any detail page returns the same fixture; the scraper keys off the slug.
	mux.HandleFunc("/tour2/updates/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, detail)
	})

	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	s := New()
	s.base = ts.URL
	s.client = ts.Client()
	return s, ts
}

func collect(t *testing.T, s *Scraper, opts scraper.ListOpts) []scraper.SceneResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ch, err := s.ListScenes(ctx, s.base+"/", opts)
	if err != nil {
		t.Fatalf("ListScenes: %v", err)
	}
	var out []scraper.SceneResult
	for r := range ch {
		out = append(out, r)
	}
	return out
}

func TestMatchesURL(t *testing.T) {
	s := New()
	good := []string{
		"https://premiumbukkake.com/",
		"https://premiumbukkake.com",
		"https://www.premiumbukkake.com/tour2/",
		"http://premiumbukkake.com/tour2/updates/Cintia-Lara-1-Bukkake.html",
	}
	for _, u := range good {
		if !s.MatchesURL(u) {
			t.Errorf("MatchesURL(%q) = false, want true", u)
		}
	}
	bad := []string{
		"https://example.com/",
		"https://otherbukkake.com/tour2/",
	}
	for _, u := range bad {
		if s.MatchesURL(u) {
			t.Errorf("MatchesURL(%q) = true, want false", u)
		}
	}
}

func TestListingSlugs(t *testing.T) {
	body := []byte(mustFixture(t, "listing.html"))
	slugs := listingSlugs(body)
	want := []string{"Dalila-Lapiedra-1-Bukkake", "Lana-Coxxx-1-Bukkake"}
	if len(slugs) != len(want) {
		t.Fatalf("got %d slugs %v, want %d", len(slugs), slugs, len(want))
	}
	for i, w := range want {
		if slugs[i] != w {
			t.Errorf("slug[%d] = %q, want %q", i, slugs[i], w)
		}
	}
}

func TestParseDetail(t *testing.T) {
	s := New()
	body := []byte(mustFixture(t, "detail.html"))
	now := time.Now().UTC()
	sc := s.parseDetail(body, "Dalila-Lapiedra-1-Bukkake", "https://premiumbukkake.com/tour2/updates/Dalila-Lapiedra-1-Bukkake.html", now)

	if sc.Title != "Dalila Lapiedra #1 - Bukkake" {
		t.Errorf("Title = %q", sc.Title)
	}
	if sc.SiteID != "premiumbukkake" {
		t.Errorf("SiteID = %q", sc.SiteID)
	}
	if sc.Studio != "Premium Bukkake" {
		t.Errorf("Studio = %q", sc.Studio)
	}
	if !strings.Contains(sc.Description, "Stunning Dalila Lapiedra") {
		t.Errorf("Description = %q", sc.Description)
	}
	if !strings.HasSuffix(sc.Thumbnail, "/tour2/content/002/PB_453_dalila_lapiedra_1_bukkake/0.jpg") {
		t.Errorf("Thumbnail = %q", sc.Thumbnail)
	}
	if sc.Preview != "https://premiumbukkake.com/trailers/PB_453_dalilalapiedra.mp4" {
		t.Errorf("Preview = %q", sc.Preview)
	}
	if len(sc.Performers) != 1 || sc.Performers[0] != "Dalila Lapiedra" {
		t.Errorf("Performers = %v", sc.Performers)
	}
	wantTags := []string{"amateur", "bukkake", "Main Video", "teen"}
	if strings.Join(sc.Tags, ",") != strings.Join(wantTags, ",") {
		t.Errorf("Tags = %v, want %v", sc.Tags, wantTags)
	}
	if sc.Date.Format("2006-01-02") != "2026-05-01" {
		t.Errorf("Date = %v", sc.Date)
	}
	// Related-video category/date below the main scene must not leak in.
	for _, tag := range sc.Tags {
		if tag == "" {
			t.Errorf("empty tag in %v", sc.Tags)
		}
	}
}

func TestRunEndToEnd(t *testing.T) {
	s, _ := newTestScraper(t)
	results := collect(t, s, scraper.ListOpts{})

	var scenes []scraper.SceneResult
	for _, r := range results {
		if r.Kind == scraper.KindScene {
			scenes = append(scenes, r)
		}
		if r.Kind == scraper.KindError {
			t.Fatalf("unexpected error result: %v", r.Err)
		}
	}
	// page_1 has 2 cards, page_2 (last) has 1 card => 3 scenes total.
	if len(scenes) != 3 {
		t.Fatalf("got %d scenes, want 3", len(scenes))
	}
	for _, r := range scenes {
		if r.Scene.Title == "" {
			t.Errorf("scene %s has empty title", r.Scene.ID)
		}
		if r.Scene.Studio != "Premium Bukkake" {
			t.Errorf("scene %s Studio = %q", r.Scene.ID, r.Scene.Studio)
		}
		if r.Scene.URL == "" {
			t.Errorf("scene %s has empty URL", r.Scene.ID)
		}
	}
}

func TestRunEarlyStop(t *testing.T) {
	s, _ := newTestScraper(t)
	// Mark a page-1 scene as known; date-sorted listing should stop early and
	// not emit it.
	opts := scraper.ListOpts{
		KnownIDs: map[string]bool{"Dalila-Lapiedra-1-Bukkake": true},
	}
	results := collect(t, s, opts)

	stopped := false
	for _, r := range results {
		if r.Kind == scraper.KindStoppedEarly {
			stopped = true
		}
		if r.Kind == scraper.KindScene && r.Scene.ID == "Dalila-Lapiedra-1-Bukkake" {
			t.Errorf("emitted a known scene")
		}
	}
	if !stopped {
		t.Errorf("expected an early-stop signal")
	}
}
