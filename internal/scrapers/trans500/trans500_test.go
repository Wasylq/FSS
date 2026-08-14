package trans500

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
	"github.com/Anastylosis/FSS/models"
	"github.com/Anastylosis/FSS/scraper"
)

// card renders one listing tile the way the category pages do: four to a row,
// thumbnail path carrying the CMS set directory, date spelled out in full.
func card(setDir, slug, title, date, performer string, cols int) string {
	return fmt.Sprintf(`<div class="col-sm-%d pad_bottom_15 text-center">
  <a href="/tour3/trailers/%s.html">
    <img class="img-responsive" alt="%s" src="http://www.trans500.com/tour/content/%s/3.jpg" />
  </a>
  <h3><a href="/tour3/trailers/%s.html">%s</a></h3>
  <p>%s</p>
  <p class="categories">Featuring: <a href="models/%s.html">%s</a></p>
</div>`, cols, slug, title, setDir, slug, title, date, strings.ToLower(performer), performer)
}

func detailHTML(title, site, desc, setDir string) string {
	return fmt.Sprintf(`<html><body><div id="content">
<h2>%s</h2>
<p class="pull-right">Site: <b>%s</b></p>
<video poster="/tour/content/%s/0.jpg"><source src="http://www.trans500.com/tour/trailers/%s.mp4" type="video/mp4"></video>
<p class="description">%s</p>
</div></body></html>`, title, site, setDir, setDir, desc)
}

type stubSite struct {
	*httptest.Server
	listingHits map[string]int
	detailHits  map[string]int
}

// catalogueServer serves a two-page category listing plus detail pages, and
// re-serves the last page past the end the way the live CMS does instead of
// 404ing.
func catalogueServer(t *testing.T) *stubSite {
	t.Helper()
	pages := [][]string{
		{
			card("kill499", "Pounding-Pamela", "Pounding Pamela", "August 6, 2026", "Pamela Levinski", 3),
			card("tap411", "Pamela-at-Play", "Pamela at Play", "July 30, 2026", "Pamela Levinski", 3),
		},
		{card("bbtg404", "Taking-On-Tatyana", "Taking On Tatyana", "July 23, 2026", "Tatyana Torres", 3)},
	}
	site := &stubSite{listingHits: map[string]int{}, detailHits: map[string]int{}}
	site.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/tour3/category.php":
			site.listingHits[r.URL.Query().Get("id")]++
			page := 1
			_, _ = fmt.Sscanf(r.URL.Query().Get("page"), "%d", &page)
			// Clamp rather than 404 — this is what makes an "empty page" stop
			// condition wrong and a "no new ids" one right.
			if page > len(pages) {
				page = len(pages)
			}
			if page < 1 {
				page = 1
			}
			_, _ = fmt.Fprint(w, "<html><body>"+strings.Join(pages[page-1], "\n")+"</body></html>")
		case strings.HasPrefix(r.URL.Path, "/tour3/trailers/"):
			slug := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/tour3/trailers/"), ".html")
			site.detailHits[slug]++
			switch slug {
			case "Pounding-Pamela":
				_, _ = fmt.Fprint(w, detailHTML("Pounding Pamela", "I Kill It TS", "Pamela is back.", "kill499"))
			case "Pamela-at-Play":
				_, _ = fmt.Fprint(w, detailHTML("Pamela at Play", "Trans at Play", "At play.", "tap411"))
			default:
				_, _ = fmt.Fprint(w, detailHTML("Taking On Tatyana", "Big Booty TGirls", "Tatyana.", "bbtg404"))
			}
		case strings.HasPrefix(r.URL.Path, "/tour3/models/"):
			// Model pages lay their cards out two to a row.
			_, _ = fmt.Fprint(w, "<html><body><div class=\"col-sm-5\"><h2>Pamela Levinski - 2 Set Online</h2></div>"+
				card("kill499", "Pounding-Pamela", "Pounding Pamela", "", "Pamela Levinski", 6)+
				card("tap411", "Pamela-at-Play", "Pamela at Play", "", "Pamela Levinski", 6)+
				"</body></html>")
		default:
			http.NotFound(w, r)
		}
	}))
	return site
}

func collect(t *testing.T, s *Scraper, studioURL string, opts scraper.ListOpts) ([]models.Scene, []error, int) {
	t.Helper()
	ch, err := s.ListScenes(context.Background(), studioURL, opts)
	if err != nil {
		t.Fatalf("ListScenes: %v", err)
	}
	var scenes []models.Scene
	var errs []error
	total := 0
	for res := range ch {
		switch res.Kind {
		case scraper.KindScene:
			scenes = append(scenes, res.Scene)
		case scraper.KindError:
			errs = append(errs, res.Err)
		case scraper.KindTotal:
			total = res.Total
		}
	}
	return scenes, errs, total
}

func newTestScraper(srv *httptest.Server) *Scraper {
	s := New()
	s.Client = srv.Client()
	s.baseOverride = srv.URL
	return s
}

