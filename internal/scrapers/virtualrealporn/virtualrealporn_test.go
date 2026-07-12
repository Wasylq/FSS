package virtualrealporn

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/scraper"
)

func readFixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return string(b)
}

// newTestServer serves a sitemap index, a single videos_sitemap (with the
// three real fixture scene URLs rewritten to point at the server), and the
// trimmed detail fixture for every scene.
func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	sitemap := readFixture(t, "videos_sitemap.xml")
	detail := readFixture(t, "detail-russian-shower.html")

	var srv *httptest.Server
	mux := http.NewServeMux()
	mux.HandleFunc("/sitemap.xml", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
<sitemap><loc>%s/videos_sitemap.xml</loc></sitemap>
</sitemapindex>`, srv.URL)
	})
	mux.HandleFunc("/videos_sitemap.xml", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		// Rewrite the real scene loc origins to the test server.
		body := strings.ReplaceAll(sitemap, "https://virtualrealporn.com", srv.URL)
		_, _ = fmt.Fprint(w, body)
	})
	// Every scene detail path returns the same trimmed real fixture.
	mux.HandleFunc("/vr-porn-video/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, detail)
	})
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func newTestScraper(t *testing.T, srv *httptest.Server) *Scraper {
	t.Helper()
	s := New(sites[0]) // virtualrealporn
	s.Client = srv.Client()
	s.base = srv.URL
	return s
}

func collect(t *testing.T, ch <-chan scraper.SceneResult) ([]models.Scene, int, bool) {
	t.Helper()
	var scenes []models.Scene
	var total int
	var stopped bool
	for r := range ch {
		switch r.Kind {
		case scraper.KindScene:
			scenes = append(scenes, r.Scene)
		case scraper.KindError:
			t.Fatalf("scraper error: %v", r.Err)
		case scraper.KindTotal:
			total = r.Total
		case scraper.KindStoppedEarly:
			stopped = true
		}
	}
	return scenes, total, stopped
}

func TestListScenesParsesNewestFirst(t *testing.T) {
	srv := newTestServer(t)
	s := newTestScraper(t, srv)

	ch, err := s.ListScenes(context.Background(), srv.URL, scraper.ListOpts{})
	if err != nil {
		t.Fatalf("ListScenes: %v", err)
	}
	scenes, total, _ := collect(t, ch)

	if len(scenes) != 3 {
		t.Fatalf("want 3 scenes, got %d", len(scenes))
	}
	if total != 3 {
		t.Errorf("want total 3, got %d", total)
	}

	// Sitemap is oldest-first (russian-shower, bubble-bath, canceled-party).
	// Newest-first reversal must put canceled-party first, russian-shower last.
	if got := scenes[0].ID; got != "canceled-party" {
		t.Errorf("first scene ID = %q, want canceled-party", got)
	}
	if got := scenes[len(scenes)-1].ID; got != "russian-shower" {
		t.Errorf("last scene ID = %q, want russian-shower", got)
	}
}

func TestSceneFieldsFromJSONLD(t *testing.T) {
	srv := newTestServer(t)
	s := newTestScraper(t, srv)

	ch, err := s.ListScenes(context.Background(), srv.URL, scraper.ListOpts{})
	if err != nil {
		t.Fatalf("ListScenes: %v", err)
	}
	scenes, _, _ := collect(t, ch)

	// All scenes use the same detail fixture (russian-shower).
	sc := scenes[0]
	if sc.SiteID != "virtualrealporn" {
		t.Errorf("SiteID = %q, want virtualrealporn", sc.SiteID)
	}
	if sc.Studio != "VirtualRealPorn" {
		t.Errorf("Studio = %q, want VirtualRealPorn", sc.Studio)
	}
	if sc.Title != "Russian shower" {
		t.Errorf("Title = %q, want \"Russian shower\" (suffix stripped)", sc.Title)
	}
	if sc.Duration != 28*60+36 {
		t.Errorf("Duration = %d, want %d", sc.Duration, 28*60+36)
	}
	if sc.Date.Year() != 2017 || sc.Date.Month() != 1 {
		t.Errorf("Date = %v, want 2017-01", sc.Date)
	}
	// Performers no longer come from JSON-LD (the current VideoObject block
	// omits "actors") — they're scraped from the "VR Pornstars" section, and
	// the trailing " VR" suffix on each name must be stripped.
	if len(sc.Performers) != 1 || sc.Performers[0] != "Nancy Ace" {
		t.Errorf("Performers = %v, want [Nancy Ace]", sc.Performers)
	}
	if sc.Description == "" {
		t.Error("Description is empty")
	}
	if !strings.Contains(sc.Thumbnail, "slider-russian-shower.jpg") {
		t.Errorf("Thumbnail = %q, want slider-russian-shower.jpg", sc.Thumbnail)
	}
	// URL is the per-scene sitemap loc (newest first = canceled-party); the
	// JSON-LD content fixture is shared, so URL and ID come from the listing.
	if !strings.HasSuffix(sc.URL, "/vr-porn-video/canceled-party/") {
		t.Errorf("URL = %q, want .../vr-porn-video/canceled-party/", sc.URL)
	}
	if sc.ID != "canceled-party" {
		t.Errorf("ID = %q, want canceled-party", sc.ID)
	}
	// Tags likewise come from the page's own tag-list markup now (JSON-LD no
	// longer carries "keywords"/"genre").
	if !hasTag(sc.Tags, "russian") || !hasTag(sc.Tags, "blonde") {
		t.Errorf("specific tags missing: %v", sc.Tags)
	}
}

func TestKnownIDsStopEarly(t *testing.T) {
	srv := newTestServer(t)
	s := newTestScraper(t, srv)

	// canceled-party is newest (first emitted). Marking it known stops at once.
	opts := scraper.ListOpts{KnownIDs: map[string]bool{"canceled-party": true}}
	ch, err := s.ListScenes(context.Background(), srv.URL, opts)
	if err != nil {
		t.Fatalf("ListScenes: %v", err)
	}
	scenes, _, stopped := collect(t, ch)
	if !stopped {
		t.Error("expected StoppedEarly")
	}
	if len(scenes) != 0 {
		t.Errorf("expected 0 scenes before known ID, got %d", len(scenes))
	}
}

func TestMatchesURL(t *testing.T) {
	s := newFor("virtualrealporn")
	cases := map[string]bool{
		"https://virtualrealporn.com/":                   true,
		"https://www.virtualrealporn.com/latest-videos/": true,
		"https://virtualrealporn.com/vr-porn-video/x/":   true,
		"https://virtualrealgay.com/":                    false,
		"https://example.com/virtualrealporn.com":        false,
	}
	for u, want := range cases {
		if got := s.MatchesURL(u); got != want {
			t.Errorf("MatchesURL(%q) = %v, want %v", u, got, want)
		}
	}

	gay := newFor("virtualrealgay")
	if !gay.MatchesURL("https://virtualrealgay.com/") {
		t.Error("gay scraper should match its own domain")
	}
	if gay.MatchesURL("https://virtualrealporn.com/") {
		t.Error("gay scraper should not match virtualrealporn.com")
	}
}

func TestAllSitesRegistered(t *testing.T) {
	want := []string{"virtualrealporn", "virtualrealgay", "virtualrealtrans", "virtualrealjapan", "virtualrealpassion"}
	for _, id := range want {
		if newFor(id) == nil {
			t.Errorf("site %q not registered", id)
		}
	}
}

func TestCleanTitle(t *testing.T) {
	cases := map[string]string{
		"Russian shower | VirtualRealPorn VR Porn video": "Russian shower",
		"Wet, wet, wet | VirtualRealGay VR Porn video":   "Wet, wet, wet",
		"No Suffix": "No Suffix",
	}
	for in, want := range cases {
		if got := cleanTitle(in); got != want {
			t.Errorf("cleanTitle(%q) = %q, want %q", in, got, want)
		}
	}
}

func hasTag(tags []string, t string) bool {
	for _, x := range tags {
		if strings.EqualFold(x, t) {
			return true
		}
	}
	return false
}
