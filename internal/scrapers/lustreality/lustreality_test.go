package lustreality

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

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url   string
		match bool
	}{
		{"https://www.lustreality.com/", true},
		{"https://lustreality.com/en/videos", true},
		{"https://www.lustreality.com/en/nothing-to-dress", true},
		{"https://lustreality.net/", false},
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

// ---- sitemap ----

func TestFetchSitemap(t *testing.T) {
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

	urls, err := s.fetchSitemap(context.Background())
	if err != nil {
		t.Fatalf("fetchSitemap: %v", err)
	}
	if len(urls) != 3 {
		t.Fatalf("got %d urls, want 3", len(urls))
	}
	if !strings.HasSuffix(urls[0], "/en/nothing-to-dress") {
		t.Errorf("urls[0] = %q", urls[0])
	}
}

func TestFetchSitemapDedupes(t *testing.T) {
	body := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
<url><loc>https://www.lustreality.com/en/a</loc></url>
<url><loc>https://www.lustreality.com/en/a</loc></url>
<url><loc>https://www.lustreality.com/en/b</loc></url>
</urlset>`)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	orig := siteBase
	siteBase = srv.URL
	defer func() { siteBase = orig }()

	s := New()
	s.Client = srv.Client()

	urls, err := s.fetchSitemap(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(urls) != 2 {
		t.Errorf("got %d urls, want 2 (duplicates collapsed)", len(urls))
	}
}

// ---- detail ----

func newDetailServer(t *testing.T, body []byte) (*Scraper, *httptest.Server) {
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
	s, srv := newDetailServer(t, readFixture(t, "detail.html"))
	now := time.Now().UTC()

	sc, ok := s.toScene(context.Background(), "https://www.lustreality.com", srv.URL+"/en/nothing-to-dress", now)
	if !ok {
		t.Fatal("toScene returned not-ok")
	}

	// The stream UUID is preferred over the slug: it survives renames.
	if sc.ID != "019f02cf-19a1-731d-bae4-dd707a75ad67" {
		t.Errorf("ID = %q, want the stream UUID", sc.ID)
	}
	if sc.SiteID != siteID || sc.Studio != studioName {
		t.Errorf("SiteID = %q, Studio = %q", sc.SiteID, sc.Studio)
	}
	if sc.Title != "Nothing to dress" {
		t.Errorf("Title = %q", sc.Title)
	}
	if !strings.HasPrefix(sc.Description, "Heading to a party") {
		t.Errorf("Description = %q", sc.Description)
	}
	// ISO 8601 "PT46M10S" -> 2770s.
	if sc.Duration != 2770 {
		t.Errorf("Duration = %d, want 2770", sc.Duration)
	}
	// uploadDate is "2025-06-30T00:00:00+01:00", i.e. 2025-06-29T23:00Z.
	want := time.Date(2025, time.June, 29, 23, 0, 0, 0, time.UTC)
	if !sc.Date.Equal(want) {
		t.Errorf("Date = %v, want %v", sc.Date, want)
	}
	if !slices.Equal(sc.Performers, []string{"Lexy Emerald"}) {
		t.Errorf("Performers = %v", sc.Performers)
	}
	if !strings.HasPrefix(sc.Thumbnail, "https://") {
		t.Errorf("Thumbnail = %q", sc.Thumbnail)
	}
	// The site has no real tag taxonomy, so tags are deliberately not set.
	if len(sc.Tags) != 0 {
		t.Errorf("Tags = %v, want none", sc.Tags)
	}
}

// Detail pages carry several JSON-LD blocks; the VideoObject must be picked by
// @type, not by position.
func TestParseVideoObjectSelectsByType(t *testing.T) {
	detail := `
<script type="application/ld+json">{"@type":"Organization","name":"LustReality"}</script>
<script type="application/ld+json">{"@type":"BreadcrumbList"}</script>
<script type="application/ld+json">{"@type":"VideoObject","name":"The Scene","duration":"PT5M"}</script>`

	vo := parseVideoObject(detail)
	if vo == nil {
		t.Fatal("no VideoObject found")
	}
	if vo.Name != "The Scene" {
		t.Errorf("Name = %q", vo.Name)
	}
}

// A page with no VideoObject is not a scene and must be dropped rather than
// emitted with empty fields.
func TestToSceneDropsPageWithoutVideoObject(t *testing.T) {
	s, srv := newDetailServer(t, []byte(`<html><head><script type="application/ld+json">{"@type":"Organization"}</script></head></html>`))

	if _, ok := s.toScene(context.Background(), "x", srv.URL+"/en/whatever", time.Now()); ok {
		t.Error("a page with no VideoObject should be dropped")
	}
}

func TestSceneIDFallsBackToSlug(t *testing.T) {
	detail := `<script type="application/ld+json">{"@type":"VideoObject","name":"X"}</script>`
	if got := sceneID(detail, "https://www.lustreality.com/en/some-slug"); got != "some-slug" {
		t.Errorf("sceneID = %q, want the slug", got)
	}
	if got := sceneID(detail, "https://www.lustreality.com/en/some-slug/"); got != "some-slug" {
		t.Errorf("sceneID with trailing slash = %q, want some-slug", got)
	}
}

// ---- end-to-end ----

func TestListScenes(t *testing.T) {
	sitemap := readFixture(t, "sitemap.xml")
	detail := readFixture(t, "detail.html")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "sitemap_video.xml") {
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
	sitemap := readFixture(t, "sitemap.xml")
	detail := readFixture(t, "detail.html")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "sitemap_video.xml") {
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
