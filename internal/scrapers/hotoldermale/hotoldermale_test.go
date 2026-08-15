package hotoldermale

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
	"github.com/Anastylosis/FSS/models"
	"github.com/Anastylosis/FSS/scraper"
)

// listingCard reproduces one card. `credits` is rendered the way the live
// listing truncates a long cast — the "and N more." phrase trails the last
// anchor and must not become part of a performer name.
func listingCard(id int, title, thumb string, minutes int, date, credits string, likes int) string {
	return fmt.Sprintf(`<div class="col-sm-4 col-xs-12 sceneBorder">
  <div class="scene_container col-12 flexGrow">
    <figure><a href="/scene/%d-%s">
      <img src="%s" class="img-responsive" alt="Photo of %s">
      <div class="short_info"><i class="icon-clock-1"></i>%d min</div>
    </a></figure>
    <div class="scene_title"><div>
      <div class="wrapperSceneTitle"><a href="/scene/%d-%s">%s</a></div>
    </div>
    <div style="clear: both;"><h4>%s</h4></div>
    <div class="info_2 clearfix">
      <span class="dateLbl">%s</span>
      <span data-actionid="ScnLik%d" class="fireActionFavLik tooltip-1 likesLbl off" title="" >%d</span>
    </div>
  </div></div>
</div>`, id, slugOf(title), thumb, title, minutes, id, slugOf(title), title, credits, date, id, likes)
}

func slugOf(title string) string {
	return strings.ToLower(strings.ReplaceAll(title, " ", "-"))
}

// detailPage carries the fuller record: the complete cast strip, the
// description immediately before the Categories heading, and the Details line.
func detailPage(id int, title, desc string, cats, cast []string, date string, minutes int, withOG bool) string {
	var catLinks []string
	for i, c := range cats {
		catLinks = append(catLinks, fmt.Sprintf(`<a href="/scenes/category/%d-%s" >%s</a>`, i+1, slugOf(c), c))
	}
	var castStrip strings.Builder
	for i, n := range cast {
		fmt.Fprintf(&castStrip, `<span class="perfImage" ><a href="/profile/%d-%s" ><img src="https://static.example/mod_%d.jpg" alt="Photo of %s" ><br >%s</a></span>`,
			600+i, slugOf(n), 600+i, n, n)
	}
	og := ""
	if withOG {
		og = `<meta property="og:image" content="https://static.example/og_` + strconv.Itoa(id) + `.jpg">`
	}
	return fmt.Sprintf(`<html><head>%s</head><body>
<div class="container margin_60"><div class="p-5">
  <h2 class="main_title sectionMainTitle">%s</h2>
</div></div>
<img src="https://static.example/_thumbs/1/f/0/9/scn_%d_1f09aab.jpg" class="img-responsive">
<div class="p-5">
  <p>%s</p>
  <h5 class="strong">Categories: %s</h5>
  <h5 class="strong">Details: %s <i class="icon-clock-1"></i>%d min</h5>
  <div>%s</div>
</div>
<div class="container margin_0"><h2 class="main_title">You Might also Like ...</h2>
  %s
</div>
</body></html>`, og, title, id, desc, strings.Join(catLinks, ", "), date, minutes, castStrip.String(),
		listingCard(999, "Some Other Scene", "https://static.example/other.jpg", 7, "Apr 9, 2020", `<a href="/profile/475-brian-davilla" >Brian Davilla</a>`, 140))
}

type stubSite struct {
	*httptest.Server
	mu    sync.Mutex
	hits  map[string]int
	pages int
	per   int
}

func (s *stubSite) hit(p string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hits[p]++
}

func (s *stubSite) count(p string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.hits[p]
}

