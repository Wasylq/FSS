package jakecruise

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
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
	ids, domains := map[string]bool{}, map[string]bool{}
	for _, cfg := range sites {
		if ids[cfg.SiteID] {
			t.Errorf("duplicate SiteID %q", cfg.SiteID)
		}
		if domains[cfg.Domain] {
			t.Errorf("duplicate domain %q", cfg.Domain)
		}
		ids[cfg.SiteID], domains[cfg.Domain] = true, true
		if cfg.StudioName == "" {
			t.Errorf("%s: StudioName is empty", cfg.SiteID)
		}
	}
}

func TestMatchesURL(t *testing.T) {
	s := newScraper(siteByID(t, "cocksuremen"))
	cases := map[string]bool{
		"https://www.cocksuremen.com":                                true,
		"https://cocksuremen.com/tour/categories/movies/2/latest/":   true,
		"http://www.cocksuremen.com/tour/models/james-hamilton.html": true,
		"https://www.jakecruise.com":                                 false,
		"https://cocksuremen.com.evil.test/":                         false,
		"":                                                           false,
	}
	for u, want := range cases {
		if got := s.MatchesURL(u); got != want {
			t.Errorf("MatchesURL(%q) = %v, want %v", u, got, want)
		}
	}
}

// The nonce in set-target-{setid}-{nonce} is regenerated on every request, so
// keying on the pair would make each incremental run see the whole catalogue as
// new and each --full run duplicate it.
func TestSceneIDIgnoresThePerRequestNonce(t *testing.T) {
	s := newScraper(siteByID(t, "cocksuremen"))
	fixture := string(readFixture(t, "listing.html"))

	nonceRe := regexp.MustCompile(`(set-target-\d+-)\d+`)
	first := s.parseListing([]byte(nonceRe.ReplaceAllString(fixture, "${1}493727")))
	second := s.parseListing([]byte(nonceRe.ReplaceAllString(fixture, "${1}146963")))

	if len(first) == 0 {
		t.Fatal("no items parsed")
	}
	for i := range first {
		if first[i].id != second[i].id {
			t.Errorf("id changed with the nonce: %q vs %q", first[i].id, second[i].id)
		}
	}
	if first[0].id != "243" {
		t.Errorf("id = %q, want the bare set id 243", first[0].id)
	}
}

func TestParseListing(t *testing.T) {
	s := newScraper(siteByID(t, "cocksuremen"))
	items := s.parseListing(readFixture(t, "listing.html"))
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}

	it := items[0]
	if it.title != "James Hamilton Barebacks Trevor Spade" {
		t.Errorf("title = %q", it.title)
	}
	// Cards link protocol-relative, so the URL must be rebuilt against the base.
	want := "https://www.cocksuremen.com/tour/trailers/james-hamilton-barebacks-trevor-spade-in-Raw-Gay-Porno.html"
	if it.url != want {
		t.Errorf("url = %q, want %q", it.url, want)
	}
	if !slices.Equal(it.performers, []string{"James Hamilton", "Trevor Spade"}) {
		t.Errorf("performers = %v", it.performers)
	}
	if it.duration != 23*60+19 {
		t.Errorf("duration = %d, want %d", it.duration, 23*60+19)
	}
	if want := time.Date(2026, time.July, 16, 0, 0, 0, 0, time.UTC); !it.date.Equal(want) {
		t.Errorf("date = %v, want %v", it.date, want)
	}
	if !strings.HasPrefix(it.thumb, "https://www.cocksuremen.com/tour/content") {
		t.Errorf("thumb = %q — card thumbs are relative and must be absolutised", it.thumb)
	}
}

// Runtime and date share one <p class="timing">. Reading them document-wide
// instead picks up thumbnail paths like content//contentthumbs/74/07/7407.jpg.
func TestTimingIsScopedToItsOwnElement(t *testing.T) {
	s := newScraper(siteByID(t, "cocksuremen"))
	card := `<div class="sexycock ">
	<img id="set-target-9-111" class="update_thumb thumbs stdimage" src="content//contentthumbs/74/07/7407.jpg" />
	<a href='//www.cocksuremen.com/tour/trailers/x.html' title="X">
	<p class="timing"> 12:34 <br/>01/02/2026 </p>
</div>`
	items := s.parseListing([]byte(card))
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].duration != 12*60+34 {
		t.Errorf("duration = %d, want %d", items[0].duration, 12*60+34)
	}
	// US-format: 01/02/2026 is 2 January, not 1 February.
	if want := time.Date(2026, time.January, 2, 0, 0, 0, 0, time.UTC); !items[0].date.Equal(want) {
		t.Errorf("date = %v, want %v", items[0].date, want)
	}
}

