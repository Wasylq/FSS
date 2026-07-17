package littlemutt

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
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
		{"http://littlemutt.com/", true},
		{"https://www.littlemutt.com/", true},
		{"http://tour.littlemutt.com/videos/7/", true},
		{"http://tour.littlemutt.com/video/964/Euromutt_-_Ema", true},
		{"http://littlemutts.com/", false},
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

// Pagination is by row offset in steps of 7, not by page number — using page
// numbers would re-fetch the first page forever.
func TestListingURL(t *testing.T) {
	orig := siteBase
	siteBase = "http://tour.littlemutt.com"
	defer func() { siteBase = orig }()

	cases := map[int]string{
		0:   "http://tour.littlemutt.com/videos/",
		7:   "http://tour.littlemutt.com/videos/7/",
		798: "http://tour.littlemutt.com/videos/798/",
	}
	for offset, want := range cases {
		if got := listingURL(offset); got != want {
			t.Errorf("listingURL(%d) = %q, want %q", offset, got, want)
		}
	}
}

func TestParseListing(t *testing.T) {
	items := parseListing(readFixture(t))
	if len(items) != 7 {
		t.Fatalf("got %d items, want 7", len(items))
	}

	got := items[0]
	if got.id != "964" {
		t.Errorf("id = %q", got.id)
	}
	if got.title != "Euromutt - Ema Sensual Massage" {
		t.Errorf("title = %q", got.title)
	}
	want := time.Date(2023, time.December, 30, 0, 0, 0, 0, time.UTC)
	if !got.date.Equal(want) {
		t.Errorf("date = %v, want %v", got.date, want)
	}
	if !strings.HasPrefix(got.thumb, "http://") {
		t.Errorf("thumb = %q", got.thumb)
	}

	// Every card must get a distinct id — the cards share a table-soup layout
	// and a sloppy delimiter would merge them.
	seen := map[string]bool{}
	for _, it := range items {
		if seen[it.id] {
			t.Errorf("duplicate id %q", it.id)
		}
		seen[it.id] = true
	}
}

// The tour emits two date shapes: an ordinal day for newer scenes and a
// month-only bulk-import placeholder for the oldest few hundred.
func TestParseDate(t *testing.T) {
	cases := []struct {
		in   string
		want time.Time
	}{
		{"Dec 30th 2023", time.Date(2023, time.December, 30, 0, 0, 0, 0, time.UTC)},
		{"Jul 1st 2026", time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)},
		{"Mar 3rd 2019", time.Date(2019, time.March, 3, 0, 0, 0, 0, time.UTC)},
		{"Jan 22nd 2020", time.Date(2020, time.January, 22, 0, 0, 0, 0, time.UTC)},
		// Month-only placeholder -> first of the month.
		{"Jan 2020", time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC)},
		{"nonsense", time.Time{}},
	}
	for _, c := range cases {
		if got := parseDate(c.in); !got.Equal(c.want) {
			t.Errorf("parseDate(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

// The short description is usually the title repeated; only a genuinely
// different one is kept.
func TestToSceneDropsEchoedDescription(t *testing.T) {
	echoed := listItem{id: "1", title: "A Scene", description: "A Scene"}
	if got := echoed.toScene("x", time.Now()).Description; got != "" {
		t.Errorf("Description = %q, want empty when it just echoes the title", got)
	}

	real := listItem{id: "2", title: "A Scene", description: "Something else entirely"}
	if got := real.toScene("x", time.Now()).Description; got != "Something else entirely" {
		t.Errorf("Description = %q", got)
	}
}

func TestParseListingEmpty(t *testing.T) {
	if items := parseListing([]byte(`<html><body>nothing</body></html>`)); len(items) != 0 {
		t.Errorf("got %d items, want 0", len(items))
	}
}

// ---- end-to-end ----

func TestListScenesWalksOffsets(t *testing.T) {
	listing := readFixture(t)

	var offsets []int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.Trim(strings.TrimPrefix(r.URL.Path, "/videos"), "/")
		off := 0
		if p != "" {
			off, _ = strconv.Atoi(p)
		}
		offsets = append(offsets, off)
		// Serve one real page, then an empty one to end the walk.
		if off == 0 {
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

	if len(scenes) != 7 {
		t.Fatalf("got %d scenes, want 7", len(scenes))
	}
	// Offsets must step by 7, not by 1.
	if len(offsets) < 2 || offsets[0] != 0 || offsets[1] != perPage {
		t.Errorf("offsets = %v, want [0 %d ...]", offsets, perPage)
	}
	for _, sc := range scenes {
		if sc.SiteID != siteID || sc.Studio != studioName {
			t.Errorf("scene %s: SiteID=%q Studio=%q", sc.ID, sc.SiteID, sc.Studio)
		}
		if sc.Title == "" || sc.URL == "" || sc.Date.IsZero() {
			t.Errorf("scene %s incomplete: %+v", sc.ID, sc)
		}
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
