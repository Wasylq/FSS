package frenchtwinks

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
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

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url   string
		match bool
	}{
		{"https://www.french-twinks.com/", true},
		{"https://french-twinks.com/en/gay-porn-videos/18-years-and-horny", true},
		{"https://www.french-twinks.com/videos-porno-gay/18-ans-et-bouillant", true},
		{"https://frenchtwinks.com/", false},
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

func newSitemapServer(t *testing.T, body []byte) *Scraper {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)

	orig := siteBase
	siteBase = srv.URL
	t.Cleanup(func() { siteBase = orig })

	s := New()
	s.Client = srv.Client()
	return s
}

// Every scene appears twice in the sitemap — once under its French <loc> and
// once under its English one — so the walk must dedupe on the scene id and
// keep the English URL.
func TestFetchSitemapDedupesAndPrefersEnglish(t *testing.T) {
	s := newSitemapServer(t, readFixture(t, "sitemap.xml"))

	items, err := s.fetchSitemap(context.Background())
	if err != nil {
		t.Fatalf("fetchSitemap: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2 (the fixture holds one scene twice plus one more)", len(items))
	}

	got := items[0]
	// The id lives only in the thumbnail path.
	if got.id != "946" {
		t.Errorf("id = %q, want 946", got.id)
	}
	if !strings.Contains(got.url, "/en/") {
		t.Errorf("url = %q, want the English URL", got.url)
	}
	want := time.Date(2017, time.May, 24, 0, 0, 0, 0, time.UTC)
	if !got.date.Equal(want) {
		t.Errorf("date = %v, want %v", got.date, want)
	}
	if len(got.tags) == 0 {
		t.Error("tags are empty")
	}

	ids := []string{items[0].id, items[1].id}
	if ids[0] == ids[1] {
		t.Errorf("ids not deduped: %v", ids)
	}
}

// An entry with no English alternate is skipped rather than emitted with a
// French URL.
func TestFetchSitemapSkipsWithoutEnglish(t *testing.T) {
	body := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"
        xmlns:video="http://www.google.com/schemas/sitemap-video/1.1"
        xmlns:xhtml="http://www.w3.org/1999/xhtml">
<url>
  <loc>https://www.french-twinks.com/videos-porno-gay/sans-anglais</loc>
  <video:video>
    <video:thumbnail_loc>https://www.french-twinks.com/img/gay-video/video-gay_111-1.jpg</video:thumbnail_loc>
    <video:publication_date>2020-01-01</video:publication_date>
    <video:category>Solo</video:category>
  </video:video>
</url>
</urlset>`)

	s := newSitemapServer(t, body)
	items, err := s.fetchSitemap(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Errorf("got %d items, want 0", len(items))
	}
}

// Non-video entries (models, categories, static pages) must not become scenes.
func TestFetchSitemapSkipsNonVideo(t *testing.T) {
	body := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
<url><loc>https://www.french-twinks.com/en/french-gay-pornstars</loc></url>
</urlset>`)

	s := newSitemapServer(t, body)
	items, err := s.fetchSitemap(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Errorf("got %d items, want 0", len(items))
	}
}

// ---- detail ----

func TestApplyDetail(t *testing.T) {
	scene := models.Scene{}
	applyDetail(&scene, string(readFixture(t, "detail.html")))

	if scene.Title != "18 Years and Horny" {
		t.Errorf("Title = %q", scene.Title)
	}
	if !strings.HasPrefix(scene.Description, "What's more normal") {
		t.Errorf("Description = %q", scene.Description)
	}
	// actors is a comma-joined string, not an array.
	if !slices.Equal(scene.Performers, []string{"Jonathan Garnier"}) {
		t.Errorf("Performers = %v", scene.Performers)
	}
	if scene.Director != "Antoine Lebel" {
		t.Errorf("Director = %q", scene.Director)
	}
	// Duration is prose, not a JSON-LD field: "15 minutes".
	if scene.Duration != 900 {
		t.Errorf("Duration = %d, want 900", scene.Duration)
	}
}

// Pages carry an Organization block as well as the VideoObject, so selection
// must be by @type — taking the first block would pick the wrong one.
func TestParseVideoObjectSelectsByType(t *testing.T) {
	detail := `
<script type="application/ld+json">{"@context":"http://schema.org","@type":"Organization","name":"French Twinks Studios"}</script>
<script type="application/ld+json">{"@context":"http://schema.org","@type":"VideoObject","name":"The Real Scene","actors":"A, B"}</script>`

	vo := parseVideoObject(detail)
	if vo == nil {
		t.Fatal("no VideoObject found")
	}
	if vo.Name != "The Real Scene" {
		t.Errorf("Name = %q, want the VideoObject's, not the Organization's", vo.Name)
	}
}

func TestApplyDetailMultipleActors(t *testing.T) {
	scene := models.Scene{}
	applyDetail(&scene, `<script type="application/ld+json">{"@type":"VideoObject","name":"X","actors":"Gael Payet, Hugo Dupres ,  "}</script>`)

	if !slices.Equal(scene.Performers, []string{"Gael Payet", "Hugo Dupres"}) {
		t.Errorf("Performers = %v", scene.Performers)
	}
}

// The sitemap date wins; uploadDate is only a fallback.
func TestApplyDetailKeepsSitemapDate(t *testing.T) {
	sitemapDate := time.Date(2017, time.May, 24, 0, 0, 0, 0, time.UTC)
	scene := models.Scene{Date: sitemapDate}
	applyDetail(&scene, `<script type="application/ld+json">{"@type":"VideoObject","uploadDate":"2020-01-01T00:00:00+00:00"}</script>`)

	if !scene.Date.Equal(sitemapDate) {
		t.Errorf("Date = %v, want the sitemap's %v", scene.Date, sitemapDate)
	}
}

func TestApplyDetailFallsBackToUploadDate(t *testing.T) {
	scene := models.Scene{}
	applyDetail(&scene, `<script type="application/ld+json">{"@type":"VideoObject","uploadDate":"2020-01-02T00:00:00+00:00"}</script>`)

	want := time.Date(2020, time.January, 2, 0, 0, 0, 0, time.UTC)
	if !scene.Date.Equal(want) {
		t.Errorf("Date = %v, want %v", scene.Date, want)
	}
}

func TestApplyDetailNoJSONLD(t *testing.T) {
	scene := models.Scene{Title: "from sitemap"}
	applyDetail(&scene, `<html><body>nothing</body></html>`)

	if scene.Title != "from sitemap" {
		t.Errorf("Title = %q, want it untouched", scene.Title)
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

	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2", len(scenes))
	}
	for _, sc := range scenes {
		if sc.SiteID != siteID || sc.Studio != studioName {
			t.Errorf("scene %s: SiteID=%q Studio=%q", sc.ID, sc.SiteID, sc.Studio)
		}
		if sc.Title == "" || sc.Date.IsZero() || len(sc.Tags) == 0 {
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
		_, _ = fmt.Fprint(w, string(detail))
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
