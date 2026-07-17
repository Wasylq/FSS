package guysinsweatpants

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
		{"https://guysinsweatpants.com/", true},
		{"https://guysinsweatpants.com/tour/categories/movies_2.html", true},
		{"http://www.guysinsweatpants.com/tour/trailers/breeding-luca.html", true},
		{"https://guysinsweatpants.net/", false},
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

// Page 1 has no numeric suffix.
func TestListingURL(t *testing.T) {
	orig := siteBase
	siteBase = "https://guysinsweatpants.com"
	defer func() { siteBase = orig }()

	if got, want := listingURL(1), "https://guysinsweatpants.com/tour/categories/movies.html"; got != want {
		t.Errorf("page 1 = %q, want %q", got, want)
	}
	if got, want := listingURL(4), "https://guysinsweatpants.com/tour/categories/movies_4.html"; got != want {
		t.Errorf("page 4 = %q, want %q", got, want)
	}
}

func TestParseListing(t *testing.T) {
	items := parseListing(readFixture(t))
	if len(items) != 3 {
		t.Fatalf("got %d items, want 3", len(items))
	}

	got := items[0]
	if got.id != "565" {
		t.Errorf("id = %q, want 565", got.id)
	}
	if got.title != "Breeding Luca" {
		t.Errorf("title = %q", got.title)
	}
	if !strings.HasSuffix(got.url, "/tour/trailers/breeding-luca.html") {
		t.Errorf("url = %q", got.url)
	}
	want := time.Date(2026, time.July, 10, 0, 0, 0, 0, time.UTC)
	if !got.date.Equal(want) {
		t.Errorf("date = %v, want %v", got.date, want)
	}
	// "21:58"
	if got.duration != 1318 {
		t.Errorf("duration = %d, want 1318", got.duration)
	}
	if !slices.Equal(got.performers, []string{"Austin Wilde", "Luca Summers"}) {
		t.Errorf("performers = %v", got.performers)
	}
	if !strings.HasPrefix(got.thumb, "https://") {
		t.Errorf("thumb = %q, want an absolute URL", got.thumb)
	}

	// Newest-first ordering underpins the KnownIDs early-stop.
	for i := 1; i < len(items); i++ {
		if items[i].date.After(items[i-1].date) {
			t.Errorf("item %d is newer than item %d", i, i-1)
		}
	}
}

// The duration block also carries a photo count; only the clock value is a
// runtime.
func TestParseListingDurationNotPhotoCount(t *testing.T) {
	card := `<div class="item item-video">
<a href="https://guysinsweatpants.com/tour/trailers/x.html" title="X" class="item-thumb-link"></a>
<img id="set-target-9" class="video_placeholder stdimage" src="/tour/content/x-1x.jpg" />
<div class="item-meta-duration"> <i class="fa-solid fa-video"></i> 21:58 &nbsp;&nbsp;<i class="fa-solid fa-camera"></i> 24&nbsp; </div>
</div>`
	items := parseListing([]byte(card))
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].duration != 1318 {
		t.Errorf("duration = %d, want 1318 (21:58, not the 24 photo count)", items[0].duration)
	}
}

func TestParseListingEmpty(t *testing.T) {
	if items := parseListing([]byte(`<html><body>nothing</body></html>`)); len(items) != 0 {
		t.Errorf("got %d items, want 0", len(items))
	}
}

// ---- end-to-end ----

func TestListScenes(t *testing.T) {
	listing := readFixture(t)
	const detail = `<html><body><div class="update-info-block text-larger"><p>Meet Luca. He&#39;s a 20 year old stud.</p></div></body></html>`

	var detailHits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/tour/categories/movies.html":
			_, _ = w.Write(listing)
		case strings.HasPrefix(r.URL.Path, "/tour/trailers/"):
			detailHits.Add(1)
			_, _ = fmt.Fprint(w, detail)
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
		if strings.Contains(sc.Description, "&#") {
			t.Errorf("scene %s description holds entities: %q", sc.ID, sc.Description)
		}
	}
}

// Only the description comes from the detail page, so a failure there must
// still leave a complete scene.
func TestDetailFailureIsNotFatal(t *testing.T) {
	listing := readFixture(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/tour/categories/movies.html":
			_, _ = w.Write(listing)
		case strings.HasPrefix(r.URL.Path, "/tour/trailers/"):
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
