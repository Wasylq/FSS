package producersfun

import (
	"context"
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
		"https://producersfun.com":                                   true,
		"https://www.producersfun.com/video/luna-luxe-directors-fun": true,
		"https://producersfun.com.evil.test/":                        false,
		"":                                                           false,
	}
	for u, want := range cases {
		if got := s.MatchesURL(u); got != want {
			t.Errorf("MatchesURL(%q) = %v, want %v", u, got, want)
		}
	}
}

func newSitemapScraper(t *testing.T) (*Scraper, *httptest.Server) {
	t.Helper()
	sitemap := readFixture(t, "sitemap.xml")
	detail := readFixture(t, "detail.html")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "sitemapxml") {
			w.Header().Set("Content-Type", "application/xml")
			_, _ = w.Write(sitemap)
			return
		}
		_, _ = w.Write(detail)
	}))
	t.Cleanup(srv.Close)

	orig := siteBase
	siteBase = srv.URL
	t.Cleanup(func() { siteBase = orig })

	s := New()
	s.Client = srv.Client()
	return s, srv
}

// The sitemap mixes scenes, performers and static pages; only /video/ entries
// are scenes, and /performers (plural) is the index, not a performer.
func TestFetchSitemapSplitsScenesFromPerformers(t *testing.T) {
	s, _ := newSitemapScraper(t)

	cat, err := s.fetchSitemap(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(cat.slugs) != 3 {
		t.Errorf("got %d scene slugs, want 3 (duplicates collapsed): %v", len(cat.slugs), cat.slugs)
	}
	if len(cat.performers) != 3 {
		t.Errorf("got %d performers, want 3: %v", len(cat.performers), cat.performers)
	}
	for _, p := range cat.performers {
		if p == "performers" {
			t.Error("the /performers index was taken as a performer")
		}
	}
	// Longest first, so a prefix slug cannot shadow a longer one.
	for i := 1; i < len(cat.performers); i++ {
		if len(cat.performers[i-1]) < len(cat.performers[i]) {
			t.Errorf("performers not sorted longest-first: %v", cat.performers)
			break
		}
	}
}

// The detail page marks up no cast, so the performer is recovered from the
// scene slug. Some performer slugs are prefixes of others, which is why the
// longest match has to win.
func TestPerformerForPrefersTheLongestMatch(t *testing.T) {
	// As fetchSitemap sorts them.
	performers := []string{"luna-luxe-jr", "andi-avalon", "luna-luxe"}

	cases := map[string]string{
		"luna-luxe-directors-fun":        "Luna Luxe",
		"luna-luxe-jr-a-fucking-podcast": "Luna Luxe Jr",
		"andi-avalon-directors-fun":      "Andi Avalon",
		"someone-else-directors-fun":     "",
	}
	for slug, want := range cases {
		if got := performerFor(slug, performers); got != want {
			t.Errorf("performerFor(%q) = %q, want %q", slug, got, want)
		}
	}
}

// A bare prefix match would credit "Luna Luxe" for a "luna-luxely-..." slug.
func TestPerformerForRequiresASlugBoundary(t *testing.T) {
	if got := performerFor("luna-luxely-fun", []string{"luna-luxe"}); got != "" {
		t.Errorf("performerFor = %q, want none — the match must end at a slug boundary", got)
	}
}

func TestToScene(t *testing.T) {
	s, _ := newSitemapScraper(t)

	sc, ok := s.toScene(context.Background(), "https://producersfun.com",
		"luna-luxe-directors-fun", []string{"luna-luxe"}, time.Now())
	if !ok {
		t.Fatal("toScene returned not-ok")
	}

	if sc.ID != "luna-luxe-directors-fun" {
		t.Errorf("ID = %q — the site exposes no numeric id, so the slug is the key", sc.ID)
	}
	if sc.Title != "Luna Luxe - Director's Fun" {
		t.Errorf("Title = %q", sc.Title)
	}
	// "July 17th, 2026" — the ordinal suffix has to be stripped before parsing.
	if want := time.Date(2026, time.July, 17, 0, 0, 0, 0, time.UTC); !sc.Date.Equal(want) {
		t.Errorf("Date = %v, want %v", sc.Date, want)
	}
	// The same paragraph ends with "73 Photos", which must not be read as time.
	if sc.Duration != 42*60+16 {
		t.Errorf("Duration = %d, want %d", sc.Duration, 42*60+16)
	}
	if !slices.Equal(sc.Performers, []string{"Luna Luxe"}) {
		t.Errorf("Performers = %v", sc.Performers)
	}
	if !slices.Contains(sc.Tags, "Asian") {
		t.Errorf("Tags = %v", sc.Tags)
	}
	if !strings.HasPrefix(sc.Description, "Luna Luxe is the perfect") {
		t.Errorf("Description = %q", sc.Description)
	}
	if strings.Contains(sc.Description, "<") {
		t.Errorf("Description holds markup: %q", sc.Description)
	}
	if !strings.HasSuffix(sc.Thumbnail, ".jpg") {
		t.Errorf("Thumbnail = %q", sc.Thumbnail)
	}
}

func TestToSceneDropsPageWithoutTitle(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<html><body>nothing</body></html>"))
	}))
	defer srv.Close()

	orig := siteBase
	siteBase = srv.URL
	defer func() { siteBase = orig }()

	s := New()
	s.Client = srv.Client()

	if _, ok := s.toScene(context.Background(), "x", "slug", nil, time.Now()); ok {
		t.Error("a page with no heading should be dropped")
	}
}

// ---- end-to-end ----

func TestListScenes(t *testing.T) {
	s, srv := newSitemapScraper(t)

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
		if sc.Title == "" || sc.Date.IsZero() || sc.Duration == 0 {
			t.Errorf("scene %s incomplete: %+v", sc.ID, sc)
		}
	}
}

func TestSitemapErrorIsReported(t *testing.T) {
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
		t.Error("a sitemap failure produced no error result")
	}
}

func TestContextCancellation(t *testing.T) {
	s, srv := newSitemapScraper(t)

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
