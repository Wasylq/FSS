package alsangels

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/scraper"
)

func TestMatchesURL(t *testing.T) {
	s := New()
	good := []string{
		"https://alsangels.com/",
		"https://alsangels.com/dailyvideos.html",
		"http://www.alsangels.com/dailyvideos.html",
	}
	for _, u := range good {
		if !s.MatchesURL(u) {
			t.Errorf("MatchesURL(%q) = false, want true", u)
		}
	}
	bad := []string{
		"https://example.com/",
		"https://alscan.com/",
		"https://notalsangels.com/",
	}
	for _, u := range bad {
		if s.MatchesURL(u) {
			t.Errorf("MatchesURL(%q) = true, want false", u)
		}
	}
}

func newTestServer(t *testing.T, gotCookie *string) *httptest.Server {
	t.Helper()
	data, err := os.ReadFile("testdata/dailyvideos.html")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if gotCookie != nil {
			*gotCookie = r.Header.Get("Cookie")
		}
		if r.URL.Path != videosPath {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write(data)
	}))
}

func collect(t *testing.T, s *Scraper, opts scraper.ListOpts) ([]models.Scene, int, bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ch, err := s.ListScenes(ctx, "https://alsangels.com/", opts)
	if err != nil {
		t.Fatalf("ListScenes: %v", err)
	}
	var scenes []models.Scene
	var total int
	stoppedEarly := false
	for res := range ch {
		switch res.Kind {
		case scraper.KindScene:
			scenes = append(scenes, res.Scene)
		case scraper.KindTotal:
			total = res.Total
		case scraper.KindError:
			t.Fatalf("scrape error: %v", res.Err)
		case scraper.KindStoppedEarly:
			stoppedEarly = true
		}
	}
	return scenes, total, stoppedEarly
}

func TestScrapeFixture(t *testing.T) {
	var gotCookie string
	ts := newTestServer(t, &gotCookie)
	defer ts.Close()

	s := New()
	s.base = ts.URL

	scenes, total, _ := collect(t, s, scraper.ListOpts{})

	if !strings.Contains(gotCookie, "age_verified=true") {
		t.Errorf("age-gate cookie not sent; got Cookie header %q", gotCookie)
	}

	if len(scenes) != 4 {
		t.Fatalf("got %d scenes, want 4", len(scenes))
	}
	if total != 4 {
		t.Errorf("progress total = %d, want 4", total)
	}

	first := scenes[0]
	if first.ID != "onyxreign002" {
		t.Errorf("ID = %q, want onyxreign002", first.ID)
	}
	if first.SiteID != "alsangels" {
		t.Errorf("SiteID = %q, want alsangels", first.SiteID)
	}
	if first.Studio != "ALS Angels" {
		t.Errorf("Studio = %q, want ALS Angels", first.Studio)
	}
	if len(first.Performers) != 1 || first.Performers[0] != "Onyx Reign" {
		t.Errorf("Performers = %v, want [Onyx Reign]", first.Performers)
	}
	if !strings.Contains(first.Title, "Onyx Reign") {
		t.Errorf("Title = %q, want it to contain Onyx Reign", first.Title)
	}
	if first.Duration != 18*60+53 {
		t.Errorf("Duration = %d, want %d", first.Duration, 18*60+53)
	}
	wantDate := time.Date(2026, 6, 18, 0, 0, 0, 0, time.UTC)
	if !first.Date.Equal(wantDate) {
		t.Errorf("Date = %v, want %v", first.Date, wantDate)
	}
	if !strings.HasPrefix(first.Description, "Onyx Reign hits one pose") {
		t.Errorf("Description = %q", first.Description)
	}
	if !strings.Contains(first.Thumbnail, "onyxreign002banner1000x400.jpg") {
		t.Errorf("Thumbnail = %q", first.Thumbnail)
	}
	if !strings.HasSuffix(first.URL, "/dailyvideos.html#onyxreign002") {
		t.Errorf("URL = %q", first.URL)
	}
	if len(first.Tags) != 1 || first.Tags[0] != "Photoshoot" {
		t.Errorf("Tags = %v, want [Photoshoot]", first.Tags)
	}
	if first.ScrapedAt.IsZero() {
		t.Error("ScrapedAt is zero")
	}

	// Sanity-check another block parsed too.
	if scenes[1].ID != "bristonmoreau009" {
		t.Errorf("scenes[1].ID = %q, want bristonmoreau009", scenes[1].ID)
	}
}

func TestScrapeKnownIDStopsEarly(t *testing.T) {
	ts := newTestServer(t, nil)
	defer ts.Close()

	s := New()
	s.base = ts.URL

	opts := scraper.ListOpts{KnownIDs: map[string]bool{"isabellajules007": true}}
	scenes, _, stoppedEarly := collect(t, s, opts)

	if !stoppedEarly {
		t.Error("expected StoppedEarly signal")
	}
	// onyxreign002, bristonmoreau009 emitted before hitting the known ID.
	if len(scenes) != 2 {
		t.Fatalf("got %d scenes before early stop, want 2", len(scenes))
	}
}
