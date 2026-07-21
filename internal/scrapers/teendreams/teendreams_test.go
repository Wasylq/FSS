package teendreams

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

func siteByID(t *testing.T, id string) siteConfig {
	t.Helper()
	for _, cfg := range sites {
		if cfg.SiteID == id {
			return cfg
		}
	}
	t.Fatalf("unknown site %q", id)
	return siteConfig{}
}

func TestUniqueSiteIDsAndDomains(t *testing.T) {
	seen := map[string]bool{}
	for _, cfg := range sites {
		if seen[cfg.SiteID] {
			t.Errorf("duplicate SiteID %q", cfg.SiteID)
		}
		seen[cfg.SiteID] = true
		for _, d := range append([]string{cfg.Domain}, cfg.Aliases...) {
			if seen[d] {
				t.Errorf("domain %q claimed twice", d)
			}
			seen[d] = true
		}
	}
}

// teen-depot.com serves the identical catalogue and its cards link to
// teendreams.com, so it is an alias rather than a second site — registering it
// separately would ingest every scene twice under two SiteIDs.
func TestTeenDepotIsAnAliasNotASite(t *testing.T) {
	for _, cfg := range sites {
		if cfg.Domain == "teen-depot.com" {
			t.Fatal("teen-depot.com must not be a site of its own")
		}
	}
	s := newScraper(siteByID(t, "teendreams"))
	for _, u := range []string{
		"https://www.teen-depot.com/",
		"https://teen-depot.com/t4/categories/movies_1_d.html",
		"https://www.teendreams.com/",
	} {
		if !s.MatchesURL(u) {
			t.Errorf("MatchesURL(%q) = false, want true", u)
		}
	}
	for _, u := range []string{"https://www.lesarchive.com/", "https://teendreams.com.evil.test/", ""} {
		if s.MatchesURL(u) {
			t.Errorf("MatchesURL(%q) = true, want false", u)
		}
	}
}

func TestParseListing(t *testing.T) {
	s := newScraper(siteByID(t, "teendreams"))
	items := s.parseListing(readFixture(t, "listing.html"))
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}

	it := items[0]
	if it.id != "23130" {
		t.Errorf("id = %q", it.id)
	}
	if it.title != "Layla Scarlett Pretty in Pink" {
		t.Errorf("title = %q", it.title)
	}
	if want := "https://www.teendreams.com/t4/trailers/Layla-Scarlett-Pink-Video.html"; it.url != want {
		t.Errorf("url = %q, want %q", it.url, want)
	}
	// The card's own src is a 1x1 placeholder — taking it would give every
	// scene the same useless thumbnail.
	if strings.Contains(it.thumb, "1x1.jpg") {
		t.Errorf("thumb = %q, want the src0_1x image not the placeholder", it.thumb)
	}
	if !strings.HasSuffix(it.thumb, "86290-1x.jpg") {
		t.Errorf("thumb = %q", it.thumb)
	}
}

// Cards on the mirror domain link to the canonical host, so the URL is rebuilt
// against the scraper's own base rather than taken verbatim.
func TestListingURLIsRebuiltAgainstTheBase(t *testing.T) {
	s := newScraper(siteByID(t, "teendreams"))
	s.base = "https://mirror.test"
	card := `<div class="content-item">
	<a href="https://www.teendreams.com/t4/trailers/X.html" title="X" class="inner">
	<img id="set-target-1" class="mainThumb thumbs stdimage" src="custom_assets/images/1x1.jpg" src0_1x="/t4/content/1-1x.jpg" />
	</a></div>`
	items := s.parseListing([]byte(card))
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if want := "https://mirror.test/t4/trailers/X.html"; items[0].url != want {
		t.Errorf("url = %q, want %q", items[0].url, want)
	}
}

func TestApplyDetail(t *testing.T) {
	var sc models.Scene
	sc.Title = "Layla Scarlett Pretty in Pink"
	applyDetail(&sc, string(readFixture(t, "detail.html")))

	if want := time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC); !sc.Date.Equal(want) {
		t.Errorf("Date = %v, want %v", sc.Date, want)
	}
	// The player renders "0:00 / 20:28"; the first value is the playhead.
	if sc.Duration != 20*60+28 {
		t.Errorf("Duration = %d, want %d", sc.Duration, 20*60+28)
	}
	if !slices.Equal(sc.Performers, []string{"Layla Scarlett"}) {
		t.Errorf("Performers = %v", sc.Performers)
	}
	// <p class="description"> is the model's bio, not a scene synopsis, so the
	// site publishes no description this scraper can honestly use.
	if sc.Description != "" {
		t.Errorf("Description = %q — the model bio must not be used as one", sc.Description)
	}
}

