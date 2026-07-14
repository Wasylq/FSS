package seehim

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
	for _, c := range sites {
		if c.SiteID == id {
			return c
		}
	}
	t.Fatalf("no site %q", id)
	return siteConfig{}
}

// ---- config ----

func TestSites(t *testing.T) {
	if len(sites) != 3 {
		t.Fatalf("got %d sites, want 3", len(sites))
	}
	ids := map[string]bool{}
	for _, c := range sites {
		if c.SiteID == "" || c.Domain == "" || c.StudioName == "" || c.ListingPath == "" {
			t.Errorf("incomplete config: %+v", c)
		}
		if ids[c.SiteID] {
			t.Errorf("duplicate SiteID %q", c.SiteID)
		}
		ids[c.SiteID] = true
	}
}

// seehimsolo serves its listing from "movies-2"; using "movies" there returns
// no cards at all.
func TestListingPaths(t *testing.T) {
	want := map[string]string{
		"seehimfuck": "movies",
		"seehimsolo": "movies-2",
		"ravebunnys": "movies",
	}
	for id, path := range want {
		if got := siteByID(t, id).ListingPath; got != path {
			t.Errorf("%s: ListingPath = %q, want %q", id, got, path)
		}
	}
}

func TestMatchesURL(t *testing.T) {
	s := newScraper(siteByID(t, "seehimfuck"))
	cases := []struct {
		url   string
		match bool
	}{
		{"https://seehimfuck.com/", true},
		{"https://www.seehimfuck.com/categories/movies/2/latest/", true},
		{"http://seehimfuck.com/trailers/HIM-BRICK-DANGER-5.html", true},
		{"https://seehimsolo.com/", false},
		{"https://ravebunnys.com/", false},
		{"", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.match {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.match)
		}
	}
}

func TestNoCrossMatching(t *testing.T) {
	for _, a := range sites {
		s := newScraper(a)
		for _, b := range sites {
			if a.SiteID == b.SiteID {
				continue
			}
			if u := "https://" + b.Domain + "/"; s.MatchesURL(u) {
				t.Errorf("%s wrongly matches %s", a.SiteID, u)
			}
		}
	}
}

// ---- listing ----

func TestParseListing(t *testing.T) {
	s := newScraper(siteByID(t, "seehimfuck"))
	items := s.parseListing(readFixture(t, "listing.html"))

	if len(items) != 3 {
		t.Fatalf("got %d items, want 3", len(items))
	}

	got := items[0]
	if got.id != "769" {
		t.Errorf("id = %q, want 769", got.id)
	}
	if !strings.HasSuffix(got.url, "/trailers/BTS-from-HER-TONGUE-WENT-WHERE-NO-EX-WIFE-EVER-DARED.html") {
		t.Errorf("url = %q", got.url)
	}
	if got.duration != 1823 { // 30:23
		t.Errorf("duration = %d, want 1823", got.duration)
	}
	want := time.Date(2026, time.July, 21, 0, 0, 0, 0, time.UTC)
	if !got.date.Equal(want) {
		t.Errorf("date = %v, want %v", got.date, want)
	}
	// The 3x thumbnail is preferred over 1x.
	if !strings.Contains(got.thumb, "-3x.jpg") {
		t.Errorf("thumb = %q, want the 3x variant", got.thumb)
	}

	// Newest-first ordering underpins the KnownIDs early-stop.
	for i := 1; i < len(items); i++ {
		if items[i].date.After(items[i-1].date) {
			t.Errorf("item %d (%v) is newer than item %d (%v); listing must be newest-first",
				i, items[i].date, i-1, items[i-1].date)
		}
	}
}

