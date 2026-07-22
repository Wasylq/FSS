package redhotstraightboys

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

// The two sites are separate catalogues on identical markup, so neither may
// answer for the other's URLs.
func TestSiblingsDoNotClaimEachOther(t *testing.T) {
	rhsb := newScraper(siteByID(t, "redhotstraightboys"))
	ssb := newScraper(siteByID(t, "spankingstraightboys"))

	if !ssb.MatchesURL("https://www.spankingstraightboys.com/tour/") {
		t.Error("spankingstraightboys does not match its own URL")
	}
	if rhsb.MatchesURL("https://www.spankingstraightboys.com/tour/") {
		t.Error("redhotstraightboys wrongly matches its sibling")
	}
	if ssb.MatchesURL("https://www.redhotstraightboys.com/tour/") {
		t.Error("spankingstraightboys wrongly matches its sibling")
	}
}

func TestMatchesURL(t *testing.T) {
	s := newScraper(siteByID(t, "redhotstraightboys"))
	cases := map[string]bool{
		"https://www.redhotstraightboys.com":                                 true,
		"https://redhotstraightboys.com/tour/categories/updates_2_d.html":    true,
		"http://www.redhotstraightboys.com/tour/updates/A-Straight-Boy.html": true,
		"https://redhotstraightboys.com.evil.test/":                          false,
		"https://girlsrimming.com/":                                          false,
		"":                                                                   false,
	}
	for u, want := range cases {
		if got := s.MatchesURL(u); got != want {
			t.Errorf("MatchesURL(%q) = %v, want %v", u, got, want)
		}
	}
}

func TestParseListing(t *testing.T) {
	s := newScraper(siteByID(t, "redhotstraightboys"))
	items := s.parseListing(readFixture(t, "listing.html"))
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}

	it := items[0]
	if it.id != "339" {
		t.Errorf("id = %q", it.id)
	}
	if it.title != "Darren's Surprise Tickling" {
		t.Errorf("title = %q", it.title)
	}
	if want := s.base + "/tour/updates/A-Straight-Boy-Is-Given-A-Surprise-Tickling.html"; it.url != want {
		t.Errorf("url = %q, want %q", it.url, want)
	}
	if want := time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC); !it.date.Equal(want) {
		t.Errorf("date = %v, want %v", it.date, want)
	}
	// The runtime is given in whole minutes.
	if it.duration != 12*60 {
		t.Errorf("duration = %d, want %d", it.duration, 12*60)
	}
	if !slices.Equal(it.performers, []string{"Darren Tate"}) {
		t.Errorf("performers = %v", it.performers)
	}
}

// The first anchor in a card wraps the thumbnail and has no text; the title is
// the second. Taking the first match would leave every title empty.
func TestTitleComesFromTheFirstNonEmptyAnchor(t *testing.T) {
	card := `<div class="update_details" data-setid="7">
	<a href="https://x/tour/updates/Y.html"><img src0_1x="https://cdn/1.jpg" /></a>
	<a href="https://x/tour/updates/Y.html">Real Title</a>
	</div>`
	s := newScraper(siteByID(t, "redhotstraightboys"))
	items := s.parseListing([]byte(card))
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].title != "Real Title" {
		t.Errorf("title = %q", items[0].title)
	}
}

// The date cell opens with an HTML comment before its value, so the cell has to
// be captured before the date is picked out of it.
func TestDateIsReadPastTheHTMLComment(t *testing.T) {
	card := `<div class="update_details" data-setid="7">
	<a href="https://x/tour/updates/Y.html">Y</a>
	<div class="cell update_date">
	<!-- Date -->
	07/15/2026		</div>
	</div>`
	s := newScraper(siteByID(t, "redhotstraightboys"))
	items := s.parseListing([]byte(card))
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	// US-format: 07/15/2026 is 15 July.
	if want := time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC); !items[0].date.Equal(want) {
		t.Errorf("date = %v, want %v", items[0].date, want)
	}
}

