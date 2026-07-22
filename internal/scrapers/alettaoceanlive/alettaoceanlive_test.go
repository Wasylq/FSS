package alettaoceanlive

import (
	"context"
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

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("reading fixture %s: %v", name, err)
	}
	return b
}

func TestID(t *testing.T) {
	if got := New().ID(); got != siteID {
		t.Errorf("ID() = %q, want %q", got, siteID)
	}
}

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := map[string]bool{
		"https://alettaoceanlive.com":                                     true,
		"https://www.alettaoceanlive.com/tour/categories/movies_2_d.html": true,
		"https://alettaoceanlive.com.evil.test/":                          false,
		"":                                                                false,
	}
	for u, want := range cases {
		if got := s.MatchesURL(u); got != want {
			t.Errorf("MatchesURL(%q) = %v, want %v", u, got, want)
		}
	}
}

func TestListingURL(t *testing.T) {
	if got, want := listingURL("movies", 1), siteBase+"/tour/categories/movies.html"; got != want {
		t.Errorf("page 1 = %q, want %q", got, want)
	}
	if got, want := listingURL("homevideos", 3), siteBase+"/tour/categories/homevideos_3_d.html"; got != want {
		t.Errorf("page 3 = %q, want %q", got, want)
	}
}

// A card's own children are movie-set-list-item__wrapper/__content/__details.
// A looser card pattern splits every card into fragments, and the fragment the
// id is read from ends before the title and date — so both come back empty.
func TestCardBoundaryIsNotSplitByChildClasses(t *testing.T) {
	items := parseListing(readFixture(t, "listing_movies.html"))
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	for _, it := range items {
		if it.title == "" {
			t.Errorf("item %q has no title — the card was split at a child class", it.id)
		}
		if it.date.IsZero() {
			t.Errorf("item %q has no date — the card was split at a child class", it.id)
		}
	}
}

func TestParseListingMovies(t *testing.T) {
	items := parseListing(readFixture(t, "listing_movies.html"))
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	it := items[0]
	if it.id != "329-4K50p-webvideo" {
		t.Errorf("id = %q — the content directory is the only stable id", it.id)
	}
	if it.title != "New Gardener Hired" {
		t.Errorf("title = %q", it.title)
	}
	if want := time.Date(2026, time.May, 23, 0, 0, 0, 0, time.UTC); !it.date.Equal(want) {
		t.Errorf("date = %v, want %v", it.date, want)
	}
	if !strings.Contains(it.url, "/tour/trailers/") {
		t.Errorf("url = %q", it.url)
	}
	// The thumbnail is an inline background-image, not an <img>.
	if !strings.HasSuffix(it.thumb, ".jpg") {
		t.Errorf("thumb = %q", it.thumb)
	}
}

// Home-video cards link to /join rather than a detail page, so that href must
// not become the scene URL.
func TestHomeVideoCardsDoNotUseTheJoinLink(t *testing.T) {
	items := parseListing(readFixture(t, "listing_homevideos.html"))
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	for _, it := range items {
		if strings.Contains(it.url, "/join") {
			t.Errorf("item %q took the join link as its URL: %q", it.id, it.url)
		}
		if it.url != "" {
			t.Errorf("item %q should have no card URL, got %q", it.id, it.url)
		}
		if !strings.HasPrefix(it.id, "H-") {
			t.Errorf("home-video id = %q, want an H- prefix", it.id)
		}
	}
}

// Every scene needs a resolvable URL, so home videos fall back to their
// listing page.
func TestHomeVideoScenesFallBackToTheListingURL(t *testing.T) {
	s := New()
	items := parseListing(readFixture(t, "listing_homevideos.html"))
	scenes := s.toScenes("https://alettaoceanlive.com", items, time.Now())
	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2", len(scenes))
	}
	for _, sc := range scenes {
		if sc.URL != listingURL("homevideos", 1) {
			t.Errorf("scene %s: URL = %q", sc.ID, sc.URL)
		}
		if !slices.Equal(sc.Performers, []string{"Aletta Ocean"}) {
			t.Errorf("scene %s: Performers = %v", sc.ID, sc.Performers)
		}
		// The site publishes no runtime anywhere.
		if sc.Duration != 0 {
			t.Errorf("scene %s: Duration = %d, want 0", sc.ID, sc.Duration)
		}
	}
}

// ---- end-to-end ----

// The catalogue is split across two categories, so the run must move on to
// homevideos once movies is exhausted rather than stopping.
func TestListScenesWalksBothCategories(t *testing.T) {
	movies := readFixture(t, "listing_movies.html")
	home := readFixture(t, "listing_homevideos.html")
	var moviePages, homePages atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "homevideos"):
			if homePages.Add(1) == 1 {
				_, _ = w.Write(home)
				return
			}
		case strings.Contains(r.URL.Path, "movies"):
			if moviePages.Add(1) == 1 {
				_, _ = w.Write(movies)
				return
			}
		}
		_, _ = w.Write([]byte("<html><body>no cards</body></html>"))
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

	if len(scenes) != 4 {
		t.Fatalf("got %d scenes, want 4 (2 movies + 2 home videos)", len(scenes))
	}
	if moviePages.Load() == 0 || homePages.Load() == 0 {
		t.Errorf("both categories must be walked: movies=%d homevideos=%d", moviePages.Load(), homePages.Load())
	}
	sawHome := false
	for _, sc := range scenes {
		if strings.HasPrefix(sc.ID, "H-") {
			sawHome = true
		}
		if sc.SiteID != siteID || sc.Studio != studioName || sc.Title == "" {
			t.Errorf("scene %s incomplete: %+v", sc.ID, sc)
		}
	}
	if !sawHome {
		t.Error("the home-video category never produced a scene")
	}
}

func TestContextCancellation(t *testing.T) {
	movies := readFixture(t, "listing_movies.html")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(movies)
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

func TestListingErrorIsReported(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
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
	sawErr := false
	for res := range ch {
		if res.Kind == scraper.KindError {
			sawErr = true
		}
	}
	if !sawErr {
		t.Error("a listing failure produced no error result")
	}
}
