package nadinejansen

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
		{"https://nadine-j.de/", true},
		{"https://nadine-j.de/models/videos/2", true},
		{"https://www.nadine-j.de/nadine/videos", true},
		{"https://nadinejansen.com/", false},
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

func TestListingURL(t *testing.T) {
	orig := siteBase
	siteBase = "https://nadine-j.de"
	defer func() { siteBase = orig }()

	if got, want := listingURL("/models/videos", 1), "https://nadine-j.de/models/videos"; got != want {
		t.Errorf("page 1 = %q, want %q", got, want)
	}
	if got, want := listingURL("/models/videos", 3), "https://nadine-j.de/models/videos/3"; got != want {
		t.Errorf("page 3 = %q, want %q", got, want)
	}
}

// Every field is on the card — the detail pages are HTTP Basic protected and
// 401, so a detail-fetching scraper would get nothing.
func TestParseListing(t *testing.T) {
	items := parseListing(readFixture(t))
	if len(items) != 3 {
		t.Fatalf("got %d items, want 3", len(items))
	}

	got := items[0]
	if got.id != "1584" {
		t.Errorf("id = %q", got.id)
	}
	if got.title != "Milk Delivery Trouble" {
		t.Errorf("title = %q", got.title)
	}
	want := time.Date(2026, time.July, 19, 0, 0, 0, 0, time.UTC)
	if !got.date.Equal(want) {
		t.Errorf("date = %v, want %v", got.date, want)
	}
	// "See the 15:14 min HD clip"
	if got.duration != 914 {
		t.Errorf("duration = %d, want 914", got.duration)
	}
	// Co-starring scenes read "Sophia & Roxanne Miller".
	if !slices.Equal(got.performers, []string{"Sophia", "Roxanne Miller"}) {
		t.Errorf("performers = %v", got.performers)
	}
	if !strings.HasPrefix(got.description, "The club is opening soon") {
		t.Errorf("description = %q", got.description)
	}
	// Descriptions are wrapped in editor junk that must be stripped.
	if strings.Contains(got.description, "<") {
		t.Errorf("description still holds markup: %q", got.description)
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

// Nadine's own catalogue uses a thinner card: an <h3> title and a .date span,
// with no performers, duration or description.
func TestParseListingThinCard(t *testing.T) {
	body := []byte(`
<a href="/member/video/1222" class="item">
  <figure class="cover"><img src="/open/videos/previews/1222/nadrenovatxs.jpg" alt="video"/></figure>
  <div class="item-text">
    <span class="date">May 30, 2022</span>
    <h3 class="theme">Topless Renovating Service</h3>
  </div>
</a>`)

	items := parseListing(body)
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	got := items[0]
	if got.id != "1222" {
		t.Errorf("id = %q", got.id)
	}
	if got.title != "Topless Renovating Service" {
		t.Errorf("title = %q", got.title)
	}
	want := time.Date(2022, time.May, 30, 0, 0, 0, 0, time.UTC)
	if !got.date.Equal(want) {
		t.Errorf("date = %v, want %v", got.date, want)
	}
}

func TestParseListingEmpty(t *testing.T) {
	if items := parseListing([]byte(`<html><body>nothing</body></html>`)); len(items) != 0 {
		t.Errorf("got %d items, want 0", len(items))
	}
}

// ---- end-to-end ----

// Both catalogues must be walked: /models/videos then /nadine/videos.
func TestListScenesWalksBothListings(t *testing.T) {
	listing := readFixture(t)
	thin := []byte(`<a href="/member/video/999" class="item"><span class="date">May 30, 2022</span><h3 class="theme">Nadine Only</h3></a>`)

	var hitModels, hitNadine bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/models/videos":
			hitModels = true
			_, _ = w.Write(listing)
		case "/nadine/videos":
			hitNadine = true
			_, _ = w.Write(thin)
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

	if !hitModels || !hitNadine {
		t.Errorf("listings hit: models=%v nadine=%v — both must be walked", hitModels, hitNadine)
	}
	if len(scenes) != 4 {
		t.Fatalf("got %d scenes, want 4 (3 model + 1 nadine)", len(scenes))
	}

	ids := map[string]bool{}
	for _, sc := range scenes {
		if ids[sc.ID] {
			t.Errorf("duplicate scene %q across listings", sc.ID)
		}
		ids[sc.ID] = true
		if sc.SiteID != siteID || sc.Studio != studioName {
			t.Errorf("scene %s: SiteID=%q Studio=%q", sc.ID, sc.SiteID, sc.Studio)
		}
		if sc.Title == "" || sc.Date.IsZero() || sc.URL == "" {
			t.Errorf("scene %s incomplete: %+v", sc.ID, sc)
		}
	}
	if !ids["999"] {
		t.Error("the /nadine/videos scene is missing")
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
