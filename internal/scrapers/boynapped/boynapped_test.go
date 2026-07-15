package boynapped

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"sync/atomic"
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
		{"https://www.boynapped.com/", true},
		{"https://boynapped.com/ultimatekink/movies/newest?page=2", true},
		{"http://www.boynapped.com/ultimatekink/movie/boyd0025", true},
		{"https://badboybondage.com/", false},
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

// ---- listing ----

func TestParseListing(t *testing.T) {
	items := parseListing(readFixture(t, "listing.html"))
	if len(items) != 3 {
		t.Fatalf("got %d items, want 3", len(items))
	}

	got := items[0]
	// The slug is the scene id.
	if got.id != "boyd0025_tyga_mitchschif_sebastiankane_part02" {
		t.Errorf("id = %q", got.id)
	}
	if got.title != "Another Round Of Torment For Tyga! - Part 2" {
		t.Errorf("title = %q", got.title)
	}
	// "16:28"
	if got.duration != 988 {
		t.Errorf("duration = %d, want 988", got.duration)
	}
	want := time.Date(2026, time.July, 17, 0, 0, 0, 0, time.UTC)
	if !got.date.Equal(want) {
		t.Errorf("date = %v, want %v", got.date, want)
	}
	wantPerf := []string{"Master Kane", "Mitch Schif", "Tyga"}
	if !slices.Equal(got.performers, wantPerf) {
		t.Errorf("performers = %v, want %v", got.performers, wantPerf)
	}
	// Protocol-relative CDN URLs must be upgraded.
	if !strings.HasPrefix(got.thumb, "https://") {
		t.Errorf("thumb = %q", got.thumb)
	}

	// Newest-first ordering underpins the KnownIDs early-stop.
	for i := 1; i < len(items); i++ {
		if items[i].date.After(items[i-1].date) {
			t.Errorf("item %d is newer than item %d; listing must be newest-first", i, i-1)
		}
	}
}

// The card date is unpadded ("Jul 8, 2026"), which needs the "Jan 2, 2006"
// layout rather than "Jan 02, 2006".
func TestParseListingUnpaddedDate(t *testing.T) {
	card := `<div class="content-item">
<h3 class="title"><a href="https://www.boynapped.com/ultimatekink/movie/x1" > A Scene </a></h3>
<span class="pub-date"><i class="fas fa-calendar-alt"></i>&nbsp;Jul 8, 2026 </span>`

	items := parseListing([]byte(card))
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	want := time.Date(2026, time.July, 8, 0, 0, 0, 0, time.UTC)
	if !items[0].date.Equal(want) {
		t.Errorf("date = %v, want %v", items[0].date, want)
	}
}

// The photo count sits in its own span next to the duration; only the clock
// value is a runtime.
func TestParseListingDurationNotPhotoCount(t *testing.T) {
	card := `<div class="content-item">
<h3 class="title"><a href="https://www.boynapped.com/ultimatekink/movie/x2" > B </a></h3>
<span class="total-photos"><i class="fas fa-image"></i>&nbsp;79 </span>
<span class="video-duration"><i class="fas fa-play"></i>&nbsp;16:28 </span>`

	items := parseListing([]byte(card))
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].duration != 988 {
		t.Errorf("duration = %d, want 988 (16:28, not the 79 photo count)", items[0].duration)
	}
}

func TestParseListingEmpty(t *testing.T) {
	if items := parseListing([]byte(`<html><body>nothing</body></html>`)); len(items) != 0 {
		t.Errorf("got %d items, want 0", len(items))
	}
}

func TestNormalizeURL(t *testing.T) {
	cases := map[string]string{
		"//cloud-nexpectation.secure.yppcdn.com/boyn/a.jpg":  "https://cloud-nexpectation.secure.yppcdn.com/boyn/a.jpg",
		"https://cloud-nexpectation.secure.yppcdn.com/b.jpg": "https://cloud-nexpectation.secure.yppcdn.com/b.jpg",
	}
	for in, want := range cases {
		if got := normalizeURL(in); got != want {
			t.Errorf("normalizeURL(%q) = %q, want %q", in, got, want)
		}
	}
}

// ---- detail ----

func TestApplyDetail(t *testing.T) {
	var scene models.Scene
	applyDetail(&scene, string(readFixture(t, "detail.html")))

	if !strings.HasPrefix(scene.Description, "Tyga is such a horny boy") {
		t.Errorf("Description = %q", scene.Description)
	}
	// Entities must be decoded and markup stripped.
	if strings.Contains(scene.Description, "&#") || strings.Contains(scene.Description, "<") {
		t.Errorf("Description not cleaned: %q", scene.Description)
	}
	if len(scene.Tags) == 0 {
		t.Fatal("Tags are empty")
	}
	seen := map[string]bool{}
	for _, tag := range scene.Tags {
		if seen[tag] {
			t.Errorf("duplicate tag %q", tag)
		}
		seen[tag] = true
	}
}

func TestApplyDetailNoBlocks(t *testing.T) {
	var scene models.Scene
	applyDetail(&scene, `<html><body>nothing here</body></html>`)

	if scene.Description != "" {
		t.Errorf("Description = %q, want empty", scene.Description)
	}
	if len(scene.Tags) != 0 {
		t.Errorf("Tags = %v, want none", scene.Tags)
	}
}

// ---- end-to-end ----

func TestListScenes(t *testing.T) {
	listing := readFixture(t, "listing.html")
	detail := readFixture(t, "detail.html")

	var detailHits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/ultimatekink/movies/newest" && r.URL.Query().Get("page") == "1":
			_, _ = w.Write(listing)
		case strings.HasPrefix(r.URL.Path, "/ultimatekink/movie/"):
			detailHits.Add(1)
			_, _ = w.Write(detail)
		default:
			_, _ = fmt.Fprint(w, `<html><body></body></html>`)
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

	if len(scenes) != 3 {
		t.Fatalf("got %d scenes, want 3", len(scenes))
	}
	if got := detailHits.Load(); got != 3 {
		t.Errorf("detail fetches = %d, want 3", got)
	}
	for _, sc := range scenes {
		if sc.SiteID != siteID || sc.Studio != studioName {
			t.Errorf("scene %s: SiteID=%q Studio=%q", sc.ID, sc.SiteID, sc.Studio)
		}
		if sc.Title == "" || sc.Date.IsZero() || sc.Duration == 0 || len(sc.Performers) == 0 {
			t.Errorf("scene %s incomplete: %+v", sc.ID, sc)
		}
		if sc.Description == "" {
			t.Errorf("scene %s has no description", sc.ID)
		}
	}
}

// Description and tags are the only detail-only fields, so a failed detail
// fetch must still yield the card-derived scene.
func TestDetailFailureIsNotFatal(t *testing.T) {
	listing := readFixture(t, "listing.html")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/ultimatekink/movies/newest" && r.URL.Query().Get("page") == "1":
			_, _ = w.Write(listing)
		case strings.HasPrefix(r.URL.Path, "/ultimatekink/movie/"):
			w.WriteHeader(http.StatusNotFound)
		default:
			_, _ = fmt.Fprint(w, `<html></html>`)
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
	if len(scenes) != 3 {
		t.Fatalf("got %d scenes, want 3", len(scenes))
	}
	for _, sc := range scenes {
		if sc.Title == "" || sc.Date.IsZero() || sc.Duration == 0 {
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