func TestNormalizeURL(t *testing.T) {
	s := newScraper(siteByID(t, "cocksuremen"))
	cases := map[string]string{
		"//cdn.example.com/a.jpg": "https://cdn.example.com/a.jpg",
		"/tour/content/a.jpg":     "https://www.cocksuremen.com/tour/content/a.jpg",
		"https://x/a.jpg":         "https://x/a.jpg",
		"content//thumbs/a.jpg":   "https://www.cocksuremen.com/tour/content//thumbs/a.jpg",
	}
	for in, want := range cases {
		if got := s.normalizeURL(in); got != want {
			t.Errorf("normalizeURL(%q) = %q, want %q", in, got, want)
		}
	}
}

// ---- end-to-end ----

const detailHTML = `<html><body>
<h3>James Hamilton Barebacks Trevor Spade</h3>
<div class="aboutvideo"><p>Hung stud James Hamilton and sexy Trevor Spade are moving into their new place.</p></div>
</body></html>`

func newSiteServer(t *testing.T) (*httptest.Server, func() int) {
	t.Helper()
	listing := readFixture(t, "listing.html")
	var listPages atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/tour/trailers/") {
			_, _ = w.Write([]byte(detailHTML))
			return
		}
		// Only the first listing page has cards; the second ends the run.
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
	s := newScraper(siteByID(t, "cocksuremen"))
	s.Client = srv.Client()
	s.base = srv.URL
	return s
}

func TestListScenes(t *testing.T) {
	srv, _ := newSiteServer(t)
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
		if sc.SiteID != "cocksuremen" || sc.Studio != "CocksureMen" {
			t.Errorf("scene %s: SiteID=%q Studio=%q", sc.ID, sc.SiteID, sc.Studio)
		}
		if sc.Title == "" || sc.Date.IsZero() || sc.Duration == 0 {
			t.Errorf("scene %s is incomplete: %+v", sc.ID, sc)
		}
		if !strings.HasPrefix(sc.Description, "Hung stud James Hamilton") {
			t.Errorf("scene %s description = %q", sc.ID, sc.Description)
		}
	}
}

// The detail page carries only the description, so a failure there must still
// leave a usable scene rather than dropping it.
func TestDetailFailureKeepsTheScene(t *testing.T) {
	listing := readFixture(t, "listing.html")
	var listPages atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/tour/trailers/") {
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
		t.Fatalf("got %d scenes, want 2 — a failed detail fetch must not drop the card", len(scenes))
	}
	if scenes[0].Description != "" {
		t.Errorf("description = %q, want empty", scenes[0].Description)
	}
	if scenes[0].Title == "" {
		t.Error("the card's own fields must survive a detail failure")
	}
}

func TestPaginationStopsOnEmptyPage(t *testing.T) {
	srv, listPages := newSiteServer(t)
	s := newTestScraper(t, srv)

	ch, err := s.ListScenes(context.Background(), srv.URL, scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}
	testutil.CollectScenes(t, ch)

	if got := listPages(); got != 2 {
		t.Errorf("fetched %d listing pages, want 2 (one with cards, one empty)", got)
	}
}

func TestContextCancellation(t *testing.T) {
	listing := readFixture(t, "listing.html")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/tour/trailers/") {
			_, _ = w.Write([]byte(detailHTML))
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

// Hot Dads Hot Lads renders two timing elements per card: the first carries the
// runtime alone, the second the runtime and the date. Reading only the first
// match leaves every scene dateless.
func TestDateFoundInASecondTimingElement(t *testing.T) {
	s := newScraper(siteByID(t, "hotdadshotlads"))
	card := `<div class="sexycock ">
	<img id="set-target-42-491345" class="update_thumb thumbs stdimage" src="content//x.jpg" />
	<a href='//www.hotdadshotlads.com/tour/trailers/y.html' title="Y">
	<p class="timing">20:11</p>
	<p class="timing"> 20:11 <br/>07/16/2026 </p>
</div>`
	items := s.parseListing([]byte(card))
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].duration != 20*60+11 {
		t.Errorf("duration = %d, want %d", items[0].duration, 20*60+11)
	}
	if want := time.Date(2026, time.July, 16, 0, 0, 0, 0, time.UTC); !items[0].date.Equal(want) {
		t.Errorf("date = %v, want %v", items[0].date, want)
	}
}
