package sapphix

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

	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/scraper"
)

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return b
}

// newTestServer serves the listing fixture on /movies/page-1/ and the detail
// fixture on every /movies/{slug}/ path. Page 2+ returns the same listing so
// cross-page dedup terminates the loop after one real page.
func newTestServer(t *testing.T, listing, detail []byte) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		switch {
		case strings.HasPrefix(r.URL.Path, "/movies/page-"):
			_, _ = fmt.Fprint(w, string(listing))
		case strings.HasPrefix(r.URL.Path, "/movies/"):
			_, _ = fmt.Fprint(w, string(detail))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func collect(t *testing.T, s *Scraper, opts scraper.ListOpts) ([]models.Scene, []error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ch, err := s.ListScenes(ctx, s.base, opts)
	if err != nil {
		t.Fatalf("ListScenes: %v", err)
	}
	var scenes []models.Scene
	var errs []error
	for r := range ch {
		switch r.Kind {
		case scraper.KindScene:
			scenes = append(scenes, r.Scene)
		case scraper.KindError:
			errs = append(errs, r.Err)
		}
	}
	return scenes, errs
}

func TestParseListingPage(t *testing.T) {
	items := parseListingPage(readFixture(t, "listing.html"))
	if len(items) != 3 {
		t.Fatalf("expected 3 cards, got %d", len(items))
	}
	first := items[0]
	if first.id != "got-boobs" {
		t.Errorf("id = %q, want got-boobs", first.id)
	}
	if first.title != "Got boobs" {
		t.Errorf("title = %q, want Got boobs", first.title)
	}
	want := time.Date(2026, 5, 30, 0, 0, 0, 0, time.UTC)
	if !first.date.Equal(want) {
		t.Errorf("date = %v, want %v", first.date, want)
	}
	if !strings.Contains(first.thumbnail, "/cover/") {
		t.Errorf("thumbnail = %q, want a /cover/ url", first.thumbnail)
	}
}

func TestParseDetailPage(t *testing.T) {
	d := parseDetailPage(readFixture(t, "detail.html"))
	if d.title != "Anal vibrations" {
		t.Errorf("title = %q, want Anal vibrations", d.title)
	}
	want := time.Date(2026, 4, 9, 0, 0, 0, 0, time.UTC)
	if !d.date.Equal(want) {
		t.Errorf("date = %v, want %v", d.date, want)
	}
	if !strings.HasPrefix(d.description, "Lulu Love and Polina Maxima") {
		t.Errorf("description = %q", d.description)
	}
	if len(d.performers) != 2 || d.performers[0] != "Lulu Love" || d.performers[1] != "Polina Maxima" {
		t.Errorf("performers = %v, want [Lulu Love Polina Maxima]", d.performers)
	}
	if len(d.tags) == 0 {
		t.Fatal("expected tags, got none")
	}
	found := false
	for _, tag := range d.tags {
		if tag == "Anal" {
			found = true
		}
	}
	if !found {
		t.Errorf("tags %v missing 'Anal'", d.tags)
	}
}

func TestRunMergesAndDedupes(t *testing.T) {
	srv := newTestServer(t, readFixture(t, "listing.html"), readFixture(t, "detail.html"))
	s := New(sites[0]) // sapphicerotica
	s.base = srv.URL

	scenes, errs := collect(t, s, scraper.ListOpts{})
	for _, e := range errs {
		t.Errorf("unexpected error: %v", e)
	}
	// 3 unique cards; page 2 repeats them so cross-page dedup stops the loop.
	if len(scenes) != 3 {
		t.Fatalf("expected 3 scenes, got %d", len(scenes))
	}

	var got *models.Scene
	for i := range scenes {
		if scenes[i].ID == "got-boobs" {
			got = &scenes[i]
		}
	}
	if got == nil {
		t.Fatal("got-boobs scene missing")
	}
	if got.SiteID != "sapphicerotica" {
		t.Errorf("SiteID = %q, want sapphicerotica", got.SiteID)
	}
	if got.Studio != "Sapphic Erotica" {
		t.Errorf("Studio = %q, want Sapphic Erotica", got.Studio)
	}
	if got.URL != srv.URL+"/movies/got-boobs/" {
		t.Errorf("URL = %q", got.URL)
	}
	// Detail page supplies title/date/desc/models/tags (overriding the card).
	if got.Title != "Anal vibrations" {
		t.Errorf("Title = %q (detail should win)", got.Title)
	}
	if len(got.Performers) != 2 {
		t.Errorf("Performers = %v", got.Performers)
	}
	if got.Description == "" {
		t.Error("Description empty")
	}
	if got.ScrapedAt.IsZero() {
		t.Error("ScrapedAt zero")
	}
}

func TestKnownIDsEarlyStop(t *testing.T) {
	srv := newTestServer(t, readFixture(t, "listing.html"), readFixture(t, "detail.html"))
	s := New(sites[0])
	s.base = srv.URL

	scenes, _ := collect(t, s, scraper.ListOpts{KnownIDs: map[string]bool{"got-boobs": true}})
	for _, sc := range scenes {
		if sc.ID == "got-boobs" {
			t.Errorf("known ID got-boobs should not be emitted as a full scene")
		}
	}
}

func TestSecondSiteListing(t *testing.T) {
	items := parseListingPage(readFixture(t, "listing_fistflush.html"))
	if len(items) != 2 {
		t.Fatalf("expected 2 fistflush cards, got %d", len(items))
	}
	if items[0].id != "nesty-linda-leclair" {
		t.Errorf("id = %q, want nesty-linda-leclair", items[0].id)
	}
	if items[0].title != "Nesty & Linda Leclair" {
		t.Errorf("title = %q", items[0].title)
	}
	want := time.Date(2026, 3, 13, 0, 0, 0, 0, time.UTC)
	if !items[0].date.Equal(want) {
		t.Errorf("date = %v, want %v", items[0].date, want)
	}
}

func TestMatchesURL(t *testing.T) {
	s := newFor("sapphix")
	if s == nil {
		t.Fatal("newFor(sapphix) returned nil")
	}
	for _, u := range []string{
		"https://www.sapphix.com/",
		"https://sapphix.com/movies/page-1/",
		"http://www.sapphix.com/movies/foo/",
	} {
		if !s.MatchesURL(u) {
			t.Errorf("MatchesURL(%q) = false, want true", u)
		}
	}
	if s.MatchesURL("https://www.sapphicerotica.com/") {
		t.Error("sapphix scraper should not match sapphicerotica.com")
	}
}
