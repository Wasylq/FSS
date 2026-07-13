package vrlatina

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

// ---- TestMatchesURL ----

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url   string
		match bool
	}{
		{"https://vrlatina.com/", true},
		{"https://vrlatina.com/most-recent/", true},
		{"https://www.vrlatina.com/most-recent/page3.html", true},
		{"https://vrlatina.com/models/bianca-still-111.html", true},
		{"https://vrlatina.com/pornstars/mishell-evil-167.html", true},
		{"https://vrlatina.com/search/anal/", true},
		{"https://vrporn.com/", false},
		{"", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.match {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.match)
		}
	}
}

// ---- TestSinglePageDetection ----

// The /most-recent/ listing paginates; model and tag pages do not. Picking the
// wrong mode either stops after one page or walks pages that 404.
func TestSinglePageDetection(t *testing.T) {
	cases := []struct {
		url    string
		single bool
	}{
		{"https://vrlatina.com/most-recent/", false},
		{"https://vrlatina.com/most-recent/page4.html", false},
		{"https://vrlatina.com/", false},
		{"https://vrlatina.com/models/bianca-still-111.html", true},
		{"https://vrlatina.com/pornstars/mishell-evil-167.html", true},
		{"https://vrlatina.com/search/doggy-style/", true},
	}
	for _, c := range cases {
		if got := singlePageRe.MatchString(c.url); got != c.single {
			t.Errorf("singlePage(%q) = %v, want %v", c.url, got, c.single)
		}
	}
}

// ---- TestListingPageURL ----

func TestListingPageURL(t *testing.T) {
	orig := siteBase
	siteBase = "https://vrlatina.com"
	defer func() { siteBase = orig }()

	// Page 1 is the bare path — there is no page1.html.
	if got, want := listingPageURL(1), "https://vrlatina.com/most-recent/"; got != want {
		t.Errorf("page 1 = %q, want %q", got, want)
	}
	if got, want := listingPageURL(2), "https://vrlatina.com/most-recent/page2.html"; got != want {
		t.Errorf("page 2 = %q, want %q", got, want)
	}
	if got, want := listingPageURL(27), "https://vrlatina.com/most-recent/page27.html"; got != want {
		t.Errorf("page 27 = %q, want %q", got, want)
	}
}

// ---- TestParseCards ----

func TestParseCards(t *testing.T) {
	cards := parseCards(readFixture(t, "listing.html"))
	if len(cards) != 3 {
		t.Fatalf("got %d cards, want 3", len(cards))
	}

	got := cards[0]
	if got.id != "562" {
		t.Errorf("id = %q, want 562", got.id)
	}
	if !strings.HasSuffix(got.url, "/video/sensationanal-562.html") {
		t.Errorf("url = %q", got.url)
	}
	if got.title != "SensationANAL" {
		t.Errorf("title = %q", got.title)
	}
	// "33:20" -> 2000s
	if got.duration != 2000 {
		t.Errorf("duration = %d, want 2000", got.duration)
	}
	if !slices.Equal(got.performers, []string{"Mishell Evil"}) {
		t.Errorf("performers = %v", got.performers)
	}
	if !strings.HasPrefix(got.thumbnail, "https://") {
		t.Errorf("thumbnail = %q", got.thumbnail)
	}
	if !strings.HasSuffix(got.preview, ".mp4") {
		t.Errorf("preview = %q", got.preview)
	}

	// Cards must stay newest-first for the KnownIDs early-stop to be sound.
	wantIDs := []string{"562", "561", "560"}
	for i, id := range wantIDs {
		if cards[i].id != id {
			t.Errorf("card[%d].id = %q, want %q", i, cards[i].id, id)
		}
	}
}

