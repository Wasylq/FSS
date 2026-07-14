package girlsrimming

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
		{"https://girlsrimming.com", true},
		{"https://girlsrimming.com/tour/", true},
		{"https://www.girlsrimming.com/tour/categories/movies/2/latest/", true},
		{"http://girlsrimming.com/tour/trailers/Gatitas-Latin-Rimming-Passion.html", true},
		{"https://auntjudys.com/", false},
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
	items := parseListing(readFixture(t, "listing.html"))
	if len(items) != 3 {
		t.Fatalf("got %d items, want 3", len(items))
	}

	got := items[0]
	if got.id != "585" {
		t.Errorf("id = %q, want 585", got.id)
	}
	if !strings.HasSuffix(got.url, "/tour/trailers/Gatitas-Latin-Rimming-Passion.html") {
		t.Errorf("url = %q", got.url)
	}
	// The first trailer anchor wraps the thumbnail and has no text; the title
	// comes from the second.
	if got.title != "Gatita's Latin Rimming Passion" {
		t.Errorf("title = %q", got.title)
	}
	// "35&nbsp;min&nbsp;of video"
	if got.duration != 2100 {
		t.Errorf("duration = %d, want 2100", got.duration)
	}
	// US-format date behind an HTML comment.
	want := time.Date(2026, time.July, 11, 0, 0, 0, 0, time.UTC)
	if !got.date.Equal(want) {
		t.Errorf("date = %v, want %v", got.date, want)
	}
	if !slices.Equal(got.performers, []string{"Gatita Veve", "Renato"}) {
		t.Errorf("performers = %v", got.performers)
	}
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

// The photo count sits next to the runtime in the same div
// ("73&nbsp;Photos, 35&nbsp;min&nbsp;of video"); only the minutes are a duration.
func TestParseListingDurationIgnoresPhotoCount(t *testing.T) {
	card := `class="update_details" data-setid="1"
<a href="https://girlsrimming.com/tour/trailers/X.html"></a>
<a href="https://girlsrimming.com/tour/trailers/X.html">X</a>
<div class="update_counts"> 73&nbsp;Photos, 35&nbsp;min&nbsp;of video </div>`

	items := parseListing([]byte(card))
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].duration != 2100 {
		t.Errorf("duration = %d, want 2100 (35 min, not the 73 photo count)", items[0].duration)
	}
}

// The date cell contains an HTML comment before the value.
func TestParseListingDateBehindComment(t *testing.T) {
	card := `class="update_details" data-setid="9"
<a href="https://girlsrimming.com/tour/trailers/Y.html">Y</a>
<div class="cell update_date"> <!-- Date --> 01/02/2020 </div>`

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

// ---- detail ----

// The keywords meta mixes genre tags with performer names; the names must not
// end up duplicated as tags.
func TestApplyDetailSeparatesTagsFromPerformers(t *testing.T) {
	scene := models.Scene{Performers: []string{"Gatita Veve", "Renato"}}
	applyDetail(&scene, readFixture(t, "detail.html"))

	want := []string{"Big Boobs", "Blondes", "Busty", "Latina", "Rimming"}
	if !slices.Equal(scene.Tags, want) {
		t.Errorf("Tags = %v, want %v", scene.Tags, want)
	}
	for _, p := range scene.Performers {
		if slices.Contains(scene.Tags, p) {
			t.Errorf("performer %q leaked into tags", p)
		}
	}
	if scene.Description == "" {
		t.Error("Description is empty")
	}
}

func TestApplyDetailPerformerMatchIsCaseInsensitive(t *testing.T) {
	scene := models.Scene{Performers: []string{"gatita veve"}}
	applyDetail(&scene, []byte(`<meta name="keywords" content="Rimming,Gatita Veve" />`))

	if slices.Contains(scene.Tags, "Gatita Veve") {
		t.Errorf("Tags = %v; performer match must ignore case", scene.Tags)
	}
	if !slices.Equal(scene.Tags, []string{"Rimming"}) {
		t.Errorf("Tags = %v, want [Rimming]", scene.Tags)
	}
}

func TestApplyDetailNoKeywords(t *testing.T) {
	scene := models.Scene{}
	applyDetail(&scene, []byte(`<meta property="og:description" content="Just a description." />`))

	if scene.Description != "Just a description." {
		t.Errorf("Description = %q", scene.Description)
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
		case r.URL.Path == "/tour/categories/movies/1/latest/":
			_, _ = w.Write(listing)
		case strings.HasPrefix(r.URL.Path, "/tour/trailers/"):
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
		if sc.Title == "" || sc.Date.IsZero() || sc.Duration == 0 {
			t.Errorf("scene %s is missing listing data: %+v", sc.ID, sc)
		}
		if len(sc.Performers) == 0 {
			t.Errorf("scene %s has no performers", sc.ID)
		}
	}
}

func TestDetailFailureIsNotFatal(t *testing.T) {
	listing := readFixture(t, "listing.html")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/tour/categories/movies/1/latest/":
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