// A bare host means the whole catalogue, which is category 5 — not any of the
// per-brand ids, each of which is a subset of it.
func TestBareHostScrapesTheFullCatalogueCategory(t *testing.T) {
	site := catalogueServer(t)
	defer site.Close()

	s := newTestScraper(site.Server)
	scenes, errs, total := collect(t, s, "https://trans500.com/", scraper.ListOpts{Workers: 2})
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(scenes) != 3 || total != 3 {
		t.Fatalf("got %d scenes (total %d), want 3", len(scenes), total)
	}
	if site.listingHits[catalogueCategory] == 0 {
		t.Errorf("category %s was never fetched; hits=%v", catalogueCategory, site.listingHits)
	}
	for id := range site.listingHits {
		if id != catalogueCategory {
			t.Errorf("fetched category %s as well as the catalogue", id)
		}
	}
}

// The pages repeat past the end rather than emptying, so the walk has to stop
// on "no new ids" — an empty-page test would loop to the page cap.
func TestWalkStopsWhenAPageAddsNothingNew(t *testing.T) {
	site := catalogueServer(t)
	defer site.Close()

	s := newTestScraper(site.Server)
	scenes, _, _ := collect(t, s, "https://trans500.com/", scraper.ListOpts{Workers: 2})

	if len(scenes) != 3 {
		t.Fatalf("got %d scenes, want 3", len(scenes))
	}
	// Two pages of content, plus the clamped third that ends the walk.
	if got := site.listingHits[catalogueCategory]; got != 3 {
		t.Errorf("fetched %d listing pages, want 3 (two of content, one to detect the end)", got)
	}
	for _, sc := range scenes {
		if site.detailHits[strings.TrimPrefix(sc.URL, site.URL+"/tour3/trailers/")] > 1 {
			t.Errorf("%s fetched more than once", sc.URL)
		}
	}
}

// The scene id comes from the thumbnail's CMS set directory, which survives a
// title rename; the slug does not.
func TestSceneIDComesFromTheSetDirectory(t *testing.T) {
	site := catalogueServer(t)
	defer site.Close()

	s := newTestScraper(site.Server)
	scenes, _, _ := collect(t, s, "https://trans500.com/", scraper.ListOpts{Workers: 1})

	ids := map[string]bool{}
	for _, sc := range scenes {
		ids[sc.ID] = true
	}
	for _, want := range []string{"kill499", "tap411", "bbtg404"} {
		if !ids[want] {
			t.Errorf("missing id %s; got %v", want, ids)
		}
	}
}

// The detail page is the only place the sub-brand is stated, and it is what
// keeps the StashDB children distinguishable inside one catalogue.
func TestDetailSuppliesDescriptionAndBrand(t *testing.T) {
	site := catalogueServer(t)
	defer site.Close()

	s := newTestScraper(site.Server)
	scenes, _, _ := collect(t, s, "https://trans500.com/", scraper.ListOpts{Workers: 1})

	got := map[string]models.Scene{}
	for _, sc := range scenes {
		got[sc.ID] = sc
	}
	if s := got["kill499"]; s.Series != "I Kill It TS" || s.Description != "Pamela is back." {
		t.Errorf("kill499 = Series %q, Description %q", s.Series, s.Description)
	}
	if s := got["tap411"]; s.Series != "Trans at Play" {
		t.Errorf("tap411 Series = %q", s.Series)
	}
	if s := got["kill499"]; s.Studio != studioName {
		t.Errorf("Studio = %q, want %q", s.Studio, studioName)
	}
	if s := got["kill499"]; s.Date.Format("2006-01-02") != "2026-08-06" {
		t.Errorf("Date = %v", s.Date)
	}
	if s := got["kill499"]; len(s.Performers) != 1 || s.Performers[0] != "Pamela Levinski" {
		t.Errorf("Performers = %v", s.Performers)
	}
	// Assets are rewritten onto the origin being scraped, so a test server (or
	// an http-only spelling) does not leak requests to the live host.
	if s := got["kill499"]; !strings.HasPrefix(s.Thumbnail, site.URL) || !strings.HasPrefix(s.Preview, site.URL) {
		t.Errorf("Thumbnail/Preview not rebased: %q / %q", s.Thumbnail, s.Preview)
	}
}

// A category id scrapes that child alone.
func TestCategoryURLScrapesThatCategory(t *testing.T) {
	site := catalogueServer(t)
	defer site.Close()

	s := newTestScraper(site.Server)
	_, errs, _ := collect(t, s, "https://trans500.com/tour3/category.php?id=44", scraper.ListOpts{Workers: 1})
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if site.listingHits["44"] == 0 {
		t.Errorf("category 44 was never fetched; hits=%v", site.listingHits)
	}
	if site.listingHits[catalogueCategory] != 0 {
		t.Error("a category URL also walked the full catalogue")
	}
}