// The card's time div doubles as a photo count on some scenes
// ("280&nbsp;Photos, 54:16"). The runtime must be read, not the count.
func TestParseListingDurationIgnoresPhotoCount(t *testing.T) {
	s := newScraper(siteByID(t, "seehimfuck"))
	items := s.parseListing(readFixture(t, "listing.html"))
	if len(items) < 3 {
		t.Fatalf("got %d items, want at least 3", len(items))
	}
	// Fixture card 2 is "280&nbsp;Photos, 54:16" -> 3256s.
	if items[1].duration != 3256 {
		t.Errorf("duration = %d, want 3256 (54:16, not the 280 photo count)", items[1].duration)
	}
	if items[2].duration != 3259 { // "254&nbsp;Photos, 54:19"
		t.Errorf("duration = %d, want 3259", items[2].duration)
	}
}

func TestParseListingEmpty(t *testing.T) {
	s := newScraper(siteByID(t, "seehimfuck"))
	if items := s.parseListing([]byte(`<html><body>no cards</body></html>`)); len(items) != 0 {
		t.Errorf("got %d items, want 0", len(items))
	}
}

func TestNormalizeURL(t *testing.T) {
	s := newScraper(siteByID(t, "seehimfuck"))
	cases := map[string]string{
		"/content//contentthumbs/75/02/7502-3x.jpg": "https://seehimfuck.com/content//contentthumbs/75/02/7502-3x.jpg",
		"//cdn.example.com/a.jpg":                   "https://cdn.example.com/a.jpg",
		"https://seehimfuck.com/x.html":             "https://seehimfuck.com/x.html",
	}
	for in, want := range cases {
		if got := s.normalizeURL(in); got != want {
			t.Errorf("normalizeURL(%q) = %q, want %q", in, got, want)
		}
	}
}

// ---- detail ----

func TestApplyDetail(t *testing.T) {
	s := newScraper(siteByID(t, "seehimfuck"))
	scene := models.Scene{Title: "stale card title"}

	s.applyDetail(&scene, string(readFixture(t, "detail.html")))

	// The card title is unreliable, so the detail <h1> must win.
	if scene.Title != "BRICKING HER PUSSY RAW!" {
		t.Errorf("Title = %q, want the detail <h1>", scene.Title)
	}
	if !strings.HasPrefix(scene.Description, "LOOK AT THIS absolute unit") {
		t.Errorf("Description = %q", scene.Description)
	}
	if strings.Contains(scene.Description, "<") {
		t.Errorf("Description still contains markup: %q", scene.Description)
	}
	want := []string{"Bianca Bangs", "Brick Danger"}
	if !slices.Equal(scene.Performers, want) {
		t.Errorf("Performers = %v, want %v", scene.Performers, want)
	}
	// Date and duration were already set from the card, so they stay put; here
	// the scene started empty so the detail fills them.
	wantDate := time.Date(2026, time.June, 26, 0, 0, 0, 0, time.UTC)
	if !scene.Date.Equal(wantDate) {
		t.Errorf("Date = %v, want %v", scene.Date, wantDate)
	}
	if scene.Duration != 53*60 {
		t.Errorf("Duration = %d, want %d", scene.Duration, 53*60)
	}
}

// Values already read from the listing card are more precise and must not be
// overwritten by the detail page's coarser ones.
func TestApplyDetailKeepsCardValues(t *testing.T) {
	s := newScraper(siteByID(t, "seehimfuck"))
	cardDate := time.Date(2026, time.July, 21, 0, 0, 0, 0, time.UTC)
	scene := models.Scene{Date: cardDate, Duration: 3256}

	s.applyDetail(&scene, string(readFixture(t, "detail.html")))

	if !scene.Date.Equal(cardDate) {
		t.Errorf("Date = %v, want the card's %v", scene.Date, cardDate)
	}
	if scene.Duration != 3256 {
		t.Errorf("Duration = %d, want the card's 3256 (minute-only detail is coarser)", scene.Duration)
	}
}