// newSite serves `pages` listing pages of `per` cards. Past the last page it
// re-serves page 1, which is what the live site does — an empty-page stop test
// would never terminate here.
func newSite(t *testing.T, pages, per int) *stubSite {
	t.Helper()
	site := &stubSite{hits: map[string]int{}, pages: pages, per: per}
	site.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		site.hit(r.URL.Path)
		switch {
		case strings.HasPrefix(r.URL.Path, "/scene/"):
			rest := strings.TrimPrefix(r.URL.Path, "/scene/")
			idStr := rest
			if i := strings.Index(rest, "-"); i > 0 {
				idStr = rest[:i]
			}
			id, _ := strconv.Atoi(idStr)
			_, _ = fmt.Fprint(w, detailPage(id, fmt.Sprintf("Scene %d", id), "A full description.",
				[]string{"Bears", "Muscle"},
				[]string{"Mack Austin", "Davin Strong", "Dale Savage"},
				"Aug 10, 2026", 27, id%2 == 0))
		case r.URL.Path == "/scenes" || strings.HasPrefix(r.URL.Path, "/scenes/category/") || strings.HasPrefix(r.URL.Path, "/profile/"):
			page, _ := strconv.Atoi(r.URL.Query().Get("page"))
			if page < 1 {
				page = 1
			}
			// Wrap rather than end.
			if page > pages {
				page = 1
			}
			var sb strings.Builder
			sb.WriteString(`<html><body>`)
			for n := 0; n < per; n++ {
				id := 900 - ((page-1)*per + n)
				sb.WriteString(listingCard(id, fmt.Sprintf("Scene %d", id),
					fmt.Sprintf("https://static.example/card_%d.jpg", id), 20, "Jul 12, 2026",
					`<a href="/profile/600-mack-austin" >Mack Austin</a>, <a href="/profile/601-davin-strong" >Davin Strong</a> and 5 more.`, 321))
			}
			sb.WriteString(`</body></html>`)
			_, _ = fmt.Fprint(w, sb.String())
		default:
			http.NotFound(w, r)
		}
	}))
	return site
}

func newTestScraper(srv *httptest.Server) *Scraper {
	s := New()
	s.Client = srv.Client()
	s.baseOverride = srv.URL
	return s
}

func collect(t *testing.T, s *Scraper, studioURL string, opts scraper.ListOpts) ([]models.Scene, []error, int, bool) {
	t.Helper()
	ch, err := s.ListScenes(context.Background(), studioURL, opts)
	if err != nil {
		t.Fatalf("ListScenes: %v", err)
	}
	var scenes []models.Scene
	var errs []error
	total := 0
	stopped := false
	for res := range ch {
		switch res.Kind {
		case scraper.KindScene:
			scenes = append(scenes, res.Scene)
		case scraper.KindError:
			errs = append(errs, res.Err)
		case scraper.KindTotal:
			total = res.Total
		case scraper.KindStoppedEarly:
			stopped = true
		}
	}
	return scenes, errs, total, stopped
}

// Past the last page the listing re-serves page 1, so the walk has to stop on
// a page that adds no new id. An empty-page test would run to the page cap.
func TestWalkStopsWhenAPageAddsNothingNew(t *testing.T) {
	site := newSite(t, 3, 4)
	defer site.Close()

	s := newTestScraper(site.Server)
	scenes, errs, total, _ := collect(t, s, "https://www.hotoldermale.com/", scraper.ListOpts{Workers: 2})
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(scenes) != 12 || total != 12 {
		t.Fatalf("got %d scenes (total %d), want 12", len(scenes), total)
	}
	// Four listing fetches: three of content, one wrapped page to detect the end.
	if got := site.count("/scenes"); got != 4 {
		t.Errorf("fetched %d listing pages, want 4", got)
	}
}

// The detail page carries the full cast; the card truncates it. Nothing may
// keep the "and N more." phrase as part of a name.
func TestDetailSuppliesTheFullCastAndDescription(t *testing.T) {
	site := newSite(t, 1, 1)
	defer site.Close()

	s := newTestScraper(site.Server)
	scenes, _, _, _ := collect(t, s, "https://www.hotoldermale.com/", scraper.ListOpts{Workers: 1})
	if len(scenes) != 1 {
		t.Fatalf("got %d scenes, want 1", len(scenes))
	}
	got := scenes[0]

	if got.ID != "900" || got.SiteID != siteID || got.Studio != studioName {
		t.Errorf("identity = %q/%q/%q", got.ID, got.SiteID, got.Studio)
	}
	if got.Description != "A full description." {
		t.Errorf("Description = %q", got.Description)
	}
	want := []string{"Mack Austin", "Davin Strong", "Dale Savage"}
	if strings.Join(got.Performers, ",") != strings.Join(want, ",") {
		t.Errorf("Performers = %v, want %v", got.Performers, want)
	}
	for _, p := range got.Performers {
		if strings.Contains(strings.ToLower(p), "more") {
			t.Errorf("performer %q kept the truncation phrase", p)
		}
	}
	if strings.Join(got.Categories, ",") != "Bears,Muscle" {
		t.Errorf("Categories = %v", got.Categories)
	}
	// The Details line wins over the card's date and runtime.
	if got.Date.Format("2006-01-02") != "2026-08-10" {
		t.Errorf("Date = %v", got.Date)
	}
	if got.Duration != 27*60 {
		t.Errorf("Duration = %d, want %d", got.Duration, 27*60)
	}
	if got.Likes != 321 {
		t.Errorf("Likes = %d", got.Likes)
	}
}