func TestParseCardsDedupAndSkipsNonVideo(t *testing.T) {
	body := []byte(`
<!-- item -->
<div class="item-col"><a href="https://vrlatina.com/video/one-1.html"><span class="item-name">One</span></a></div>
<!-- item END -->
<!-- item -->
<div class="item-col"><a href="https://vrlatina.com/video/one-1.html"><span class="item-name">One dup</span></a></div>
<!-- item END -->
<!-- item -->
<div class="item-col"><a href="https://vrlatina.com/models/somebody-9.html">not a video</a></div>
<!-- item END -->`)

	cards := parseCards(body)
	if len(cards) != 1 {
		t.Fatalf("got %d cards, want 1", len(cards))
	}
	if cards[0].id != "1" {
		t.Errorf("id = %q, want 1", cards[0].id)
	}
}

// TestParseCardTitleFallback covers a card whose item-name span is missing —
// the slug carries a usable title.
func TestParseCardTitleFallback(t *testing.T) {
	body := []byte(`<!-- item --><a href="https://vrlatina.com/video/passion-no-problem-560.html"></a><!-- item END -->`)
	cards := parseCards(body)
	if len(cards) != 1 {
		t.Fatalf("got %d cards, want 1", len(cards))
	}
	if cards[0].title != "passion no problem" {
		t.Errorf("title = %q, want %q", cards[0].title, "passion no problem")
	}
}

// ---- TestDetailParsing ----

// The date/tags/performer containers open with a nested <div class="label">
// caption, so this pins that the document-wide matching still lands on the
// real data rather than the caption.
func TestDetailParsing(t *testing.T) {
	detail := readFixture(t, "detail.html")

	m := releaseDateRe.FindSubmatch(detail)
	if m == nil {
		t.Fatal("release date not found")
	}
	if got := string(m[1]); got != "Jul 14, 2026" {
		t.Errorf("date = %q, want %q", got, "Jul 14, 2026")
	}
	if _, err := time.Parse(dateLayout, string(m[1])); err != nil {
		t.Errorf("date %q does not parse with layout %q: %v", m[1], dateLayout, err)
	}

	var tags []string
	for _, tm := range tagRe.FindAllSubmatch(detail, -1) {
		tags = append(tags, string(tm[1]))
	}
	if len(tags) != 22 {
		t.Errorf("got %d tags, want 22", len(tags))
	}
	if !slices.Contains(tags, "Anal") || !slices.Contains(tags, "8K") {
		t.Errorf("tags missing expected entries: %v", tags)
	}

	var names []string
	for _, nm := range modelRe.FindAllSubmatch(detail, -1) {
		names = append(names, string(nm[1]))
	}
	if !slices.Equal(names, []string{"Mishell Evil"}) {
		t.Errorf("performers = %v, want [Mishell Evil]", names)
	}
}

// ---- end-to-end ----

func newTestSite(t *testing.T) (*Scraper, *httptest.Server, *atomic.Int32) {
	t.Helper()
	listing := readFixture(t, "listing.html")
	detail := readFixture(t, "detail.html")

	var detailHits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/most-recent/":
			_, _ = w.Write(listing)
		case strings.HasPrefix(r.URL.Path, "/video/"):
			detailHits.Add(1)
			_, _ = w.Write(detail)
		case strings.HasPrefix(r.URL.Path, "/models/"),
			strings.HasPrefix(r.URL.Path, "/pornstars/"),
			strings.HasPrefix(r.URL.Path, "/search/"):
			_, _ = w.Write(listing)
		default:
			// Later /most-recent/pageN.html are past the end.
			_, _ = fmt.Fprint(w, `<html><body></body></html>`)
		}
	}))
	t.Cleanup(srv.Close)

	orig := siteBase
	siteBase = srv.URL
	t.Cleanup(func() { siteBase = orig })

	s := New()
	s.Client = srv.Client()
	return s, srv, &detailHits
}