// Tag links live under /categories/, the same namespace as the listing itself,
// so the site's own listing link must not become a tag.
func TestApplyDetailDropsListingPathFromTags(t *testing.T) {
	s := newScraper(siteByID(t, "seehimfuck"))
	detail := `
<a href="https://seehimfuck.com/categories/movies/1/latest/">Most Recent</a>
<a href="https://seehimfuck.com/categories/analsex/1/latest/">Anal Sex</a>
<a href="https://seehimfuck.com/categories/rimming/1/latest/">Rimming</a>
<a href="https://seehimfuck.com/categories/analsex/1/latest/">Anal Sex</a>`

	var scene models.Scene
	s.applyDetail(&scene, detail)

	want := []string{"Anal Sex", "Rimming"}
	if !slices.Equal(scene.Tags, want) {
		t.Errorf("Tags = %v, want %v", scene.Tags, want)
	}
}

// seehimsolo's listing path is movies-2, so that is the value its tag filter
// must drop.
func TestApplyDetailDropsListingPathPerSite(t *testing.T) {
	s := newScraper(siteByID(t, "seehimsolo"))
	detail := `
<a href="https://seehimsolo.com/categories/movies-2/1/latest/">Most Recent</a>
<a href="https://seehimsolo.com/categories/solo/1/latest/">Solo</a>`

	var scene models.Scene
	s.applyDetail(&scene, detail)

	if !slices.Equal(scene.Tags, []string{"Solo"}) {
		t.Errorf("Tags = %v, want [Solo]", scene.Tags)
	}
}

// ---- end-to-end ----

func TestListScenes(t *testing.T) {
	listing := readFixture(t, "listing.html")
	detail := readFixture(t, "detail.html")

	var detailHits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/categories/movies/1/latest/":
			_, _ = w.Write(listing)
		case strings.HasPrefix(r.URL.Path, "/trailers/"):
			detailHits.Add(1)
			_, _ = w.Write(detail)
		default:
			_, _ = fmt.Fprint(w, `<html><body></body></html>`)
		}
	}))
	defer srv.Close()

	s := newScraper(siteByID(t, "seehimfuck"))
	s.Client = srv.Client()
	s.base = srv.URL

	ch, err := s.ListScenes(context.Background(), srv.URL, scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}
	scenes := testutil.CollectScenes(t, ch)

	if len(scenes) != 3 {
		t.Fatalf("got %d scenes, want 3", len(scenes))
	}
	if got := detailHits.Load(); got != 3 {
		t.Errorf("detail fetches = %d, want 3", got)
	}
	for _, sc := range scenes {
		if sc.SiteID != "seehimfuck" || sc.Studio != "See HIM Fuck" {
			t.Errorf("scene %s: SiteID=%q Studio=%q", sc.ID, sc.SiteID, sc.Studio)
		}
		if sc.Date.IsZero() {
			t.Errorf("scene %s has no date", sc.ID)
		}
		if sc.Duration == 0 {
			t.Errorf("scene %s has no duration", sc.ID)
		}
		if len(sc.Performers) == 0 {
			t.Errorf("scene %s has no performers", sc.ID)
		}
	}
}

// A detail fetch that fails must still yield the card-derived scene.
func TestDetailFailureIsNotFatal(t *testing.T) {
	listing := readFixture(t, "listing.html")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/categories/movies/1/latest/":
			_, _ = w.Write(listing)
		case strings.HasPrefix(r.URL.Path, "/trailers/"):
			w.WriteHeader(http.StatusNotFound)
		default:
			_, _ = fmt.Fprint(w, `<html></html>`)
		}
	}))
	defer srv.Close()

	s := newScraper(siteByID(t, "seehimfuck"))
	s.Client = srv.Client()
	s.base = srv.URL

	ch, err := s.ListScenes(context.Background(), srv.URL, scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}
	scenes := testutil.CollectScenes(t, ch)
	if len(scenes) != 3 {
		t.Fatalf("got %d scenes, want 3", len(scenes))
	}
	for _, sc := range scenes {
		if sc.Title == "" || sc.Date.IsZero() {
			t.Errorf("scene %s lost its card data", sc.ID)
		}
	}
}

func TestContextCancellation(t *testing.T) {
	listing := readFixture(t, "listing.html")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(listing)
	}))
	defer srv.Close()

	s := newScraper(siteByID(t, "seehimfuck"))
	s.Client = srv.Client()
	s.base = srv.URL

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
