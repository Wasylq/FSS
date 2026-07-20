package kristenbjorn

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Wasylq/FSS/internal/httpx"
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

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url   string
		match bool
	}{
		{"https://www.kristenbjorn.com", true},
		{"https://kristenbjorn.com/video-2675/fire-dance", true},
		{"http://www.kristenbjorn.com/pornvideo-tags/muscle", true},
		{"https://kbcastings.com/", false},
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

// The sitemap lists models, tags, DVDs and theatre pages alongside scenes.
func TestFetchSitemapKeepsOnlyVideos(t *testing.T) {
	sm := readFixture(t, "sitemap.xml")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write(sm)
	}))
	defer srv.Close()

	orig := siteBase
	siteBase = srv.URL
	defer func() { siteBase = orig }()

	s := New()
	s.Client = srv.Client()

	refs, err := s.fetchSitemap(context.Background())
	if err != nil {
		t.Fatalf("fetchSitemap: %v", err)
	}
	if len(refs) != 3 {
		t.Fatalf("got %d refs, want 3 (the model entry must be dropped)", len(refs))
	}
	for _, r := range refs {
		if !strings.Contains(r.url, "/video-") {
			t.Errorf("non-video url kept: %q", r.url)
		}
	}
}

// The sitemap is ~1.4 MB, so the read cap has to exceed the httpx default or a
// truncated read yields zero scenes.
func TestMaxSitemapBytesExceedsDefault(t *testing.T) {
	if maxSitemapBytes <= httpx.MaxPageBytes {
		t.Errorf("maxSitemapBytes = %d, must exceed httpx.MaxPageBytes = %d", maxSitemapBytes, httpx.MaxPageBytes)
	}
}

// ---- detail ----

func newDetailScraper(t *testing.T, body []byte) (*Scraper, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)

	s := New()
	s.Client = srv.Client()
	return s, srv
}

func TestToScene(t *testing.T) {
	s, srv := newDetailScraper(t, readFixture(t, "detail.html"))

	sc, ok := s.toScene(context.Background(), "https://www.kristenbjorn.com", sceneRef{id: "2675", url: srv.URL + "/video-2675/x"}, time.Now())
	if !ok {
		t.Fatal("toScene returned not-ok")
	}
	if sc.ID != "2675" {
		t.Errorf("ID = %q", sc.ID)
	}
	if !strings.HasPrefix(sc.Title, "FIRE DANCE REMASTERED 7") {
		t.Errorf("Title = %q", sc.Title)
	}
	// uploadDate is "2026-07-17 06:04:50" — not RFC 3339.
	want := time.Date(2026, time.July, 17, 6, 4, 50, 0, time.UTC)
	if !sc.Date.Equal(want) {
		t.Errorf("Date = %v, want %v", sc.Date, want)
	}
	// The JSON-LD description is HTML.
	if strings.Contains(sc.Description, "<") {
		t.Errorf("Description still holds markup: %q", sc.Description)
	}
	if !strings.HasPrefix(sc.Description, "These amazing castles") {
		t.Errorf("Description = %q", sc.Description)
	}
	// Names are rendered lower-case in the title attributes.
	if !slices.Contains(sc.Performers, "Carlos Caballero") {
		t.Errorf("Performers = %v", sc.Performers)
	}
	if !slices.Contains(sc.Categories, "Muscle") {
		t.Errorf("Categories = %v", sc.Categories)
	}
	// The site publishes no runtime.
	if sc.Duration != 0 {
		t.Errorf("Duration = %d, want 0", sc.Duration)
	}
}

// Pages carry a Product block as well as the VideoObject, so selection must be
// by @type — taking the first block would pick the wrong one.
func TestParseVideoObjectSelectsByType(t *testing.T) {
	detail := `
<script type="application/ld+json">{"@context":"https://schema.org","@type":"Product","name":"Wrong"}</script>
<script type="application/ld+json">{"@context":"https://schema.org","@type":"VideoObject","name":"Right"}</script>`

	vo := parseVideoObject(detail)
	if vo == nil {
		t.Fatal("no VideoObject found")
	}
	if vo.Name != "Right" {
		t.Errorf("Name = %q, want the VideoObject's", vo.Name)
	}
}

// The title attributes are what separate the scene's own credits from the
// site-wide nav menu, which links all 876 stars and 599 categories.
func TestTitleCasedScopesToTitleAttributes(t *testing.T) {
	detail := `
<a href="/gay-porn-star/345/carl-wilde" title="Gay Porn Star: carl wilde">Carl Wilde</a>
<a href="/pornvideo-tags/muscle" title="Categorie: muscle">Muscle</a>
<div class="nav-menu">
  <a href="/gay-porn-star/999/someone-else">Someone Else</a>
  <a href="/pornvideo-tags/other">Other</a>
</div>`

	if got := titleCased(starRe, detail); !slices.Equal(got, []string{"Carl Wilde"}) {
		t.Errorf("performers = %v, want [Carl Wilde] — nav links must not leak in", got)
	}
	if got := titleCased(catRe, detail); !slices.Equal(got, []string{"Muscle"}) {
		t.Errorf("categories = %v, want [Muscle]", got)
	}
}

func TestTitleCase(t *testing.T) {
	cases := map[string]string{
		"carl wilde":         "Carl Wilde",
		"muscle":             "Muscle",
		"mauricio goldstein": "Mauricio Goldstein",
		"":                   "",
	}
	for in, want := range cases {
		if got := titleCase(in); got != want {
			t.Errorf("titleCase(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestToSceneDropsPageWithoutVideoObject(t *testing.T) {
	s, srv := newDetailScraper(t, []byte(`<html><head><script type="application/ld+json">{"@type":"Product"}</script></head></html>`))
	if _, ok := s.toScene(context.Background(), "x", sceneRef{id: "1", url: srv.URL + "/video-1/x"}, time.Now()); ok {
		t.Error("a page with no VideoObject should be dropped")
	}
}

// ---- end-to-end ----

func TestListScenes(t *testing.T) {
	sitemap := readFixture(t, "sitemap.xml")
	detail := readFixture(t, "detail.html")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/sitemap.xml" {
			w.Header().Set("Content-Type", "application/xml")
			_, _ = w.Write(sitemap)
			return
		}
		_, _ = w.Write(detail)
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
		if sc.Title == "" || sc.Date.IsZero() {
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
	sitemap := readFixture(t, "sitemap.xml")
	detail := readFixture(t, "detail.html")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/sitemap.xml" {
			_, _ = w.Write(sitemap)
			return
		}
		_, _ = w.Write(detail)
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