func TestRunListing(t *testing.T) {
	s, srv, detailHits := newTestSite(t)

	scenes, _ := collect(t, s, srv.URL+"/most-recent/")

	if len(scenes) != 3 {
		t.Fatalf("got %d scenes, want 3", len(scenes))
	}
	if got := detailHits.Load(); got != 3 {
		t.Errorf("detail fetches = %d, want 3", got)
	}

	sc := scenes[0]
	if sc.ID != "562" {
		t.Errorf("ID = %q, want 562", sc.ID)
	}
	if sc.SiteID != siteID {
		t.Errorf("SiteID = %q", sc.SiteID)
	}
	if sc.Studio != studioName {
		t.Errorf("Studio = %q", sc.Studio)
	}
	if sc.Title != "SensationANAL" {
		t.Errorf("Title = %q", sc.Title)
	}
	if sc.Duration != 2000 {
		t.Errorf("Duration = %d, want 2000", sc.Duration)
	}
	want := time.Date(2026, time.July, 14, 0, 0, 0, 0, time.UTC)
	if !sc.Date.Equal(want) {
		t.Errorf("Date = %v, want %v", sc.Date, want)
	}
	if !slices.Equal(sc.Performers, []string{"Mishell Evil"}) {
		t.Errorf("Performers = %v", sc.Performers)
	}
	if sc.Description == "" {
		t.Error("Description is empty")
	}
	if sc.ScrapedAt.IsZero() {
		t.Error("ScrapedAt is zero")
	}

	// The site lists each performer's name among the tags too; those must be
	// filtered out so tags stay descriptive.
	if slices.Contains(sc.Tags, "Mishell Evil") {
		t.Errorf("performer name leaked into tags: %v", sc.Tags)
	}
	if len(sc.Tags) != 21 {
		t.Errorf("got %d tags, want 21 (22 minus the performer)", len(sc.Tags))
	}
}

func TestRunSinglePage(t *testing.T) {
	s, srv, _ := newTestSite(t)

	scenes, total := collect(t, s, srv.URL+"/models/mishell-evil-167.html")
	if len(scenes) != 3 {
		t.Fatalf("got %d scenes, want 3", len(scenes))
	}
	if total != 3 {
		t.Errorf("total = %d, want 3", total)
	}
}

// TestDetailFailureIsNotFatal — the listing card alone is a usable scene.
func TestDetailFailureIsNotFatal(t *testing.T) {
	listing := readFixture(t, "listing.html")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/most-recent/":
			_, _ = w.Write(listing)
		case strings.HasPrefix(r.URL.Path, "/video/"):
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

	scenes, _ := collect(t, s, srv.URL+"/most-recent/")
	if len(scenes) != 3 {
		t.Fatalf("got %d scenes, want 3", len(scenes))
	}
	for _, sc := range scenes {
		if sc.Title == "" {
			t.Errorf("scene %s has no title", sc.ID)
		}
		if sc.Duration == 0 {
			t.Errorf("scene %s lost its card duration", sc.ID)
		}
		if !sc.Date.IsZero() {
			t.Errorf("scene %s: Date should be zero without a detail page", sc.ID)
		}
	}
}

func TestContextCancellation(t *testing.T) {
	s, srv, _ := newTestSite(t)

	ctx, cancel := context.WithCancel(context.Background())
	ch, err := s.ListScenes(ctx, srv.URL+"/most-recent/", scraper.ListOpts{})
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

// ---- helpers ----

func collect(t *testing.T, s *Scraper, studioURL string) ([]models.Scene, int) {
	t.Helper()
	ch, err := s.ListScenes(context.Background(), studioURL, scraper.ListOpts{})
	if err != nil {
		t.Fatalf("ListScenes: %v", err)
	}
	var scenes []models.Scene
	total := 0
	for res := range ch {
		switch res.Kind {
		case scraper.KindScene:
			scenes = append(scenes, res.Scene)
		case scraper.KindTotal:
			total = res.Total
		case scraper.KindError:
			t.Fatalf("scraper error: %v", res.Err)
		}
	}
	return scenes, total
}