// The related-scenes rail at the bottom of a detail page is a listing card of
// its own, so nothing may pick up its cast, its date or its title.
func TestRelatedRailDoesNotLeakIntoTheScene(t *testing.T) {
	d := parseDetail(detailPage(910, "Real Title", "Real description.",
		[]string{"Bears"}, []string{"Mack Austin"}, "Aug 10, 2026", 27, true))

	if d.title != "Real Title" {
		t.Errorf("title = %q", d.title)
	}
	if d.description != "Real description." {
		t.Errorf("description = %q", d.description)
	}
	for _, p := range d.performers {
		if p == "Brian Davilla" {
			t.Error("the related rail's performer was credited to this scene")
		}
	}
	if d.date.Format("2006-01-02") != "2026-08-10" {
		t.Errorf("date = %v — the rail's Apr 9, 2020 may have won", d.date)
	}
	if d.duration != 27*60 {
		t.Errorf("duration = %d — the rail's 7 min may have won", d.duration)
	}
}

// og:image is missing on a good share of the catalogue, so the scene's own
// `scn_{id}_` promo frame is the fallback.
func TestThumbnailFallsBackToTheSceneFrame(t *testing.T) {
	withOG := parseDetail(detailPage(910, "T", "d", nil, nil, "Aug 10, 2026", 27, true))
	if !strings.Contains(withOG.thumb, "og_910") {
		t.Errorf("thumb = %q, want the og:image", withOG.thumb)
	}
	withoutOG := parseDetail(detailPage(910, "T", "d", nil, nil, "Aug 10, 2026", 27, false))
	if !strings.Contains(withoutOG.thumb, "scn_910_") {
		t.Errorf("thumb = %q, want the scene frame", withoutOG.thumb)
	}
}

// A category or profile URL walks that path instead of the whole catalogue.
func TestFilteredListingWalksItsOwnPath(t *testing.T) {
	for _, u := range []string{
		"https://www.hotoldermale.com/scenes/category/3-bears",
		"https://www.hotoldermale.com/profile/609-mack-austin",
	} {
		site := newSite(t, 1, 2)
		s := newTestScraper(site.Server)
		scenes, errs, _, _ := collect(t, s, u, scraper.ListOpts{Workers: 1})
		if len(errs) != 0 {
			t.Errorf("%s: unexpected errors: %v", u, errs)
		}
		if len(scenes) != 2 {
			t.Errorf("%s: got %d scenes, want 2", u, len(scenes))
		}
		if site.count("/scenes") != 0 {
			t.Errorf("%s: also walked the full catalogue", u)
		}
		site.Close()
	}
}

// A single scene URL skips the walk.
func TestSingleSceneURLSkipsTheWalk(t *testing.T) {
	site := newSite(t, 3, 4)
	defer site.Close()

	s := newTestScraper(site.Server)
	scenes, errs, total, _ := collect(t, s, "https://www.hotoldermale.com/scene/910-daddy-mack-austin", scraper.ListOpts{Workers: 1})
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(scenes) != 1 || total != 1 {
		t.Fatalf("got %d scenes (total %d), want 1", len(scenes), total)
	}
	if scenes[0].ID != "910" {
		t.Errorf("ID = %q", scenes[0].ID)
	}
	if site.count("/scenes") != 0 {
		t.Error("a single-scene URL walked the listing")
	}
}

// The listing is newest-first, so a stored id ends the walk.
func TestKnownIDStopsTheWalkEarly(t *testing.T) {
	site := newSite(t, 3, 4)
	defer site.Close()

	s := newTestScraper(site.Server)
	scenes, _, _, stopped := collect(t, s, "https://www.hotoldermale.com/",
		scraper.ListOpts{Workers: 1, KnownIDs: map[string]bool{"898": true}})

	if !stopped {
		t.Error("StoppedEarly was not reported")
	}
	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want the 2 ahead of the known id", len(scenes))
	}
}

