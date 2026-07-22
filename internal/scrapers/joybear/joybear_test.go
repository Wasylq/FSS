package joybear

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
		"https://www.joybear.com":                          true,
		"https://joybear.com/movies/producer-intervention": true,
		"https://joybear.com.evil.test/":                   false,
		"":                                                 false,
	}
	for u, want := range cases {
		if got := s.MatchesURL(u); got != want {
			t.Errorf("MatchesURL(%q) = %v, want %v", u, got, want)
		}
	}
}

func newSiteScraper(t *testing.T) (*Scraper, *httptest.Server) {
	t.Helper()
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
	t.Cleanup(srv.Close)

	orig := siteBase
	siteBase = srv.URL
	t.Cleanup(func() { siteBase = orig })

	s := New()
	s.Client = srv.Client()
	return s, srv
}

// The sitemap also lists models and DVDs; only /movies/ entries are scenes,
// and the duplicate entry must collapse.
func TestFetchSitemapKeepsOnlyMovies(t *testing.T) {
	s, _ := newSiteScraper(t)

	slugs, err := s.fetchSitemap(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(slugs, []string{"producer-intervention", "fuck-normal"}) {
		t.Errorf("slugs = %v", slugs)
	}
}

func TestToScene(t *testing.T) {
	s, _ := newSiteScraper(t)

	sc, ok := s.toScene(context.Background(), "https://www.joybear.com", "producer-intervention", time.Now())
	if !ok {
		t.Fatal("toScene returned not-ok")
	}

	if sc.ID != "producer-intervention" {
		t.Errorf("ID = %q — the site exposes no numeric id", sc.ID)
	}
	// The heading is prefixed with "Scene - ".
	if sc.Title != "Producer Intervention" {
		t.Errorf("Title = %q, want the prefix stripped", sc.Title)
	}
	// The <title> is the only place the collection is exposed.
	if sc.Series != "The Love House" {
		t.Errorf("Series = %q", sc.Series)
	}
	if !slices.Contains(sc.Performers, "Jasko Fide") {
		t.Errorf("Performers = %v", sc.Performers)
	}
	if !strings.HasPrefix(sc.Description, "Everyone knew Kara") {
		t.Errorf("Description = %q", sc.Description)
	}
	if strings.Contains(sc.Description, "<") {
		t.Errorf("Description holds markup: %q", sc.Description)
	}
	if !strings.HasSuffix(sc.Thumbnail, ".webp") {
		t.Errorf("Thumbnail = %q", sc.Thumbnail)
	}
	// The site publishes neither anywhere.
	if !sc.Date.IsZero() {
		t.Errorf("Date = %v, want zero", sc.Date)
	}
	if sc.Duration != 0 {
		t.Errorf("Duration = %d, want 0", sc.Duration)
	}
}

// The categories block is rendered inside an HTML comment. The values are real
// per-scene metadata the template simply does not display.
func TestCommentedCategoriesAreRead(t *testing.T) {
	got := commentedCategories(string(readFixture(t, "detail.html")))
	if !slices.Equal(got, []string{"Hung", "Menage a trois", "Outdoor Sex"}) {
		t.Errorf("categories = %v", got)
	}
}

func TestCommentedCategoriesDedupesAndHandlesAbsence(t *testing.T) {
	got := commentedCategories(`<!-- <div class="categories">
	<ul class="castDetails"><li><a href="#">Hung</a></li><li><a href="#">Hung</a></li><li><a href="#">Tattoos</a></li></ul>
	</div>-->`)
	if !slices.Equal(got, []string{"Hung", "Tattoos"}) {
		t.Errorf("categories = %v", got)
	}
	if got := commentedCategories("<html>no categories</html>"); got != nil {
		t.Errorf("categories = %v, want nil", got)
	}
}

func TestSeriesFromTitle(t *testing.T) {
	cases := map[string]string{
		"<title>Joybear.com |  The Love House | Producer Intervention</title>": "The Love House",
		"<title>Joybear.com | Pleasure Fix Series | Fuck Normal</title>":       "Pleasure Fix Series",
		// Not enough segments to carry a collection.
		"<title>Joybear.com | Home</title>": "",
		"<html>no title</html>":             "",
	}
	for in, want := range cases {
		if got := seriesFromTitle(in); got != want {
			t.Errorf("seriesFromTitle(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestToSceneDropsPageWithoutHeading(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<html><body>nothing</body></html>"))
	}))
	defer srv.Close()

	orig := siteBase
	siteBase = srv.URL
	defer func() { siteBase = orig }()

	s := New()
	s.Client = srv.Client()

	if _, ok := s.toScene(context.Background(), "x", "slug", time.Now()); ok {
		t.Error("a page with no heading should be dropped")
	}
}

// ---- end-to-end ----

func TestListScenes(t *testing.T) {
	s, srv := newSiteScraper(t)

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
		if sc.Title == "" || len(sc.Performers) == 0 {
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
	s, srv := newSiteScraper(t)

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
