package itspov

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
	cases := []struct {
		url   string
		match bool
	}{
		{"https://itspov.com", true},
		{"https://www.itspov.com/videos", true},
		{"http://itspov.com/channels/intimatepov", true},
		// The brands' own domains are splash pages with no scene links, so they
		// are deliberately not matched.
		{"https://intimatepov.com/", false},
		{"https://backdoorpov.com/", false},
		{"", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.match {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.match)
		}
	}
}

// sorting=new is not optional: bare /videos is sorted by views, and an
// unrecognised value silently falls back to oldest-first.
func TestListingURLAlwaysSortsByNew(t *testing.T) {
	u, _ := listingURL("https://itspov.com", 3)
	if !strings.Contains(u, "sorting=new") {
		t.Errorf("listing URL %q must carry sorting=new", u)
	}
	if !strings.Contains(u, "page=3") {
		t.Errorf("listing URL %q lost the page number", u)
	}
	if strings.Contains(u, "filters=") {
		t.Errorf("the apex URL must not filter: %q", u)
	}
}

// Facet slugs are prefixed by type; the bare slug is silently ignored and
// returns the whole catalogue.
func TestListingURLFacets(t *testing.T) {
	cases := []struct {
		studioURL string
		filter    string
		studio    string
	}{
		{"https://itspov.com/channels/intimatepov", "filters=collection_intimatepov", "Intimate POV"},
		{"https://itspov.com/channels/backdoorpov", "filters=collection_backdoorpov", "Backdoor POV"},
		// An unknown channel still filters, but has no brand name to report.
		{"https://itspov.com/channels/futurepov", "filters=collection_futurepov", studioName},
		{"https://itspov.com/pornstars/keila-bassi", "filters=actor_keila-bassi", studioName},
		{"https://itspov.com/categories/anal-sex", "filters=category_anal-sex", studioName},
	}
	for _, c := range cases {
		u, studio := listingURL(c.studioURL, 1)
		if !strings.Contains(u, c.filter) {
			t.Errorf("listingURL(%q) = %q, want it to contain %q", c.studioURL, u, c.filter)
		}
		if studio != c.studio {
			t.Errorf("listingURL(%q) studio = %q, want %q", c.studioURL, studio, c.studio)
		}
	}
}

func TestParseTotal(t *testing.T) {
	if got := parseTotal(readFixture(t, "listing.html")); got != 326 {
		t.Errorf("parseTotal = %d, want 326", got)
	}
	if got := parseTotal([]byte("<html>no total here</html>")); got != 0 {
		t.Errorf("parseTotal with no marker = %d, want 0", got)
	}
}

func TestParseListing(t *testing.T) {
	items := parseListing(readFixture(t, "listing.html"))
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	first := items[0]
	if first.id != "881934" {
		t.Errorf("id = %q", first.id)
	}
	if first.slug != "housesitting-threesome" {
		t.Errorf("slug = %q", first.slug)
	}
	if !strings.HasSuffix(first.thumb, ".webp") || !strings.HasPrefix(first.thumb, "https://") {
		t.Errorf("thumb = %q", first.thumb)
	}
}

func TestParseDuration(t *testing.T) {
	// The CMS format is "24m55" / "1h04m12" — not colon-separated, so
	// parseutil's helpers do not apply.
	cases := map[string]int{
		"24m55":   24*60 + 55,
		"1h04m12": 3600 + 4*60 + 12,
		"7m00":    7 * 60,
		"5m":      5 * 60,
		"":        0,
		"24:55":   0,
		"garbage": 0,
	}
	for in, want := range cases {
		if got := parseDuration(in); got != want {
			t.Errorf("parseDuration(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestToScene(t *testing.T) {
	detail := readFixture(t, "detail.html")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(detail)
	}))
	defer srv.Close()

	orig := siteBase
	siteBase = srv.URL
	defer func() { siteBase = orig }()

	s := New()
	s.Client = srv.Client()

	sc := s.toScene(context.Background(), "https://itspov.com", "Intimate POV",
		listItem{id: "882222", slug: "keila-has-never-enough", thumb: "https://cdn/x.webp"}, time.Now())

	if sc.ID != "882222" || sc.Title != "Keila Has Never Enough" {
		t.Errorf("scene = %+v", sc)
	}
	if sc.Studio != "Intimate POV" {
		t.Errorf("Studio = %q — the channel name must win over the network name", sc.Studio)
	}
	if want := time.Date(2026, time.July, 21, 0, 0, 0, 0, time.UTC); !sc.Date.Equal(want) {
		t.Errorf("Date = %v, want %v", sc.Date, want)
	}
	if sc.Duration != 24*60+55 {
		t.Errorf("Duration = %d, want %d", sc.Duration, 24*60+55)
	}
	if !slices.Equal(sc.Performers, []string{"Keila Bassi"}) {
		t.Errorf("Performers = %v — the site-wide nav must not leak in", sc.Performers)
	}
	if sc.Thumbnail != "https://cdn/x.webp" {
		t.Errorf("Thumbnail = %q — og:image is a generic block image, so the card's thumb is used", sc.Thumbnail)
	}
	// The description span embeds a <style> block that must not survive.
	if strings.Contains(sc.Description, "<") || strings.Contains(sc.Description, "mso-data-placement") {
		t.Errorf("Description still holds markup: %q", sc.Description)
	}
	if !strings.HasPrefix(sc.Description, "Spending a week alone with Keila Bassi") {
		t.Errorf("Description = %q", sc.Description)
	}
	// "small" is an ellipsised copy of "full"; the full text must win.
	if strings.HasSuffix(sc.Description, "...") {
		t.Errorf("Description is the truncated copy: %q", sc.Description)
	}
}

