package tribdolls

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
	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/scraper"
)

// navLinkRe rewrites the fixture's pagination nav in tests.
var navLinkRe = regexp.MustCompile(`/all-trib-dolls-videos/\d+/`)

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

// tribdolls.com redirects to the hyphenated host but serves a mismatched
// certificate, so both spellings resolve here while requests go to the
// hyphenated one.
func TestMatchesURL(t *testing.T) {
	s := New()
	cases := map[string]bool{
		"https://www.trib-dolls.com":                          true,
		"https://trib-dolls.com/all-trib-dolls-videos/2/":     true,
		"https://tribdolls.com/":                              true,
		"https://www.tribdolls.com/movies/jane-vs-lena-2248/": true,
		"https://trib-dolls.com.evil.test/":                   false,
		"":                                                    false,
	}
	for u, want := range cases {
		if got := s.MatchesURL(u); got != want {
			t.Errorf("MatchesURL(%q) = %v, want %v", u, got, want)
		}
	}
}

// The last page number is printed in the page-1 nav, so the end is read rather
// than found by overshooting.
func TestParseMaxPage(t *testing.T) {
	if got := parseMaxPage(readFixture(t, "listing.html")); got != 75 {
		t.Errorf("parseMaxPage = %d, want 75", got)
	}
	if got := parseMaxPage([]byte("<html>no nav</html>")); got != 0 {
		t.Errorf("parseMaxPage with no nav = %d, want 0", got)
	}
}

func TestParseListing(t *testing.T) {
	today := time.Date(2026, time.July, 22, 0, 0, 0, 0, time.UTC)
	items := parseListing(readFixture(t, "listing.html"), today)
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}

	it := items[0]
	if it.id != "2248" {
		t.Errorf("id = %q — the id is the trailing slug segment", it.id)
	}
	if it.title != "Jane vs. Lena" {
		t.Errorf("title = %q", it.title)
	}
	if want := siteBase + "/movies/jane-vs-lena-2248/"; it.url != want {
		t.Errorf("url = %q, want %q", it.url, want)
	}
	// "TD2248 / 25:04" — the studio code and runtime share one span.
	if it.code != "TD2248" {
		t.Errorf("code = %q", it.code)
	}
	if it.duration != 25*60+4 {
		t.Errorf("duration = %d, want %d", it.duration, 25*60+4)
	}
	if !strings.HasPrefix(it.thumb, siteBase+"/data/movies/") {
		t.Errorf("thumb = %q", it.thumb)
	}
	// "44 days ago" against the fixed reference day.
	if want := today.AddDate(0, 0, -44); !it.date.Equal(want) {
		t.Errorf("date = %v, want %v", it.date, want)
	}
}

// The site publishes no absolute date anywhere, only "N days ago" — always in
// whole days, even 5000+ of them. Resolving that against the scrape day is
// exact and stable: a run a day later sees N incremented by one.
func TestRelativeDateIsStableAcrossDays(t *testing.T) {
	card := func(n string) []byte {
		return []byte(`<div class="card">
		<a href="/movies/x-1/" title="X"></a>
		<div class="float-left"><span>` + n + ` days ago</span></div>
		</div>`)
	}

	day1 := time.Date(2026, time.July, 22, 0, 0, 0, 0, time.UTC)
	day2 := day1.AddDate(0, 0, 1)

	first := parseListing(card("44"), day1)
	second := parseListing(card("45"), day2)
	if len(first) != 1 || len(second) != 1 {
		t.Fatal("expected one item from each parse")
	}
	if !first[0].date.Equal(second[0].date) {
		t.Errorf("date drifted between scrapes: %v vs %v", first[0].date, second[0].date)
	}

	// Deep-archive ages are still whole days, so the same arithmetic holds.
	old := parseListing(card("5203"), day1)
	if want := day1.AddDate(0, 0, -5203); !old[0].date.Equal(want) {
		t.Errorf("date = %v, want %v", old[0].date, want)
	}
}