// A listing that fetched cleanly and parsed to nothing is a template change,
// which must not read as an empty catalogue to an authoritative --full save.
func TestEmptyFirstListingPageIsAParseError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, "<html><body>nothing here</body></html>")
	}))
	defer srv.Close()

	s := newTestScraper(srv)
	scenes, errs, _, _ := collect(t, s, "https://www.hotoldermale.com/", scraper.ListOpts{Workers: 1})
	if len(scenes) != 0 {
		t.Fatalf("got %d scenes", len(scenes))
	}
	if len(errs) == 0 {
		t.Fatal("an unparseable listing reported no error")
	}
	if k := scraper.Classify(errs[0]); k != scraper.FailureParse {
		t.Errorf("classified as %v, want FailureParse", k)
	}
}

// A failed detail costs the description and the full cast, not the scene: the
// card already carries id, title, date, runtime, thumbnail and a partial cast.
func TestDetailFailureKeepsTheCardOnlyScene(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/scene/") {
			http.Error(w, "gone", http.StatusInternalServerError)
			return
		}
		// Every page serves the same single card, so the walk ends on the
		// second fetch when it adds nothing new.
		_, _ = fmt.Fprint(w, `<html><body>`+listingCard(900, "Card Title", "https://static.example/c.jpg", 20, "Jul 12, 2026",
			`<a href="/profile/600-mack-austin" >Mack Austin</a>`, 5)+`</body></html>`)
	}))
	defer srv.Close()

	s := newTestScraper(srv)
	scenes, errs, _, _ := collect(t, s, "https://www.hotoldermale.com/", scraper.ListOpts{Workers: 1})
	if len(scenes) != 1 {
		t.Fatalf("got %d scenes, want the card-only scene", len(scenes))
	}
	got := scenes[0]
	if got.Title != "Card Title" || got.Duration != 20*60 || got.Date.Format("2006-01-02") != "2026-07-12" {
		t.Errorf("card fields lost: %+v", got)
	}
	if len(got.Performers) != 1 || got.Performers[0] != "Mack Austin" {
		t.Errorf("Performers = %v", got.Performers)
	}
	if len(errs) == 0 {
		t.Error("the failed detail fetch was not reported")
	}
}

func TestMatchesURL(t *testing.T) {
	s := New()
	for _, u := range []string{
		"https://hotoldermale.com",
		"https://www.hotoldermale.com/",
		"http://www.hotoldermale.com/scenes?page=3",
		"https://www.hotoldermale.com/profile/609-mack-austin",
	} {
		if !s.MatchesURL(u) {
			t.Errorf("MatchesURL(%q) = false, want true", u)
		}
	}
	for _, u := range []string{
		"https://blog.hotoldermale.com/",
		"https://hotoldermalefan.com/",
		"https://example.com/hotoldermale.com",
	} {
		if s.MatchesURL(u) {
			t.Errorf("MatchesURL(%q) = true, want false", u)
		}
	}
}

func TestListingPath(t *testing.T) {
	cases := map[string]string{
		"https://www.hotoldermale.com/":                          "/scenes",
		"https://www.hotoldermale.com/scenes":                    "/scenes",
		"https://www.hotoldermale.com/scenes/category/3-bears":   "/scenes/category/3-bears",
		"https://www.hotoldermale.com/profile/609-mack-austin":   "/profile/609-mack-austin",
		"https://www.hotoldermale.com/scenes/category/3-bears/?": "/scenes/category/3-bears",
	}
	for in, want := range cases {
		if got := listingPath(in); got != want {
			t.Errorf("listingPath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestScenesValidate(t *testing.T) {
	site := newSite(t, 1, 3)
	defer site.Close()

	s := newTestScraper(site.Server)
	scenes, _, _, _ := collect(t, s, "https://www.hotoldermale.com/", scraper.ListOpts{Workers: 2})
	for _, sc := range scenes {
		testutil.ValidateScene(t, sc)
	}
}

func TestContextCancellationStopsTheWalk(t *testing.T) {
	site := newSite(t, 40, 10)
	defer site.Close()

	s := newTestScraper(site.Server)
	ctx, cancel := context.WithCancel(context.Background())
	ch, err := s.ListScenes(ctx, "https://www.hotoldermale.com/", scraper.ListOpts{Workers: 3, Delay: 10 * time.Millisecond})
	if err != nil {
		t.Fatalf("ListScenes: %v", err)
	}
	cancel()
	for range ch { //nolint:revive // drain so the goroutines can finish their sends
	}
}
