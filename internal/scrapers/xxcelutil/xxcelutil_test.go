package xxcelutil

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
	"github.com/Wasylq/FSS/scraper"
)

func newXXCel() *Scraper {
	return New(SiteConfig{SiteID: "xxcel", Domain: "xx-cel.com", Host: "https://xx-cel.com", StudioName: "XX-Cel"})
}

const listingFixture = `
<div class="grid">
  <a href="/movies/video-megara-steele-video-8">
    <div class="image-wrapper">
      <video preload="none" poster="//media.xx-cel.com/content/movies/video-megara-steele-video-8/cover/hd.jpg"></video>
    </div>
  </a>
  <a href="/movies/video-another-scene">
    <video poster="//media.xx-cel.com/content/movies/video-another-scene/screenshots/video-another-scene_screen97.jpg"></video>
  </a>
  <a href="/movies/page-2/?sort=recent">Next</a>
  <a href="/movies/page-30/?sort=recent">Last</a>
</div>
`

const detailFixtureXC = `
<h1>Megara Steele video 8</h1>
<div class="vid-details">
  <span class="released title"> starring: <a href='/models/megara-steele'>Megara Steele</a> </span>
  <span class="released title"> released on: <strong>Feb 26, 2024</strong> </span>
  <span class="duration title"> duration: <strong>10:21</strong> </span>
</div>
`

const detailFixtureHH = `
<div class="vid-details text-center-mobile">
  <span class="feature title"> <strong><a href='/models/roxy-rush'>Roxy Rush</a></strong> </span>
  <span class="released title"> released on: <strong>Jan 26, 2024</strong> </span>
  <span class="duration title"> duration: <strong>45:09</strong> </span>
</div>
`

func TestParseListing(t *testing.T) {
	s := newXXCel()
	scenes := s.parseListing([]byte(listingFixture))
	if len(scenes) != 2 {
		t.Fatalf("expected 2 scenes (page- links skipped), got %d", len(scenes))
	}
	if scenes[0].slug != "video-megara-steele-video-8" {
		t.Errorf("slug = %q", scenes[0].slug)
	}
	if scenes[0].url != "https://xx-cel.com/movies/video-megara-steele-video-8" {
		t.Errorf("url = %q", scenes[0].url)
	}
	if scenes[0].thumb != "https://media.xx-cel.com/content/movies/video-megara-steele-video-8/cover/hd.jpg" {
		t.Errorf("thumb = %q", scenes[0].thumb)
	}
	// second card uses a screenshots-style poster (heavyonhotties layout)
	if scenes[1].thumb != "https://media.xx-cel.com/content/movies/video-another-scene/screenshots/video-another-scene_screen97.jpg" {
		t.Errorf("thumb[1] = %q", scenes[1].thumb)
	}
}

func TestEstimateTotal(t *testing.T) {
	if got := estimateTotal([]byte(listingFixture), 24); got != 30*24 {
		t.Errorf("estimateTotal = %d, want %d", got, 30*24)
	}
}

func TestParseDetail(t *testing.T) {
	dx := parseDetail([]byte(detailFixtureXC))
	if dx.date.Format("2006-01-02") != "2024-02-26" {
		t.Errorf("XC date = %v", dx.date)
	}
	if dx.duration != 10*60+21 {
		t.Errorf("XC duration = %d, want %d", dx.duration, 10*60+21)
	}
	if len(dx.performers) != 1 || dx.performers[0] != "Megara Steele" {
		t.Errorf("XC performers = %v", dx.performers)
	}

	dh := parseDetail([]byte(detailFixtureHH))
	if dh.date.Format("2006-01-02") != "2024-01-26" {
		t.Errorf("HH date = %v", dh.date)
	}
	if dh.duration != 45*60+9 {
		t.Errorf("HH duration = %d", dh.duration)
	}
	if len(dh.performers) != 1 || dh.performers[0] != "Roxy Rush" {
		t.Errorf("HH performers = %v", dh.performers)
	}
}