func TestToSceneDropsPageWithoutTitle(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<html><body>nothing here</body></html>"))
	}))
	defer srv.Close()

	orig := siteBase
	siteBase = srv.URL
	defer func() { siteBase = orig }()

	s := New()
	s.Client = srv.Client()

	if sc := s.toScene(context.Background(), "x", studioName, listItem{id: "1", slug: "a"}, time.Now()); sc.ID != "" {
		t.Errorf("a page with no title should be dropped, got %+v", sc)
	}
}

// ---- end-to-end ----

func newSiteServer(t *testing.T, total int) (*httptest.Server, func() int) {
	t.Helper()
	listing := string(readFixture(t, "listing.html"))
	listing = strings.Replace(listing, ">326 videos<", ">"+itoa(total)+" videos<", 1)
	detail := readFixture(t, "detail.html")

	var pages atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/videos/") {
			_, _ = w.Write(detail)
			return
		}
		n := int(pages.Add(1))
		// Give each page its own ids, as the real listing does — the repeat
		// case is covered separately.
		page := listing
		for _, id := range []string{"881934", "881966"} {
			page = strings.ReplaceAll(page, id, id+itoa(n))
		}
		_, _ = w.Write([]byte(page))
	}))
	t.Cleanup(srv.Close)
	return srv, func() int { return int(pages.Load()) }
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

func TestListScenes(t *testing.T) {
	srv, _ := newSiteServer(t, 2)

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
	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2", len(scenes))
	}
	for _, sc := range scenes {
		if sc.SiteID != siteID || sc.Studio != studioName {
			t.Errorf("scene %s: SiteID=%q Studio=%q", sc.ID, sc.SiteID, sc.Studio)
		}
	}
}

// A page past the end is clamped to the last one and re-served rather than
// coming back empty, so the total is what has to stop the loop.
func TestPaginationStopsOnTotal(t *testing.T) {
	srv, pages := newSiteServer(t, perPage*3)

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

	if got := pages(); got != 3 {
		t.Errorf("fetched %d listing pages, want 3 (total %d / %d per page)", got, perPage*3, perPage)
	}
	if len(scenes) != 6 {
		t.Errorf("got %d scenes, want 6 (2 per page over 3 pages)", len(scenes))
	}
}

// The clamped last page re-serves cards already seen. Dedup has to drop them,
// or a truncated total would spin forever re-emitting the same scenes.
func TestClampedRepeatPageIsDeduped(t *testing.T) {
	listing := string(readFixture(t, "listing.html"))
	// A total three pages deep, but the server always serves page 1's cards.
	listing = strings.Replace(listing, ">326 videos<", ">"+itoa(perPage*3)+" videos<", 1)
	detail := readFixture(t, "detail.html")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/videos/") {
			_, _ = w.Write(detail)
			return
		}
		_, _ = w.Write([]byte(listing))
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
	if scenes := testutil.CollectScenes(t, ch); len(scenes) != 2 {
		t.Errorf("got %d scenes, want 2 — the repeated cards must be deduped", len(scenes))
	}
}

func TestContextCancellation(t *testing.T) {
	srv, _ := newSiteServer(t, 10_000)

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
