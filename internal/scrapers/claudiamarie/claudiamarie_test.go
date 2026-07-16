package claudiamarie

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
	"github.com/Wasylq/FSS/scraper"
)

func readFixture(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/listing.html")
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	return b
}

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url   string
		match bool
	}{
		{"https://claudiamarie.com/", true},
		{"https://www.claudiamarie.com/tour/updates/page_2.html", true},
		{"http://claudiamarie.com/tour/", true},
		{"https://claudiamarie.net/", false},
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

func TestParseListing(t *testing.T) {
	items := parseListing(readFixture(t))
	if len(items) != 3 {
		t.Fatalf("got %d items, want 3", len(items))
	}

	got := items[0]
	// The content slug is the only stable id — there is no numeric one.
	if got.id != "040726bigtitsummer6" {
		t.Errorf("id = %q", got.id)
	}
	if got.title != "Big Tit Summer 6 Anal Justice" {
		t.Errorf("title = %q", got.title)
	}
	// "21 minute(s) of video"
	if got.duration != 1260 {
		t.Errorf("duration = %d, want 1260", got.duration)
	}
	want := time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC)
	if !got.date.Equal(want) {
		t.Errorf("date = %v, want %v", got.date, want)
	}
	if !slices.Equal(got.performers, []string{"Claudia Marie", "Justice"}) {
		t.Errorf("performers = %v", got.performers)
	}
	if len(got.tags) != 4 {
		t.Errorf("tags = %v, want 4", got.tags)
	}
	if !strings.HasPrefix(got.description, "Mr. Marie has left for the day") {
		t.Errorf("description = %q", got.description)
	}
	// The full description comes from the anchor's title attribute, so it is
	// not the truncated "…" version.
	if strings.HasSuffix(got.description, "...") {
		t.Errorf("description looks truncated: %q", got.description)
	}
	if !strings.HasPrefix(got.thumb, "https://") {
		t.Errorf("thumb = %q", got.thumb)
	}
}

// The slug encodes a date that is NOT the release date — "040726…" is April
// while the scene published 07/15/2026. Only update_date may be trusted.
func TestParseListingIgnoresSlugDate(t *testing.T) {
	items := parseListing(readFixture(t))
	if len(items) == 0 {
		t.Fatal("no items")
	}
	got := items[0]
	if !strings.HasPrefix(got.id, "040726") {
		t.Skip("fixture slug no longer carries a conflicting date")
	}
	if got.date.Month() == time.April {
		t.Errorf("date = %v; the slug date must not be used", got.date)
	}
}

// Each page renders a "coming soon" sidebar whose entries reuse the
// update_date class with future dates. Reading fields page-wide instead of
// per-block would pull those in.
func TestParseListingExcludesSidebar(t *testing.T) {
	body := readFixture(t)
	if !strings.Contains(string(body), `<div class="sidebar">`) {
		t.Skip("fixture has no sidebar to exclude")
	}

	items := parseListing(body)
	if len(items) != 3 {
		t.Fatalf("got %d items, want 3 — the sidebar must not become a scene", len(items))
	}
	for _, it := range items {
		if it.date.After(time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)) {
			t.Errorf("item %s has a sidebar future date %v", it.id, it.date)
		}
	}
}

func TestParseListingEmpty(t *testing.T) {
	if items := parseListing([]byte(`<html><body>nothing</body></html>`)); len(items) != 0 {
		t.Errorf("got %d items, want 0", len(items))
	}
}

// There are no per-scene pages, so the URL is synthesised and must still be
// stable and unique per scene.
func TestToSceneSynthesisesURL(t *testing.T) {
	items := parseListing(readFixture(t))
	seen := map[string]bool{}
	for _, it := range items {
		sc := it.toScene("https://claudiamarie.com", time.Now())
		if sc.URL == "" {
			t.Errorf("scene %s has no URL", sc.ID)
		}
		if seen[sc.URL] {
			t.Errorf("duplicate synthesised URL %q", sc.URL)
		}
		seen[sc.URL] = true
		if !strings.Contains(sc.URL, it.id) {
			t.Errorf("URL %q does not carry the scene id %q", sc.URL, it.id)
		}
	}
}

// ---- end-to-end ----

func TestListScenes(t *testing.T) {
	listing := readFixture(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/tour/updates/page_1.html" {
			_, _ = w.Write(listing)
			return
		}
		// Pages past the end return 200 with an empty template.
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
		if sc.Title == "" || sc.Date.IsZero() || sc.Duration == 0 {
			t.Errorf("scene %s incomplete: %+v", sc.ID, sc)
		}
		if len(sc.Performers) == 0 {
			t.Errorf("scene %s has no performers", sc.ID)
		}
	}
}

// An empty page past the end must stop the walk rather than error — the site
// answers 200 with a bare template rather than a 404.
func TestListScenesStopsOnEmptyPage(t *testing.T) {
	listing := readFixture(t)
	var pages int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pages++
		if r.URL.Path == "/tour/updates/page_1.html" {
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
	testutil.CollectScenes(t, ch)

	if pages != 2 {
		t.Errorf("fetched %d pages, want 2 (one with cards, one empty)", pages)
	}
}

func TestContextCancellation(t *testing.T) {
	listing := readFixture(t)
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