// Model pages use a two-per-row card and have no pagination, so the walk must
// read them once and stop.
func TestModelPageIsReadOnce(t *testing.T) {
	site := catalogueServer(t)
	defer site.Close()

	s := newTestScraper(site.Server)
	scenes, errs, _ := collect(t, s, "https://trans500.com/tour3/models/pamela-levinski.html", scraper.ListOpts{Workers: 1})
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2", len(scenes))
	}
	if site.listingHits[""] != 0 {
		t.Error("the model page walked into pagination")
	}
}

// The model index is not a model, so it must not be read as one.
func TestModelIndexIsNotTreatedAsAModel(t *testing.T) {
	site := catalogueServer(t)
	defer site.Close()

	s := newTestScraper(site.Server)
	_, errs, _ := collect(t, s, "https://trans500.com/tour3/models/models.html", scraper.ListOpts{Workers: 1})
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if site.listingHits[catalogueCategory] == 0 {
		t.Error("the model index did not fall through to the catalogue")
	}
}

// A listing that fetches cleanly and parses to nothing is a template change,
// and must not read as an empty catalogue to an authoritative --full save.
func TestEmptyFirstListingPageIsAParseError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, "<html><body>nothing here</body></html>")
	}))
	defer srv.Close()

	s := newTestScraper(srv)
	scenes, errs, _ := collect(t, s, "https://trans500.com/", scraper.ListOpts{Workers: 1})
	if len(scenes) != 0 {
		t.Fatalf("got %d scenes from a blank listing", len(scenes))
	}
	if len(errs) == 0 {
		t.Fatal("an unparseable listing reported no error")
	}
	if k := scraper.Classify(errs[0]); k != scraper.FailureParse {
		t.Errorf("classified as %v, want FailureParse", k)
	}
}

// A detail page that fails still yields the scene: the card already carries id,
// title, date, performers and thumbnail, so dropping it would lose more than it
// protects. The failure is still reported.
func TestDetailFailureKeepsTheSceneAndReportsTheError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/tour3/trailers/") {
			http.Error(w, "gone", http.StatusInternalServerError)
			return
		}
		page := 1
		_, _ = fmt.Sscanf(r.URL.Query().Get("page"), "%d", &page)
		_, _ = fmt.Fprint(w, "<html><body>"+
			card("kill499", "Pounding-Pamela", "Pounding Pamela", "August 6, 2026", "Pamela Levinski", 3)+
			"</body></html>")
	}))
	defer srv.Close()

	s := newTestScraper(srv)
	scenes, errs, _ := collect(t, s, "https://trans500.com/", scraper.ListOpts{Workers: 1})
	if len(scenes) != 1 {
		t.Fatalf("got %d scenes, want the card-only scene", len(scenes))
	}
	if scenes[0].Title != "Pounding Pamela" || scenes[0].ID != "kill499" {
		t.Errorf("card fields lost: %+v", scenes[0])
	}
	if len(errs) == 0 {
		t.Error("the failed detail fetch was not reported")
	}
}

func TestMatchesURL(t *testing.T) {
	s := New()
	for _, u := range []string{
		"https://trans500.com",
		"https://trans500.com/",
		"http://www.trans500.com/tour3/category.php?id=44",
		"https://trans500.com/tour3/models/pamela-levinski.html",
	} {
		if !s.MatchesURL(u) {
			t.Errorf("MatchesURL(%q) = false, want true", u)
		}
	}
	for _, u := range []string{
		"https://trans500live.com/",
		"https://bigbootytgirls.com/",
		"https://example.com/trans500.com",
	} {
		if s.MatchesURL(u) {
			t.Errorf("MatchesURL(%q) = true, want false", u)
		}
	}
}

func TestScenesValidate(t *testing.T) {
	site := catalogueServer(t)
	defer site.Close()

	s := newTestScraper(site.Server)
	scenes, _, _ := collect(t, s, "https://trans500.com/", scraper.ListOpts{Workers: 2})
	for _, sc := range scenes {
		testutil.ValidateScene(t, sc)
	}
}

func TestContextCancellationStopsTheWalk(t *testing.T) {
	site := catalogueServer(t)
	defer site.Close()

	s := newTestScraper(site.Server)
	ctx, cancel := context.WithCancel(context.Background())
	ch, err := s.ListScenes(ctx, "https://trans500.com/", scraper.ListOpts{Workers: 2, Delay: 10 * time.Millisecond})
	if err != nil {
		t.Fatalf("ListScenes: %v", err)
	}
	cancel()
	for range ch { //nolint:revive // drain so the goroutine can finish its sends
	}
}

func TestResolveURLRebasesTheTourHost(t *testing.T) {
	cases := map[string]string{
		"/tour/content/kill499/0.jpg":                        "http://base/tour/content/kill499/0.jpg",
		"http://www.trans500.com/tour/content/kill499/3.jpg": "http://base/tour/content/kill499/3.jpg",
		"https://cdn.example/x.jpg":                          "https://cdn.example/x.jpg",
		"models/pamela.html":                                 "http://base/tour3/models/pamela.html",
		"":                                                   "",
	}
	for in, want := range cases {
		if got := resolveURL("http://base", in); got != want {
			t.Errorf("resolveURL(%q) = %q, want %q", in, got, want)
		}
	}
}