func TestSlugToTitle(t *testing.T) {
	cases := map[string]string{
		"video-megara-steele-video-8": "Megara Steele Video 8",
		"i-love-redheads":             "I Love Redheads",
	}
	for in, want := range cases {
		if got := slugToTitle(in); got != want {
			t.Errorf("slugToTitle(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMatchesURL(t *testing.T) {
	xc := newXXCel()
	hh := New(SiteConfig{SiteID: "heavyonhotties", Domain: "heavyonhotties.com", Host: "https://www.heavyonhotties.com"})
	if !xc.MatchesURL("https://xx-cel.com/movies/page-1/") {
		t.Error("xc should match xx-cel.com")
	}
	if xc.MatchesURL("https://heavyonhotties.com/") {
		t.Error("xc should not match heavyonhotties.com")
	}
	if !hh.MatchesURL("https://www.heavyonhotties.com/movies/i-love-redheads") {
		t.Error("hh should match www.heavyonhotties.com")
	}
}

// --- end to end ---------------------------------------------------------------
//
// The tests above exercise the parsers directly, which left the whole fetch path
// — ListScenes, run, enqueueListing, fetchPage, fetchDetail — at 0% and the
// package at 37.4%. That mattered more here than for a single site: xxcelutil is
// shared infrastructure, and its host package `xxcel` is a config-only wrapper
// with no unit tests of its own, so this was the total coverage of the whole
// network.
//
// SiteConfig.Host is injectable, so the whole walk runs against httptest.
func TestListScenesEndToEnd(t *testing.T) {
	var listingHits, detailHits int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/movies/page-"):
			listingHits++
			// Page 2 onwards is empty so the walk terminates.
			if !strings.HasPrefix(r.URL.Path, "/movies/page-1/") {
				_, _ = fmt.Fprint(w, `<div class="grid"></div>`)
				return
			}
			_, _ = fmt.Fprint(w, listingFixture)
		case strings.HasPrefix(r.URL.Path, "/movies/"):
			detailHits++
			_, _ = fmt.Fprint(w, detailFixtureXC)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	s := New(SiteConfig{SiteID: "xxcel", Domain: "xx-cel.com", Host: ts.URL, StudioName: "XX-Cel"})
	s.Client = ts.Client()

	ch, err := s.ListScenes(context.Background(), ts.URL+"/movies/", scraper.ListOpts{Workers: 2})
	if err != nil {
		t.Fatal(err)
	}
	scenes := testutil.CollectScenes(t, ch)
	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2", len(scenes))
	}
	if listingHits == 0 || detailHits == 0 {
		t.Errorf("listing/detail fetches = %d/%d, want both non-zero", listingHits, detailHits)
	}

	// Offline means offline: a scene URL off the test server means the scrape
	// escaped to the live site.
	for _, sc := range scenes {
		if !strings.HasPrefix(sc.URL, ts.URL) {
			t.Errorf("scene %q URL %q is not on the test server", sc.ID, sc.URL)
		}
		if sc.SiteID != "xxcel" {
			t.Errorf("scene %q SiteID = %q", sc.ID, sc.SiteID)
		}
		if sc.Studio != "XX-Cel" {
			t.Errorf("scene %q Studio = %q", sc.ID, sc.Studio)
		}
	}

	// Detail-page fields must actually be merged in, not just fetched. Note the
	// title comes from the slug via slugToTitle ("Megara Steele Video 8"), not
	// from the detail page's <h1> — the detail fetch supplies date, duration and
	// performers only.
	var found bool
	for _, sc := range scenes {
		if sc.ID == "video-megara-steele-video-8" {
			found = true
			if sc.Title != "Megara Steele Video 8" {
				t.Errorf("Title = %q, want the slug-derived title", sc.Title)
			}
			if sc.Duration != 10*60+21 {
				t.Errorf("Duration = %d, want %d", sc.Duration, 10*60+21)
			}
			if sc.Date.Format("2006-01-02") != "2024-02-26" {
				t.Errorf("Date = %v, want 2024-02-26", sc.Date)
			}
			if len(sc.Performers) == 0 {
				t.Error("Performers is empty — the detail page was not merged")
			}
		}
	}
	if !found {
		t.Errorf("expected scene not found; got %+v", scenes)
	}
}

func TestIDAndPatterns(t *testing.T) {
	s := newXXCel()
	if s.ID() != "xxcel" {
		t.Errorf("ID = %q", s.ID())
	}
	if len(s.Patterns()) == 0 {
		t.Error("Patterns is empty — the scraper would be invisible in `fss list-scrapers`")
	}
}
