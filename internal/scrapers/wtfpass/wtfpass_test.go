package wtfpass

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

	"github.com/Wasylq/FSS/scraper"
)

// Trimmed fixture mirroring real wtfpass.com markup: two cards covering
// the two-`<p class="title">` (compact + extended) layout, plus a
// realistic pagination block.
const listingHTML = `<html><body>

<div class="thumb-video cf">
  <div class="thumb-container">
    <a class="thumb-video-link" href="https://wtfpass.com/videos/3987/tap0016_Demida/" title="You get me to cloud nine &amp; back">
      <img class="thumb" src="https://wtfpass.com/contents/videos_screenshots/3000/3987/360x240/1.jpg" alt="You get me to cloud nine" />
      <span class="rating jsThumbRating"><i></i> 87%</span>
      <span class="duration ">27 min</span>
      <span class="hd">HD</span>
    </a>
  </div>
  <div class="thumb-data">
    <p class="title"><a href="https://wtfpass.com/videos/3987/tap0016_Demida/" class="link-black">You get me to cloud nine</a></p>
    <p class="data-row site-and-date cf">
      <span class="site"><span class="icon-c1"><i></i></span> <a href="https://wtfpass.com/sites/theartporn/" class="link-gray">The Art Porn</a></span>
    </p>
    <p class="data-row data-categories">
      <span class="icon-c1"><i></i></span>
      <a class="link-blue" href="https://wtfpass.com/categories/lingerie/">lingerie</a>
      <a class="link-blue" href="https://wtfpass.com/categories/blonde/">blonde</a>
    </p>
  </div>
  <div class="thumb-data-extend">
    <p class="title"><a href="https://wtfpass.com/videos/3987/tap0016_Demida/" class="link-black">You get me to cloud nine</a></p>
    <div class="video-data">
      <div class="data-row site-and-date">
        <span class="site"><span class="icon-c1"><i></i></span> <a href="https://wtfpass.com/sites/theartporn/" class="link-gray">The Art Porn</a></span>
        <span class="date-added"><span class="icon-c1"><i></i></span> 12 years ago</span>
        <span class="views"><span class="icon-c1"><i></i></span> 964 709</span>
      </div>
    </div>
  </div>
</div>

<div class="thumb-video cf">
  <div class="thumb-container">
    <a class="thumb-video-link" href="https://wtfpass.com/videos/4555/hmp0145_Mackenzie/" title="Relaxing massage finale">
      <img class="thumb" src="https://wtfpass.com/contents/videos_screenshots/4000/4555/360x240/1.jpg" alt="Relaxing massage finale" />
      <span class="duration">19 min</span>
    </a>
  </div>
  <div class="thumb-data">
    <p class="title"><a href="https://wtfpass.com/videos/4555/hmp0145_Mackenzie/" class="link-black">Relaxing massage finale</a></p>
    <p class="data-row site-and-date cf">
      <span class="site"><a href="https://wtfpass.com/sites/hdmassageporn/" class="link-gray">HD Massage Porn</a></span>
    </p>
  </div>
  <div class="thumb-data-extend">
    <div class="video-data">
      <div class="data-row site-and-date">
        <span class="site"><a href="https://wtfpass.com/sites/hdmassageporn/" class="link-gray">HD Massage Porn</a></span>
        <span class="views"><span class="icon-c1"><i></i></span> 1 234</span>
      </div>
    </div>
  </div>
</div>

<div class="pagination">
  <ul class="pagination-list">
    <li><span class="button active">1</span></li>
    <li><a class="button" href="/videos/2/">2</a></li>
    <li><a class="button" href="/videos/3/">3</a></li>
    <li><span class="button unactive">...</span></li>
    <li><a class="button" href="/videos/85/">85</a></li>
  </ul>
</div>
</body></html>`

const emptyHTML = `<html><body><div class="container">no scenes</div></body></html>`

func TestParseListing(t *testing.T) {
	items := parseListing([]byte(listingHTML))
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	first := items[0]
	if first.id != "3987" {
		t.Errorf("ID = %q", first.id)
	}
	if first.url != "https://wtfpass.com/videos/3987/tap0016_Demida/" {
		t.Errorf("URL = %q", first.url)
	}
	// Title from title= attribute, with entity unescape.
	if first.title != "You get me to cloud nine & back" {
		t.Errorf("Title = %q", first.title)
	}
	if first.thumb != "https://wtfpass.com/contents/videos_screenshots/3000/3987/360x240/1.jpg" {
		t.Errorf("Thumb = %q", first.thumb)
	}
	if first.duration != 27*60 {
		t.Errorf("Duration = %d, want %d (27 min)", first.duration, 27*60)
	}
	if first.series != "The Art Porn" {
		t.Errorf("Series = %q (expected per-card site label)", first.series)
	}
	if len(first.categories) != 2 {
		t.Errorf("Categories = %v, want 2", first.categories)
	}
	if first.views != 964709 {
		t.Errorf("Views = %d (expected '964 709' parsed as integer)", first.views)
	}

	second := items[1]
	if second.id != "4555" {
		t.Errorf("Second ID = %q", second.id)
	}
	if second.series != "HD Massage Porn" {
		t.Errorf("Second series = %q", second.series)
	}
	if second.views != 1234 {
		t.Errorf("Second views = %d, want 1234", second.views)
	}
}

