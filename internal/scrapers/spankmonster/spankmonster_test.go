package spankmonster

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/scraper"
)

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("reading fixture %s: %v", name, err)
	}
	return b
}

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url   string
		match bool
	}{
		{"https://www.spankmonster.com/", true},
		{"https://spankmonster.com/spank-monster-updates.html?page=2", true},
		{"https://helixstudios.com/", false},
		{"", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.match {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.match)
		}
	}
}

func TestID(t *testing.T) {
	if got := New().ID(); got != siteID {
		t.Errorf("ID() = %q, want %q", got, siteID)
	}
}

// The CMS wraps attributes across lines, so the card regexes must tolerate
// whitespace between attribute pairs — a literal space matches nothing.
func TestParseListing(t *testing.T) {
	items := parseListing(readFixture(t, "listing.html"))
	if len(items) != 3 {
		t.Fatalf("got %d items, want 3", len(items))
	}

	got := items[0]
	if got.id != "1795495" {
		t.Errorf("id = %q", got.id)
	}
	if !strings.Contains(got.url, "/1795495/") {
		t.Errorf("url = %q", got.url)
	}
	// The card title is prefixed with a rounded runtime, which must be stripped.
	if strings.Contains(got.title, "min |") {
		t.Errorf("title still carries the runtime prefix: %q", got.title)
	}
	if !strings.HasPrefix(got.title, "Tiny Spinner White Girl") {
		t.Errorf("title = %q", got.title)
	}
	if !slices.Equal(got.performers, []string{"Krystal Palmer"}) {
		t.Errorf("performers = %v", got.performers)
	}
	if !strings.HasPrefix(got.thumb, "https://imgs1cdn.adultempire.com/") {
		t.Errorf("thumb = %q", got.thumb)
	}
}

func TestParseListingEmpty(t *testing.T) {
	if items := parseListing([]byte(`<html><body>nothing</body></html>`)); len(items) != 0 {
		t.Errorf("got %d items, want 0", len(items))
	}
}

func TestApplyDetail(t *testing.T) {
	scene := models.Scene{Title: "51 min | truncated card title", Duration: 35 * 60}
	applyDetail(&scene, string(readFixture(t, "detail.html")))

	// The <h1> is the full title, without the runtime prefix.
	if !strings.HasPrefix(scene.Title, "Tiny Spinner White Girl") {
		t.Errorf("Title = %q", scene.Title)
	}
	want := time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC)
	if !scene.Date.Equal(want) {
		t.Errorf("Date = %v, want %v", scene.Date, want)
	}
	// The card said 35 min; the detail page says 51 and must win.
	if scene.Duration != 51*60 {
		t.Errorf("Duration = %d, want %d (the detail page wins over the card)", scene.Duration, 51*60)
	}
	if scene.Director != "Donnie Cabo" {
		t.Errorf("Director = %q", scene.Director)
	}
	if !slices.Equal(scene.Performers, []string{"Krystal Palmer"}) {
		t.Errorf("Performers = %v", scene.Performers)
	}
	if len(scene.Tags) == 0 {
		t.Fatal("Tags are empty")
	}
	if !slices.Contains(scene.Tags, "POV") {
		t.Errorf("Tags = %v, expected POV among them", scene.Tags)
	}
	seen := map[string]bool{}
	for _, tag := range scene.Tags {
		if seen[tag] {
			t.Errorf("duplicate tag %q", tag)
		}
		seen[tag] = true
	}
}

func TestCleanText(t *testing.T) {
	cases := map[string]string{
		"\n\t\t\tKrystal Palmer\n\t\t": "Krystal Palmer",
		"A &amp; B":                    "A & B",
		"":                             "",
	}
	for in, want := range cases {
		if got := cleanText(in); got != want {
			t.Errorf("cleanText(%q) = %q, want %q", in, got, want)
		}
	}
}

// ---- end-to-end ----

// Every request must carry the age cookie; without it the site 302s to
// /AgeConfirmation and nothing is scraped.
func TestListScenesSendsAgeCookie(t *testing.T) {
	listing := readFixture(t, "listing.html")
	detail := readFixture(t, "detail.html")

	var missingCookie int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie("ageConfirmed"); err != nil || c.Value != "true" {
			missingCookie++
			http.Redirect(w, r, "/AgeConfirmation", http.StatusFound)
			return
		}
		switch {
		case strings.HasPrefix(r.URL.Path, "/spank-monster-updates.html"):
			if r.URL.Query().Get("page") == "1" {
				_, _ = w.Write(listing)
				return
			}
			_, _ = fmt.Fprint(w, `<html><body></body></html>`)
		default:
			_, _ = w.Write(detail)
		}
	}))
	defer srv.Close()

	orig := siteBase
	siteBase = srv.URL
	defer func() { siteBase = orig }()

	s := New()
	s.Client = srv.Client()

	ch, err := s.ListScenes(context.Background(), srv.URL, scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}
	scenes := testutil.CollectScenes(t, ch)

	if missingCookie != 0 {
		t.Errorf("%d requests were sent without the age cookie", missingCookie)
	}
	if len(scenes) != 3 {
		t.Fatalf("got %d scenes, want 3", len(scenes))
	}
	for _, sc := range scenes {
		if sc.SiteID != siteID || sc.Studio != studioName {
			t.Errorf("scene %s: SiteID=%q Studio=%q", sc.ID, sc.SiteID, sc.Studio)
		}
		if sc.Title == "" || sc.Date.IsZero() || sc.Duration == 0 {
			t.Errorf("scene %s incomplete: %+v", sc.ID, sc)
		}
	}
}

func TestDetailFailureIsNotFatal(t *testing.T) {
	listing := readFixture(t, "listing.html")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/spank-monster-updates.html") {
			if r.URL.Query().Get("page") == "1" {
				_, _ = w.Write(listing)
				return
			}
			_, _ = fmt.Fprint(w, `<html></html>`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	orig := siteBase
	siteBase = srv.URL
	defer func() { siteBase = orig }()

	s := New()
	s.Client = srv.Client()

	ch, err := s.ListScenes(context.Background(), srv.URL, scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}
	scenes := testutil.CollectScenes(t, ch)
	if len(scenes) != 3 {
		t.Fatalf("got %d scenes, want 3", len(scenes))
	}
	for _, sc := range scenes {
		if sc.Title == "" || len(sc.Performers) == 0 {
			t.Errorf("scene %s lost its card data", sc.ID)
		}
	}
}

func TestContextCancellation(t *testing.T) {
	listing := readFixture(t, "listing.html")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(listing)
	}))
	defer srv.Close()

	orig := siteBase
	siteBase = srv.URL
	defer func() { siteBase = orig }()

	s := New()
	s.Client = srv.Client()

	ctx, cancel := context.WithCancel(context.Background())
	ch, err := s.ListScenes(ctx, srv.URL, scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}
	cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for range ch {
		}
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("channel did not close after context cancellation")
	}
}
