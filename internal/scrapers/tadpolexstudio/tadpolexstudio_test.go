package tadpolexstudio

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
		{"https://www.tadpolexstudio.com/", true},
		{"https://tadpolexstudio.com/categories/movies_2.html", true},
		{"http://www.tadpolexstudio.com/scenes/x_vids.html", true},
		{"https://tadpolexxxstudio.com/", false},
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
	siteBase = "https://www.tadpolexstudio.com"
	defer func() { siteBase = orig }()

	if got, want := listingURL(1), "https://www.tadpolexstudio.com/categories/movies.html"; got != want {
		t.Errorf("page 1 = %q, want %q", got, want)
	}
	if got, want := listingURL(5), "https://www.tadpolexstudio.com/categories/movies_5.html"; got != want {
		t.Errorf("page 5 = %q, want %q", got, want)
	}
}

// The CMS emits `<a  href=` with a double space, so the anchor regexes must
// tolerate arbitrary whitespace — a single literal space matches nothing.
func TestParseListing(t *testing.T) {
	items := parseListing(readFixture(t))
	if len(items) != 3 {
		t.Fatalf("got %d items, want 3", len(items))
	}

	got := items[0]
	if got.id != "358" {
		t.Errorf("id = %q, want 358", got.id)
	}
	if got.title != "Tad Pole Creampies Adorable Ronnie Violet" {
		t.Errorf("title = %q", got.title)
	}
	if !strings.HasSuffix(got.url, "_vids.html") {
		t.Errorf("url = %q", got.url)
	}
	want := time.Date(2026, time.July, 21, 0, 0, 0, 0, time.UTC)
	if !got.date.Equal(want) {
		t.Errorf("date = %v, want %v", got.date, want)
	}
	// "24 min"
	if got.duration != 1440 {
		t.Errorf("duration = %d, want 1440", got.duration)
	}
	if !slices.Equal(got.performers, []string{"Ronnie Violet", "Tad Pole"}) {
		t.Errorf("performers = %v", got.performers)
	}
	if !strings.HasPrefix(got.thumb, "https://") {
		t.Errorf("thumb = %q, want an absolute URL", got.thumb)
	}
}

// The date value trails an HTML comment inside the card.
func TestParseListingDateBehindComment(t *testing.T) {
	card := `<div class="latestUpdateB" data-setid="9">
<h4 class="link_bright">
	<a  href="https://www.tadpolexstudio.com/scenes/x_vids.html">X</a>
</h4>
<li class="text_med"><span class="s_icon"><i class="fa-solid fa-calendar"></i></span><!-- Date --> 01/02/2020</li>
</div>`
	items := parseListing([]byte(card))
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	want := time.Date(2020, time.January, 2, 0, 0, 0, 0, time.UTC)
	if !items[0].date.Equal(want) {
		t.Errorf("date = %v, want %v (MM/DD/YYYY)", items[0].date, want)
	}
}

func TestParseListingEmpty(t *testing.T) {
	if items := parseListing([]byte(`<html><body>nothing</body></html>`)); len(items) != 0 {
		t.Errorf("got %d items, want 0", len(items))
	}
}

// The listing section itself is a /categories/ link, so it must not become a tag.
func TestParseTagsDropsListingCategory(t *testing.T) {
	detail := `
<a href="https://www.tadpolexstudio.com/categories/movies.html">Movies</a>
<a href="https://www.tadpolexstudio.com/categories/creampie.html">Creampie</a>
<a href="https://www.tadpolexstudio.com/categories/pov.html">POV</a>
<a href="https://www.tadpolexstudio.com/categories/creampie.html">Creampie</a>`

	got := parseTags(detail)
	if !slices.Equal(got, []string{"Creampie", "POV"}) {
		t.Errorf("parseTags = %v, want [Creampie POV]", got)
	}
}

// ---- end-to-end ----

func TestListScenes(t *testing.T) {
	listing := readFixture(t)
	const detail = `<html><body>
<a href="https://www.tadpolexstudio.com/categories/movies.html">Movies</a>
<a href="https://www.tadpolexstudio.com/categories/creampie.html">Creampie</a>
</body></html>`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/categories/movies.html":
			_, _ = w.Write(listing)
		case strings.HasPrefix(r.URL.Path, "/scenes/"):
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
	for _, sc := range scenes {
		if sc.SiteID != siteID || sc.Studio != studioName {
			t.Errorf("scene %s: SiteID=%q Studio=%q", sc.ID, sc.SiteID, sc.Studio)
		}
		if sc.Title == "" || sc.Date.IsZero() || sc.Duration == 0 || len(sc.Performers) == 0 {
			t.Errorf("scene %s incomplete: %+v", sc.ID, sc)
		}
		if !slices.Equal(sc.Tags, []string{"Creampie"}) {
			t.Errorf("scene %s tags = %v, want [Creampie]", sc.ID, sc.Tags)
		}
	}
}

func TestDetailFailureIsNotFatal(t *testing.T) {
	listing := readFixture(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/categories/movies.html":
			_, _ = w.Write(listing)
		case strings.HasPrefix(r.URL.Path, "/scenes/"):
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
