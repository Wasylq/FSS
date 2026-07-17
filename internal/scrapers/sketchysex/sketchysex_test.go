package sketchysex

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
		{"https://sketchysex.com/", true},
		{"https://www.sketchysex.com/index.php?page=2", true},
		{"http://sketchysex.com/trailer.php?id=358", true},
		{"https://darkhall.com/", false},
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

// The listing card is the only structured date — the detail page's own
// <div class="date"> is rendered empty.
func TestParseListing(t *testing.T) {
	items := parseListing(readFixture(t, "listing.html"))
	if len(items) != 3 {
		t.Fatalf("got %d items, want 3", len(items))
	}

	got := items[0]
	if got.id != "358" {
		t.Errorf("id = %q, want 358", got.id)
	}
	if got.title != "FEEDING FRENZY" {
		t.Errorf("title = %q", got.title)
	}
	want := time.Date(2026, time.July, 8, 0, 0, 0, 0, time.UTC)
	if !got.date.Equal(want) {
		t.Errorf("date = %v, want %v", got.date, want)
	}
	if !strings.HasPrefix(got.thumb, "https://") {
		t.Errorf("thumb = %q", got.thumb)
	}

	// Newest-first ordering underpins the KnownIDs early-stop.
	for i := 1; i < len(items); i++ {
		if items[i].date.After(items[i-1].date) {
			t.Errorf("item %d is newer than item %d", i, i-1)
		}
	}
}

func TestParseListingEmpty(t *testing.T) {
	if items := parseListing([]byte(`<html><body>nothing</body></html>`)); len(items) != 0 {
		t.Errorf("got %d items, want 0", len(items))
	}
}

func TestApplyDetail(t *testing.T) {
	var scene models.Scene
	applyDetail(&scene, string(readFixture(t, "detail.html")))

	if scene.Description == "" {
		t.Fatal("Description is empty")
	}
	// The description opens with a prose date that must be stripped.
	if strings.HasPrefix(scene.Description, "July") {
		t.Errorf("Description still carries its date prefix: %q", scene.Description)
	}
	if len(scene.Tags) == 0 {
		t.Fatal("Tags are empty")
	}
	if !slices.Contains(scene.Tags, "anonymous") {
		t.Errorf("Tags = %v, expected 'anonymous' among them", scene.Tags)
	}
	seen := map[string]bool{}
	for _, tag := range scene.Tags {
		if seen[tag] {
			t.Errorf("duplicate tag %q", tag)
		}
		seen[tag] = true
	}
	// The cast container is rendered but empty — the site's premise is
	// anonymous — so performers are expected to be nil, not a bogus entry.
	if len(scene.Performers) != 0 {
		t.Errorf("Performers = %v, want none for this scene", scene.Performers)
	}
}

func TestApplyDetailStripsDatePrefix(t *testing.T) {
	cases := map[string]string{
		`<div class="VideoDescription">July 8th, 2026 - What do we do?</div>`:    "What do we do?",
		`<div class="VideoDescription">March 1st, 2020 - Something else.</div>`:  "Something else.",
		`<div class="VideoDescription">No date prefix at all.</div>`:             "No date prefix at all.",
		`<div class="VideoDescription">December 22nd, 2019 - Another one.</div>`: "Another one.",
	}
	for detail, want := range cases {
		var scene models.Scene
		applyDetail(&scene, detail)
		if scene.Description != want {
			t.Errorf("Description = %q, want %q", scene.Description, want)
		}
	}
}

// ---- end-to-end ----

func TestListScenes(t *testing.T) {
	listing := readFixture(t, "listing.html")
	detail := readFixture(t, "detail.html")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/trailer.php") {
			_, _ = w.Write(detail)
			return
		}
		if r.URL.Query().Get("page") == "1" {
			_, _ = w.Write(listing)
			return
		}
		_, _ = fmt.Fprint(w, `<html><body></body></html>`)
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
		if sc.SiteID != siteID || sc.Studio != studioName {
			t.Errorf("scene %s: SiteID=%q Studio=%q", sc.ID, sc.SiteID, sc.Studio)
		}
		if sc.Title == "" || sc.Date.IsZero() {
			t.Errorf("scene %s incomplete: %+v", sc.ID, sc)
		}
		if len(sc.Tags) == 0 {
			t.Errorf("scene %s has no tags", sc.ID)
		}
		// The site publishes no runtime anywhere.
		if sc.Duration != 0 {
			t.Errorf("scene %s has a duration (%d) but the site publishes none", sc.ID, sc.Duration)
		}
	}
}

func TestDetailFailureIsNotFatal(t *testing.T) {
	listing := readFixture(t, "listing.html")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/trailer.php") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.URL.Query().Get("page") == "1" {
			_, _ = w.Write(listing)
			return
		}
		_, _ = fmt.Fprint(w, `<html></html>`)
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
		if sc.Title == "" || sc.Date.IsZero() {
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