// The player's leading "0:00" is the playhead. Taking the first time on the
// page would give every scene a zero runtime.
func TestDurationSkipsThePlayhead(t *testing.T) {
	var sc models.Scene
	applyDetail(&sc, `<div class="player-time"><span>0:00 / </span>20:28</div>`)
	if sc.Duration != 20*60+28 {
		t.Errorf("Duration = %d, want %d", sc.Duration, 20*60+28)
	}
}

// LesArchive files each set under a "model" page named after the set itself, so
// a derived name that merely restates the title is the set, not a performer.
func TestSetFiledUnderItselfIsNotAPerformer(t *testing.T) {
	sc := models.Scene{Title: "8 Girl Orgy"}
	applyDetail(&sc, `<a href="https://www.lesarchive.com/t4/models/8-girl-orgy.html" class="view-btn">View Model Profile</a>`)
	if len(sc.Performers) != 0 {
		t.Errorf("Performers = %v, want none", sc.Performers)
	}

	// A real model name still comes through.
	sc = models.Scene{Title: "Pretty in Pink"}
	applyDetail(&sc, `<a href="https://x/t4/models/LaylaScarlett.html" class="view-btn">View Model Profile</a>`)
	if !slices.Equal(sc.Performers, []string{"Layla Scarlett"}) {
		t.Errorf("Performers = %v", sc.Performers)
	}
}

func TestSplitCamel(t *testing.T) {
	cases := map[string]string{
		"LaylaScarlett": "Layla Scarlett",
		"8-girl-orgy":   "8 girl orgy",
		"Anastasia":     "Anastasia",
		"Mary_Jane":     "Mary Jane",
		"":              "",
	}
	for in, want := range cases {
		if got := splitCamel(in); got != want {
			t.Errorf("splitCamel(%q) = %q, want %q", in, got, want)
		}
	}
}

// ---- end-to-end ----

func newSiteServer(t *testing.T) (*httptest.Server, func() int) {
	t.Helper()
	listing := readFixture(t, "listing.html")
	detail := readFixture(t, "detail.html")
	var listPages atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/t4/trailers/") {
			_, _ = w.Write(detail)
			return
		}
		if listPages.Add(1) == 1 {
			_, _ = w.Write(listing)
			return
		}
		_, _ = w.Write([]byte("<html><body>no cards</body></html>"))
	}))
	t.Cleanup(srv.Close)
	return srv, func() int { return int(listPages.Load()) }
}

func newTestScraper(t *testing.T, srv *httptest.Server) *Scraper {
	t.Helper()
	s := newScraper(siteByID(t, "teendreams"))
	s.Client = srv.Client()
	s.base = srv.URL
	return s
}

func TestListScenes(t *testing.T) {
	srv, listPages := newSiteServer(t)
	s := newTestScraper(t, srv)

	ch, err := s.ListScenes(context.Background(), srv.URL, scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}
	scenes := testutil.CollectScenes(t, ch)
	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2", len(scenes))
	}
	for _, sc := range scenes {
		if sc.SiteID != "teendreams" || sc.Studio != "Teen Dreams" {
			t.Errorf("scene %s: SiteID=%q Studio=%q", sc.ID, sc.SiteID, sc.Studio)
		}
		if sc.Title == "" || sc.Date.IsZero() {
			t.Errorf("scene %s incomplete: %+v", sc.ID, sc)
		}
	}
	if got := listPages(); got != 2 {
		t.Errorf("fetched %d listing pages, want 2 (one with cards, one empty)", got)
	}
}

// The card carries no date or duration of its own, but a failed detail fetch
// should still leave the title and URL rather than dropping the scene.
func TestDetailFailureKeepsTheCard(t *testing.T) {
	listing := readFixture(t, "listing.html")
	var listPages atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/t4/trailers/") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if listPages.Add(1) == 1 {
			_, _ = w.Write(listing)
			return
		}
		_, _ = w.Write([]byte("<html><body>no cards</body></html>"))
	}))
	defer srv.Close()

	s := newTestScraper(t, srv)
	ch, err := s.ListScenes(context.Background(), srv.URL, scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}
	scenes := testutil.CollectScenes(t, ch)
	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2", len(scenes))
	}
	if scenes[0].Title == "" || scenes[0].URL == "" {
		t.Errorf("card fields lost on detail failure: %+v", scenes[0])
	}
	if !scenes[0].Date.IsZero() {
		t.Errorf("Date = %v, want zero", scenes[0].Date)
	}
}

func TestContextCancellation(t *testing.T) {
	listing := readFixture(t, "listing.html")
	detail := readFixture(t, "detail.html")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/t4/trailers/") {
			_, _ = w.Write(detail)
			return
		}
		_, _ = w.Write(listing)
	}))
	defer srv.Close()

	s := newTestScraper(t, srv)
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

	s := newTestScraper(t, srv)
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
