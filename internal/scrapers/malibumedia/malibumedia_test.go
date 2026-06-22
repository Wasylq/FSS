package malibumedia

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/scraper"
)

func loadFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return b
}

func xartScraper() *Scraper { return newFor("x-art") }

func coletteScraper() *Scraper { return newFor("colettevideos") }

func TestParseListingXArt(t *testing.T) {
	items := xartScraper().parseListing(loadFixture(t, "xart_listing.html"))
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	first := items[0]
	if first.id != "what-an-amazing-ass" {
		t.Errorf("id = %q", first.id)
	}
	if first.title != "What an Amazing Ass" {
		t.Errorf("title = %q", first.title)
	}
	if first.thumbnail != "https://www.x-art.com/videos/what-an-amazing-ass/what-an-amazing-ass-02.jpg" {
		t.Errorf("thumbnail = %q", first.thumbnail)
	}
	if items[1].id != "first-time-lesbians" {
		t.Errorf("second id = %q", items[1].id)
	}
}

func TestParseListingColette(t *testing.T) {
	items := coletteScraper().parseListing(loadFixture(t, "colette_listing.html"))
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	if items[0].id != "orgy_is_the_new_black" {
		t.Errorf("id = %q", items[0].id)
	}
	if items[1].id != "skip_the_foreplay" {
		t.Errorf("second id = %q", items[1].id)
	}
}

func TestParseListingEmpty(t *testing.T) {
	if got := xartScraper().parseListing([]byte("<html><body>nothing</body></html>")); len(got) != 0 {
		t.Fatalf("got %d items, want 0", len(got))
	}
}

func TestMaxPageNum(t *testing.T) {
	if got := maxPageNum(loadFixture(t, "xart_listing.html")); got != 24 {
		t.Errorf("maxPageNum = %d, want 24", got)
	}
}

func TestParseDetailXArt(t *testing.T) {
	d := parseDetail(loadFixture(t, "xart_detail.html"))
	if d.title != "What an Amazing Ass" {
		t.Errorf("title = %q", d.title)
	}
	if !strings.HasPrefix(d.description, "This husband and wife pair") {
		t.Errorf("description = %q", d.description)
	}
	if got := d.date.Format("2006-01-02"); got != "2026-06-18" {
		t.Errorf("date = %q, want 2026-06-18", got)
	}
	if len(d.performers) != 2 || d.performers[0] != "Caprice" || d.performers[1] != "Marcello" {
		t.Errorf("performers = %v, want [Caprice Marcello]", d.performers)
	}
}

func TestParseDetailColette(t *testing.T) {
	d := parseDetail(loadFixture(t, "colette_detail.html"))
	if d.title != "Orgy is the New Black" {
		t.Errorf("title = %q", d.title)
	}
	if got := d.date.Format("2006-01-02"); got != "2015-05-04" {
		t.Errorf("date = %q, want 2015-05-04", got)
	}
	want := []string{"Marica", "Kacy Lane", "Aria"}
	if len(d.performers) != len(want) {
		t.Fatalf("performers = %v, want %v", d.performers, want)
	}
	for i, w := range want {
		if d.performers[i] != w {
			t.Errorf("performers[%d] = %q, want %q", i, d.performers[i], w)
		}
	}
	// Colette has no og:description; the fallback <p> block (with inner tags
	// stripped) supplies the description.
	if !strings.HasPrefix(d.description, "He had no problem handling all three") {
		t.Errorf("description = %q", d.description)
	}
	if d.thumbnail != "https://www.colettevideos.com/videos/orgy_is_the_new_black/colette_orgy_is_the_new_black_3.jpg" {
		t.Errorf("thumbnail = %q", d.thumbnail)
	}
}

func TestMatchesURL(t *testing.T) {
	x := xartScraper()
	c := coletteScraper()
	cases := []struct {
		url     string
		xart    bool
		colette bool
	}{
		{"https://www.x-art.com/", true, false},
		{"https://x-art.com/videos/all/2", true, false},
		{"https://www.colettevideos.com/updates/", false, true},
		{"https://www.example.com/", false, false},
	}
	for _, tc := range cases {
		if x.MatchesURL(tc.url) != tc.xart {
			t.Errorf("x-art.MatchesURL(%q) = %v, want %v", tc.url, x.MatchesURL(tc.url), tc.xart)
		}
		if c.MatchesURL(tc.url) != tc.colette {
			t.Errorf("colette.MatchesURL(%q) = %v, want %v", tc.url, c.MatchesURL(tc.url), tc.colette)
		}
	}
}

func TestNewFor(t *testing.T) {
	if newFor("x-art").ID() != "x-art" {
		t.Error("newFor x-art")
	}
	if newFor("colettevideos").ID() != "colettevideos" {
		t.Error("newFor colettevideos")
	}
	if newFor("nope") != nil {
		t.Error("newFor unknown should be nil")
	}
}

// xartServer serves the X-Art fixtures: the /videos/all/1 listing, scene
// detail pages, and empty bodies for later pages (stops pagination).
func xartServer(t *testing.T, listing, detail []byte) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/videos/all/1":
			_, _ = fmt.Fprint(w, string(listing))
		case strings.HasPrefix(r.URL.Path, "/videos/"):
			_, _ = fmt.Fprint(w, string(detail))
		default:
			_, _ = fmt.Fprint(w, "<html><body></body></html>")
		}
	}))
}