func TestParseListing_dedupes(t *testing.T) {
	doubled := listingHTML + listingHTML
	items := parseListing([]byte(doubled))
	if len(items) != 2 {
		t.Errorf("got %d items after dedup, want 2", len(items))
	}
}

func TestEstimateTotal(t *testing.T) {
	// pagination lists /videos/2/ … /videos/85/ → max 85 × 2 = 170.
	got := estimateTotal([]byte(listingHTML), 2)
	if got != 170 {
		t.Errorf("estimateTotal = %d, want 170", got)
	}
	if got := estimateTotal([]byte(emptyHTML), 1); got != 1 {
		t.Errorf("estimateTotal(empty) = %d, want 1", got)
	}
}

func TestListingURL(t *testing.T) {
	s := New(SiteConfig{ID: "x", SiteBase: "https://wtfpass.com"})
	if got := s.listingURL(1); got != "https://wtfpass.com/videos/" {
		t.Errorf("page 1 → %q", got)
	}
	if got := s.listingURL(7); got != "https://wtfpass.com/videos/7/" {
		t.Errorf("page 7 → %q", got)
	}
}

func TestMatchesURL(t *testing.T) {
	get := func(id string) *Scraper {
		for _, cfg := range sites {
			if cfg.ID == id {
				return New(cfg)
			}
		}
		return nil
	}
	cases := []struct {
		scraperID, url string
		want           bool
	}{
		{"wtfpass", "https://wtfpass.com/", true},
		{"wtfpass", "https://wtfpass.com/videos/3/", true},
		{"wtfpass", "https://cashforsextape.com/", false},
		{"cashforsextape", "https://cashforsextape.com/videos/2/", true},
		{"theartporn", "https://theartporn.com/", true},
		{"theartporn", "https://example.com/", false},
		// Substring trap.
		{"wtfpass", "https://wtfpassx.com/", false},
	}
	for _, c := range cases {
		s := get(c.scraperID)
		if s == nil {
			t.Fatalf("unknown scraper ID %q", c.scraperID)
		}
		if got := s.MatchesURL(c.url); got != c.want {
			t.Errorf("MatchesURL[%s](%q) = %v, want %v", c.scraperID, c.url, got, c.want)
		}
	}
}

func TestSitesTable(t *testing.T) {
	seen := map[string]bool{}
	for _, cfg := range sites {
		if seen[cfg.ID] {
			t.Errorf("duplicate ID: %q", cfg.ID)
		}
		seen[cfg.ID] = true
	}
	if len(sites) != 17 {
		t.Errorf("expected 17 sites, got %d", len(sites))
	}
}

func TestToScene_parentUsesPerCardSeries(t *testing.T) {
	// Parent SiteName="" — per-card label wins.
	s := New(SiteConfig{ID: "wtfpass", SiteBase: "https://wtfpass.com", SiteName: ""})
	item := sceneItem{id: "1", title: "x", series: "The Art Porn"}
	scene := s.toScene(item, "https://wtfpass.com/", testNow())
	if scene.Series != "The Art Porn" {
		t.Errorf("Series = %q", scene.Series)
	}
}

func TestToScene_childFallsBackToSiteName(t *testing.T) {
	s := New(SiteConfig{ID: "cashforsextape", SiteBase: "https://cashforsextape.com", SiteName: "Cash for Sextape"})
	// Card without a per-card site label (rare but possible) → fall back.
	item := sceneItem{id: "1", title: "x"}
	scene := s.toScene(item, "https://cashforsextape.com/", testNow())
	if scene.Series != "Cash for Sextape" {
		t.Errorf("Series = %q (expected fallback to SiteName)", scene.Series)
	}
}

func TestListScenes_endToEnd(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		switch r.URL.Path {
		case "/videos/":
			_, _ = fmt.Fprint(w, listingHTML)
		default:
			_, _ = fmt.Fprint(w, emptyHTML)
		}
	}))
	defer ts.Close()

	s := New(SiteConfig{
		ID: "wtfpass", SiteBase: ts.URL,
		MatchRe: regexp.MustCompile(`.*`),
	})
	ch, err := s.ListScenes(context.Background(), ts.URL+"/", scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}
	var scenes int
	for r := range ch {
		if r.Kind == scraper.KindScene {
			scenes++
			if r.Scene.Studio != "WTFPass" {
				t.Errorf("Studio = %q", r.Scene.Studio)
			}
		}
	}
	if scenes != 2 {
		t.Errorf("got %d scenes, want 2", scenes)
	}
}

func TestListScenes_knownIDsStopsEarly(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		if r.URL.Path == "/videos/" {
			_, _ = fmt.Fprint(w, listingHTML)
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	s := New(SiteConfig{
		ID: "wtfpass", SiteBase: ts.URL,
		MatchRe: regexp.MustCompile(`.*`),
	})
	ch, err := s.ListScenes(context.Background(), ts.URL+"/", scraper.ListOpts{
		KnownIDs: map[string]bool{"4555": true},
	})
	if err != nil {
		t.Fatal(err)
	}
	var scenes int
	var stoppedEarly bool
	for r := range ch {
		switch r.Kind {
		case scraper.KindScene:
			scenes++
		case scraper.KindStoppedEarly:
			stoppedEarly = true
		}
	}
	if scenes != 1 || !stoppedEarly {
		t.Errorf("scenes=%d stoppedEarly=%v, want 1+true", scenes, stoppedEarly)
	}
}

func testNow() time.Time { return time.Date(2026, 5, 29, 0, 0, 0, 0, time.UTC) }