func TestApplyDetail(t *testing.T) {
	var sc models.Scene
	applyDetail(&sc, string(readFixture(t, "detail.html")))

	if len(sc.Categories) == 0 {
		t.Fatalf("Categories = %v", sc.Categories)
	}
	if !slices.Contains(sc.Categories, "Kissing") {
		t.Errorf("Categories = %v", sc.Categories)
	}
	if len(sc.Performers) == 0 {
		t.Errorf("Performers = %v", sc.Performers)
	}
}

func TestApplyDetailDedupes(t *testing.T) {
	var sc models.Scene
	applyDetail(&sc, `
	<div class="categories">
	1 days ago |
	<a href="/category/trib-1/">Trib</a> | <a href="/category/trib-1/">Trib</a> | <a href="/category/nude-4/">Nude</a>
	</div>
	<a href="/girls/jane-44/" title="Jane"></a>
	<a href="/girls/jane-44/" title="Jane"></a>`)
	if !slices.Equal(sc.Categories, []string{"Trib", "Nude"}) {
		t.Errorf("Categories = %v", sc.Categories)
	}
	if !slices.Equal(sc.Performers, []string{"Jane"}) {
		t.Errorf("Performers = %v", sc.Performers)
	}
}

// ---- end-to-end ----

func newSiteServer(t *testing.T) (*httptest.Server, func() int) {
	t.Helper()
	listing := readFixture(t, "listing.html")
	detail := readFixture(t, "detail.html")
	var listPages atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/movies/") {
			_, _ = w.Write(detail)
			return
		}
		listPages.Add(1)
		_, _ = w.Write(listing)
	}))
	t.Cleanup(srv.Close)

	orig := siteBase
	siteBase = srv.URL
	t.Cleanup(func() { siteBase = orig })

	return srv, func() int { return int(listPages.Load()) }
}

func TestListScenes(t *testing.T) {
	srv, _ := newSiteServer(t)
	s := New()
	s.Client = srv.Client()

	ch, err := s.ListScenes(context.Background(), srv.URL, scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}
	scenes := testutil.CollectScenes(t, ch)

	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2", len(scenes))
	}
	for _, sc := range scenes {
		if sc.SiteID != siteID || sc.Studio != studioName {
			t.Errorf("scene %s: SiteID=%q Studio=%q", sc.ID, sc.SiteID, sc.Studio)
		}
		if sc.Title == "" || sc.Date.IsZero() || sc.Duration == 0 {
			t.Errorf("scene %s incomplete: %+v", sc.ID, sc)
		}
		// The studio's own catalogue number.
		if !strings.HasPrefix(sc.Series, "TD") {
			t.Errorf("scene %s: Series = %q", sc.ID, sc.Series)
		}
		// The meta description is site-wide boilerplate, so none is recorded.
		if sc.Description != "" {
			t.Errorf("scene %s: Description = %q, want empty", sc.ID, sc.Description)
		}
	}
}

// The server here always serves page 1's cards. Without the nav's max-page
// value the dedup would still stop the run, but the nav is what ends it
// cleanly on the real site.
func TestPaginationStopsAtTheNavMaxPage(t *testing.T) {
	listing := readFixture(t, "listing.html")
	detail := readFixture(t, "detail.html")
	var listPages atomic.Int32

	// Rewrite every nav link so the fixture claims a single page.
	onePage := navLinkRe.ReplaceAllString(string(listing), "/all-trib-dolls-videos/1/")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/movies/") {
			_, _ = w.Write(detail)
			return
		}
		listPages.Add(1)
		_, _ = w.Write([]byte(onePage))
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

	if got := int(listPages.Load()); got != 1 {
		t.Errorf("fetched %d listing pages, want 1 — the nav says there is only one", got)
	}
}

func TestContextCancellation(t *testing.T) {
	srv, _ := newSiteServer(t)
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