// Sites on this skin put a photo count in the same block as the runtime, so the
// count must not be mistaken for minutes.
func TestPhotoCountIsNotReadAsDuration(t *testing.T) {
	card := `<div class="update_details" data-setid="7">
	<a href="https://x/tour/updates/Y.html">Y</a>
	<div class="update_counts">73&nbsp;Photos, 35&nbsp;min&nbsp;of video</div>
	</div>`
	s := newScraper(siteByID(t, "redhotstraightboys"))
	items := s.parseListing([]byte(card))
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].duration != 35*60 {
		t.Errorf("duration = %d, want %d", items[0].duration, 35*60)
	}
}

// ---- detail ----

func TestApplyDetail(t *testing.T) {
	var sc models.Scene
	applyDetail(&sc, string(readFixture(t, "detail.html")))

	if !strings.HasPrefix(sc.Description, "We're committed to tickling") {
		t.Errorf("Description = %q", sc.Description)
	}
	if !slices.Equal(sc.Tags, []string{"Bubble Butt", "Hung", "Massage"}) {
		t.Errorf("Tags = %v", sc.Tags)
	}
}

// The listing routes and the tag index sit under the same /tour/categories/
// path as real tags, so both have to be excluded.
func TestListingAndTagIndexAreNotTags(t *testing.T) {
	var sc models.Scene
	applyDetail(&sc, `
	<a href="/tour/categories/updates_1_d.html">More Updates</a>
	<a href="/tour/categories/updates_12_d.html">Page 12</a>
	<a href="/tour/categories/tags.html">All Tags</a>
	<a href="/tour/categories/Hung.html">Hung</a>`)
	if !slices.Equal(sc.Tags, []string{"Hung"}) {
		t.Errorf("Tags = %v, want [Hung]", sc.Tags)
	}
}

func TestApplyDetailDedupesTags(t *testing.T) {
	var sc models.Scene
	applyDetail(&sc, `
	<a href="/tour/categories/Hung.html">Hung</a>
	<a href="/tour/categories/hung.html">Hung</a>
	<a href="/tour/categories/massage.html">Massage</a>`)
	if !slices.Equal(sc.Tags, []string{"Hung", "Massage"}) {
		t.Errorf("Tags = %v", sc.Tags)
	}
}

// ---- end-to-end ----

func newSiteServer(t *testing.T) (*httptest.Server, func() int) {
	t.Helper()
	listing := readFixture(t, "listing.html")
	detail := readFixture(t, "detail.html")
	var listPages atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/tour/updates/") {
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
	s := newScraper(siteByID(t, "redhotstraightboys"))
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
		if sc.SiteID != "redhotstraightboys" || sc.Studio != "Red Hot Straight Boys" {
			t.Errorf("scene %s: SiteID=%q Studio=%q", sc.ID, sc.SiteID, sc.Studio)
		}
		if sc.Title == "" || sc.Date.IsZero() || sc.Duration == 0 {
			t.Errorf("scene %s incomplete: %+v", sc.ID, sc)
		}
		if len(sc.Tags) == 0 {
			t.Errorf("scene %s has no tags", sc.ID)
		}
	}
	if got := listPages(); got != 2 {
		t.Errorf("fetched %d listing pages, want 2", got)
	}
}

// The card is complete on its own, so a failed detail fetch costs only the
// description and tags.
func TestDetailFailureKeepsTheScene(t *testing.T) {
	listing := readFixture(t, "listing.html")
	var listPages atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/tour/updates/") {
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
	sc := scenes[0]
	if sc.Title == "" || sc.Date.IsZero() || sc.Duration == 0 || len(sc.Performers) == 0 {
		t.Errorf("card fields lost on detail failure: %+v", sc)
	}
	if sc.Description != "" || len(sc.Tags) != 0 {
		t.Errorf("detail-only fields should be empty, got %+v", sc)
	}
}

func TestContextCancellation(t *testing.T) {
	listing := readFixture(t, "listing.html")
	detail := readFixture(t, "detail.html")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/tour/updates/") {
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