// coletteServer serves the single-page /updates/ listing plus detail pages.
func coletteServer(t *testing.T, listing, detail []byte) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/updates/":
			_, _ = fmt.Fprint(w, string(listing))
		case strings.HasPrefix(r.URL.Path, "/videos/"):
			_, _ = fmt.Fprint(w, string(detail))
		default:
			_, _ = fmt.Fprint(w, "<html><body></body></html>")
		}
	}))
}

func collect(t *testing.T, s *Scraper, opts scraper.ListOpts) ([]models.Scene, bool) {
	t.Helper()
	ch, err := s.ListScenes(context.Background(), s.base+"/", opts)
	if err != nil {
		t.Fatalf("ListScenes: %v", err)
	}
	var scenes []models.Scene
	stopped := false
	for r := range ch {
		switch r.Kind {
		case scraper.KindScene:
			scenes = append(scenes, r.Scene)
		case scraper.KindError:
			t.Errorf("error result: %v", r.Err)
		case scraper.KindStoppedEarly:
			stopped = true
		}
	}
	return scenes, stopped
}

func TestRunXArt(t *testing.T) {
	ts := xartServer(t, loadFixture(t, "xart_listing.html"), loadFixture(t, "xart_detail.html"))
	defer ts.Close()

	s := xartScraper()
	s.base = ts.URL
	s.Client = ts.Client()

	scenes, _ := collect(t, s, scraper.ListOpts{Workers: 2})
	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2", len(scenes))
	}
	sc := scenes[0]
	if sc.ID != "what-an-amazing-ass" {
		t.Errorf("ID = %q", sc.ID)
	}
	if sc.SiteID != "x-art" {
		t.Errorf("SiteID = %q", sc.SiteID)
	}
	if sc.Studio != "X-Art" {
		t.Errorf("Studio = %q", sc.Studio)
	}
	if sc.Title != "What an Amazing Ass" {
		t.Errorf("Title = %q", sc.Title)
	}
	if sc.URL != ts.URL+"/videos/what-an-amazing-ass" {
		t.Errorf("URL = %q", sc.URL)
	}
	if sc.StudioURL != ts.URL {
		t.Errorf("StudioURL = %q", sc.StudioURL)
	}
	if got := sc.Date.Format("2006-01-02"); got != "2026-06-18" {
		t.Errorf("Date = %q", got)
	}
	if len(sc.Performers) != 2 {
		t.Errorf("Performers = %v", sc.Performers)
	}
	if sc.Description == "" {
		t.Error("Description empty")
	}
	if sc.Thumbnail == "" {
		t.Error("Thumbnail empty")
	}
	if sc.ScrapedAt.IsZero() {
		t.Error("ScrapedAt zero")
	}
}

func TestRunColette(t *testing.T) {
	ts := coletteServer(t, loadFixture(t, "colette_listing.html"), loadFixture(t, "colette_detail.html"))
	defer ts.Close()

	s := coletteScraper()
	s.base = ts.URL
	s.Client = ts.Client()

	scenes, _ := collect(t, s, scraper.ListOpts{Workers: 2})
	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2", len(scenes))
	}
	for _, sc := range scenes {
		if sc.SiteID != "colettevideos" || sc.Studio != "Colette" {
			t.Errorf("wrong site/studio: %q / %q", sc.SiteID, sc.Studio)
		}
		if sc.ID == "" || sc.Title == "" {
			t.Errorf("missing id/title: %+v", sc)
		}
		if !strings.HasSuffix(sc.URL, "/") {
			t.Errorf("colette URL should have trailing slash: %q", sc.URL)
		}
	}
}

func TestRunKnownIDsEarlyStopXArt(t *testing.T) {
	ts := xartServer(t, loadFixture(t, "xart_listing.html"), loadFixture(t, "xart_detail.html"))
	defer ts.Close()

	s := xartScraper()
	s.base = ts.URL
	s.Client = ts.Client()

	known := "what-an-amazing-ass"
	scenes, stopped := collect(t, s, scraper.ListOpts{
		Workers:  2,
		KnownIDs: map[string]bool{known: true},
	})
	if !stopped {
		t.Error("expected StoppedEarly result")
	}
	for _, sc := range scenes {
		if sc.ID == known {
			t.Errorf("known scene should not be emitted: %q", known)
		}
	}
}

func TestRunKnownIDsEarlyStopColette(t *testing.T) {
	ts := coletteServer(t, loadFixture(t, "colette_listing.html"), loadFixture(t, "colette_detail.html"))
	defer ts.Close()

	s := coletteScraper()
	s.base = ts.URL
	s.Client = ts.Client()

	known := "orgy_is_the_new_black"
	scenes, stopped := collect(t, s, scraper.ListOpts{
		Workers:  2,
		KnownIDs: map[string]bool{known: true},
	})
	if !stopped {
		t.Error("expected StoppedEarly result")
	}
	for _, sc := range scenes {
		if sc.ID == known {
			t.Errorf("known scene should not be emitted: %q", known)
		}
	}
}
