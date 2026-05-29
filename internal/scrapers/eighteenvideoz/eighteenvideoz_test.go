package eighteenvideoz

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/Wasylq/FSS/scraper"
)

// ---- VariantA-parent fixture (18videoz.com style) ----

const parentListingHTML = `<html><body>
<div class="holder">

<div class="th">
  <a class="thumb_wrap" onclick="re_add_click_old('2104', '1')">
    <span class="wrap_image">
      <img src="https://cdn2.18videoz.com/images/wtl1602-1.jpg" alt="">
      <span class="stick">New</span>
      <span class="time">22:53</span>
    </span>
    <span class="tools">
      <span class="caption">Teen babe intense sensual fuck &amp; more</span>
      <span class="sub">
        <span class="box_view">
          Scene from <span class="author">Teeny Lovers</span>
        </span>
      </span>
    </span>
  </a>
</div>

<div class="th">
  <a class="thumb_wrap" onclick="re_add_click_old('2220', '5')">
    <span class="wrap_image">
      <img src="https://cdn2.18videoz.com/images/wtl1685-5.jpg" alt="">
      <span class="time">26:30</span>
    </span>
    <span class="tools">
      <span class="caption">Teen couple making a sex video</span>
      <span class="sub">
        <span class="box_view">
          Scene from <span class="author">Casual Teen Sex</span>
        </span>
      </span>
    </span>
  </a>
</div>

</div>

<div class="pagination_sub">
  <ul class="holder">
    <li class="item_page"><span>1</span></li>
    <li class="item_page"><span><a href="/index.php/main/show_sets2/9">9</a></span></li>
  </ul>
</div>
</body></html>`

// ---- VariantB-rich fixture (casualteensex / teensanalyzed style) ----

const richListingHTML = `<html><body>
<div class="th" onclick="show_trailer('539');add_click('539', '1', '5');">
  <div class="t">
    <div><img src="https://cdn2.casualteensex.com/tour/images/562/562-5.jpg"></div>
  </div>
  <div id="desc539" style="display:none;">Long description body for scene 539. Hot teen scene.</div>
  <div id="time539" style="display:none;">00:00 / 28:27</div>
  <div class="desc">
    <p class="d1">Asslick leads to hot anal sex</p>
    <p class="d2">Views: <strong>10103063</strong></p>
  </div>
</div>

<div class="th" style="width:330px;" id=thumb_174 onclick="add_click('174', '0', '2'); show_trailer('174');">
  <div class="t">
    <img id=img_174 src="https://cdn2.teensanalyzed.com/images/tour2/157/157-2.jpg">
    <div class="time">24:05</div>
  </div>
  <div id="title174" style="display:none;">A smooth first anal</div>
  <div id="desc174" style="display:none;">How can this beautiful slender teeny read a magazine in bed when her sex-addicted boyfriend is extremely horny.</div>
  <div id="time174" style="display:none;">00:00 / 24:05</div>
  <div class="desc">
    <p class="d1"><a href="/index.php/main/show_sets/174/0">A smooth first anal</a></p>
  </div>
</div>

</body></html>`

// ---- VariantD-table fixture (firstanaldate / bangmyteenass style) ----

const tableListingHTML = `<html><body>
<table width="958">
  <tr><td><img src="images/tr01_1.gif" width="958" height="26"></td></tr>
  <tr><td height="38" colspan="7" align="center" background="images/tr01_2.gif" class="title">Deep and passionate anal</td></tr>
  <tr><td colspan="7"><img src="images/tr01_3.gif" width="958" height="10"></td></tr>
  <tr><td colspan="3"><img src="images/092-1.jpg" width="620" height="349"></td>
      <td><img src="images/092-2.jpg" width="300" height="349"></td></tr>
  <tr><td colspan="3">
    <img src="images/092-3.jpg" width="300" height="169">
    <img src="images/092-4.jpg" width="300" height="169">
  </td></tr>
  <tr><td colspan="7" class="description">Molly loves it when her boyfriend starts it slow giving her a gentle sensual kiss.</td></tr>
</table>

<table width="958">
  <tr><td colspan="7" class="title">Assfucked in pink stockings</td></tr>
  <tr><td><img src="images/093-1.jpg" width="300" height="169"></td></tr>
  <tr><td class="description">This gorgeous blonde teeny looks so seductive in her pink hold-up stockings.</td></tr>
</table>

<table class="pages">
  <tr><td colspan="7"><a href="index1.htm?">NEXT PAGE &gt;&gt;</a></td></tr>
  <tr><td colspan="7"><a href="index3.htm">page 3</a></td></tr>
</table>
</body></html>`

// ---- VariantC-thumb fixture (younglibertines style) ----

const thumbListingHTML = `<html><body>
<div class="thumbs">

<div class="thumb">
  <div class="t">
    <img src="https://cdn2.younglibertines.com/pictures/286/286-1.jpg" onclick="show_trailer('286');add_click('286', '1');">
  </div>
  <div class="desc">
    <div style="height:70px;">
      <p>When Ania told me she wanted us to have sex with another girl I got so horny that we had to call her girlfriend.</p>
    </div>
    <div class="time">17:43</div>
  </div>
</div>

</div>

<ul class="holder">
  <li class="item_page"><span><a href="/index.php/main/show_sets/4">4</a></span></li>
</ul>
</body></html>`

const emptyHTML = `<html><body><div class="container">no scenes</div></body></html>`

func parentConfig(base string) SiteConfig {
	return SiteConfig{
		ID: "18videoz", SiteBase: base, SiteName: "",
		PaginationPath: "/index.php/main/show_sets2",
		Patterns:       []string{"18videoz.com/"},
		MatchRe:        regexp.MustCompile(`.*`),
	}
}

func TestParseListing_variantA_parent(t *testing.T) {
	items := parseListing([]byte(parentListingHTML))
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	first := items[0]
	if first.id != "2104" {
		t.Errorf("ID = %q", first.id)
	}
	if first.title != "Teen babe intense sensual fuck & more" {
		t.Errorf("Title = %q (entity unescape failed?)", first.title)
	}
	if first.series != "Teeny Lovers" {
		t.Errorf("Series = %q (should come from <span class=\"author\">)", first.series)
	}
	if first.duration != 22*60+53 {
		t.Errorf("Duration = %d", first.duration)
	}
	if first.thumb != "https://cdn2.18videoz.com/images/wtl1602-1.jpg" {
		t.Errorf("Thumb = %q", first.thumb)
	}
	if items[1].series != "Casual Teen Sex" {
		t.Errorf("Second series = %q", items[1].series)
	}
}

func TestParseListing_variantB_rich(t *testing.T) {
	items := parseListing([]byte(richListingHTML))
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}

	// First card: casualteensex variant — title from <p class="d1">,
	// description from id="descN", duration from id="timeN" "HH:MM / MM:SS".
	cts := items[0]
	if cts.id != "539" {
		t.Errorf("CTS ID = %q", cts.id)
	}
	if cts.title != "Asslick leads to hot anal sex" {
		t.Errorf("CTS title = %q", cts.title)
	}
	if !strings.HasPrefix(cts.description, "Long description body") {
		t.Errorf("CTS description = %q", cts.description)
	}
	if cts.duration != 28*60+27 {
		t.Errorf("CTS duration = %d", cts.duration)
	}
	if cts.thumb != "https://cdn2.casualteensex.com/tour/images/562/562-5.jpg" {
		t.Errorf("CTS thumb = %q", cts.thumb)
	}

	// Second card: teensanalyzed variant — title from hidden div (most
	// authoritative), description from id="descN", detail URL captured.
	ta := items[1]
	if ta.id != "174" {
		t.Errorf("TA ID = %q", ta.id)
	}
	if ta.title != "A smooth first anal" {
		t.Errorf("TA title = %q (should come from hidden id=\"title174\")", ta.title)
	}
	if ta.detailPath != "/index.php/main/show_sets/174/0" {
		t.Errorf("TA detailPath = %q", ta.detailPath)
	}
	if ta.duration != 24*60+5 {
		t.Errorf("TA duration = %d", ta.duration)
	}
}

func TestParseListing_variantC_thumb(t *testing.T) {
	items := parseListing([]byte(thumbListingHTML))
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	yl := items[0]
	if yl.id != "286" {
		t.Errorf("YL ID = %q", yl.id)
	}
	// Variant C cards have no title field — the parser synthesises one
	// from the first sentence of the description. The fixture's
	// description is a single sentence longer than maxLen, so the
	// synthesised title falls back to a word-boundary truncation with `…`.
	if !strings.HasPrefix(yl.title, "When Ania told me") || !strings.HasSuffix(yl.title, "…") {
		t.Errorf("YL title = %q (expected synthesised title)", yl.title)
	}
	if !strings.HasPrefix(yl.description, "When Ania told me") {
		t.Errorf("YL description = %q", yl.description)
	}
	if yl.duration != 17*60+43 {
		t.Errorf("YL duration = %d", yl.duration)
	}
	if yl.thumb != "https://cdn2.younglibertines.com/pictures/286/286-1.jpg" {
		t.Errorf("YL thumb = %q", yl.thumb)
	}
}

func TestParseListing_variantD_table(t *testing.T) {
	items := parseListing([]byte(tableListingHTML))
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}

	first := items[0]
	if first.id != "092" {
		t.Errorf("D ID = %q, want 092 (from images/092-1.jpg)", first.id)
	}
	if first.title != "Deep and passionate anal" {
		t.Errorf("D title = %q", first.title)
	}
	if !strings.HasPrefix(first.description, "Molly loves it when her boyfriend") {
		t.Errorf("D description = %q", first.description)
	}
	if first.thumb != "images/092-1.jpg" {
		t.Errorf("D thumb = %q (expected root-relative path; toScene will absolutise)", first.thumb)
	}

	second := items[1]
	if second.id != "093" {
		t.Errorf("D Second ID = %q, want 093", second.id)
	}
	if second.title != "Assfucked in pink stockings" {
		t.Errorf("D Second title = %q", second.title)
	}
}

func TestEstimateTotal_variantD(t *testing.T) {
	// Fixture's pagination links point at index1.htm + index3.htm → max 3
	// × 2 cards = 6.
	got := estimateTotal([]byte(tableListingHTML), 2)
	if got != 6 {
		t.Errorf("estimateTotal(variant D) = %d, want 6", got)
	}
}

func TestToScene_variantD_absolutisesThumb(t *testing.T) {
	cfg := SiteConfig{
		ID: "bangmyteenass", SiteBase: "http://bangmyteenass.com",
		SiteName: "Bang My Teen Ass",
	}
	s := New(cfg)
	item := sceneItem{id: "92", title: "x", thumb: "images/092-1.jpg"}
	scene := s.toScene(item, cfg.SiteBase+"/", testNow())
	if scene.Thumbnail != "http://bangmyteenass.com/images/092-1.jpg" {
		t.Errorf("Thumbnail = %q (expected scheme+host prefix)", scene.Thumbnail)
	}
}

func TestListingURL_variantD(t *testing.T) {
	s := New(SiteConfig{
		ID: "bangmyteenass", SiteBase: "http://bangmyteenass.com",
		PaginationPath: "/index{N}.htm",
	})
	// Page 1 = bare homepage.
	if got := s.listingURL(1); got != "http://bangmyteenass.com/" {
		t.Errorf("variant D page 1 → %q", got)
	}
	// Page 5 = /index5.htm.
	if got := s.listingURL(5); got != "http://bangmyteenass.com/index5.htm" {
		t.Errorf("variant D page 5 → %q", got)
	}
}

func TestParseListing_dedupes(t *testing.T) {
	doubled := parentListingHTML + parentListingHTML
	items := parseListing([]byte(doubled))
	if len(items) != 2 {
		t.Errorf("got %d items after dedup, want 2", len(items))
	}
}

func TestEstimateTotal(t *testing.T) {
	// pagination block has pages 1, 9 → max 9 × 2 cards = 18.
	got := estimateTotal([]byte(parentListingHTML), 2)
	if got != 18 {
		t.Errorf("estimateTotal = %d, want 18", got)
	}
	// No /show_sets/N anywhere → just perPage.
	if got := estimateTotal([]byte(emptyHTML), 1); got != 1 {
		t.Errorf("estimateTotal(empty) = %d, want 1", got)
	}
}

func TestListingURL(t *testing.T) {
	// Paginated.
	s := New(parentConfig("https://18videoz.com"))
	if got := s.listingURL(1); got != "https://18videoz.com/index.php/main/show_sets2/1" {
		t.Errorf("paginated page 1 → %q", got)
	}
	if got := s.listingURL(7); got != "https://18videoz.com/index.php/main/show_sets2/7" {
		t.Errorf("paginated page 7 → %q", got)
	}
	// Single-page (teensanalyzed style).
	sp := New(SiteConfig{ID: "x", SiteBase: "https://x.example"})
	if got := sp.listingURL(1); got != "https://x.example/" {
		t.Errorf("single-page page 1 → %q", got)
	}
	if got := sp.listingURL(5); got != "https://x.example/" {
		t.Errorf("single-page page 5 → %q (should still be /)", got)
	}
}

func TestMatchesURL(t *testing.T) {
	s := New(SiteConfig{
		MatchRe: regexp.MustCompile(`^https?://(?:www\.)?18videoz\.com`),
	})
	if !s.MatchesURL("https://18videoz.com/") {
		t.Error("should match 18videoz.com")
	}
	if s.MatchesURL("https://teenylovers.com/") {
		t.Error("should not match teenylovers.com (different scraper)")
	}
}

func TestSitesTable_uniqueIDsAndDomainIsolation(t *testing.T) {
	seen := map[string]bool{}
	for _, cfg := range sites {
		if seen[cfg.ID] {
			t.Errorf("duplicate ID: %q", cfg.ID)
		}
		seen[cfg.ID] = true
		// Each MatchRe must reject every OTHER site's canonical URL.
		for _, other := range sites {
			if other.ID == cfg.ID {
				continue
			}
			otherURL := other.SiteBase + "/"
			if cfg.MatchRe.MatchString(otherURL) {
				t.Errorf("site %q.MatchRe matched %s (should not)", cfg.ID, otherURL)
			}
		}
		if !cfg.MatchRe.MatchString(cfg.SiteBase + "/") {
			t.Errorf("site %q.MatchRe does not match its own SiteBase", cfg.ID)
		}
	}
	if len(sites) != 10 {
		t.Errorf("expected 10 sites, got %d", len(sites))
	}
}

func TestToScene_parentUsesPerCardSeries(t *testing.T) {
	// Parent config has SiteName="", per-card author overrides.
	s := New(parentConfig("https://18videoz.com"))
	item := sceneItem{id: "1", title: "x", series: "Teeny Lovers"}
	scene := s.toScene(item, "https://18videoz.com/", testNow())
	if scene.Series != "Teeny Lovers" {
		t.Errorf("Series = %q, want %q", scene.Series, "Teeny Lovers")
	}
}

func TestToScene_childFallsBackToSiteName(t *testing.T) {
	cfg := SiteConfig{
		ID: "casualteensex", SiteBase: "https://casualteensex.com",
		SiteName: "Casual Teen Sex",
	}
	s := New(cfg)
	// Child card has no <span class="author"> — item.series is empty.
	item := sceneItem{id: "1", title: "x"}
	scene := s.toScene(item, "https://casualteensex.com/", testNow())
	if scene.Series != "Casual Teen Sex" {
		t.Errorf("Series = %q, want fallback %q", scene.Series, "Casual Teen Sex")
	}
}

func TestToScene_detailPathPreferred(t *testing.T) {
	s := New(parentConfig("https://x.example"))
	item := sceneItem{id: "174", title: "x", detailPath: "/index.php/main/show_sets/174/0"}
	scene := s.toScene(item, "https://x.example/", testNow())
	if scene.URL != "https://x.example/index.php/main/show_sets/174/0" {
		t.Errorf("URL = %q (detail path should be preferred over synthesised anchor)", scene.URL)
	}
	// Without detail path: synthesise the anchor.
	bare := sceneItem{id: "999", title: "x"}
	bareScene := s.toScene(bare, "https://x.example/", testNow())
	if bareScene.URL != "https://x.example/#scene-999" {
		t.Errorf("URL = %q (no detail path, should synthesise)", bareScene.URL)
	}
}

func TestListScenes_endToEnd(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		switch r.URL.Path {
		case "/index.php/main/show_sets2/1":
			_, _ = fmt.Fprint(w, parentListingHTML)
		default:
			_, _ = fmt.Fprint(w, emptyHTML)
		}
	}))
	defer ts.Close()

	s := New(SiteConfig{
		ID: "18videoz", SiteBase: ts.URL,
		PaginationPath: "/index.php/main/show_sets2",
		MatchRe:        regexp.MustCompile(`.*`),
	})
	ch, err := s.ListScenes(context.Background(), ts.URL+"/", scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}
	var scenes int
	for r := range ch {
		if r.Kind == scraper.KindScene {
			scenes++
			if r.Scene.Studio != "18videoz" {
				t.Errorf("Studio = %q", r.Scene.Studio)
			}
		}
	}
	if scenes != 2 {
		t.Errorf("got %d scenes, want 2", scenes)
	}
}

func TestListScenes_singlePage_stopsAfterOneFetch(t *testing.T) {
	hits := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, richListingHTML)
	}))
	defer ts.Close()

	// PaginationPath empty → single-page mode.
	s := New(SiteConfig{
		ID: "teensanalyzed", SiteBase: ts.URL, SiteName: "Teens Analyzed",
		MatchRe: regexp.MustCompile(`.*`),
	})
	ch, err := s.ListScenes(context.Background(), ts.URL+"/", scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}
	for range ch {
	}
	if hits != 1 {
		t.Errorf("single-page mode should fetch exactly once, got %d hits", hits)
	}
}

func TestListScenes_knownIDsStopsEarly(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		if r.URL.Path == "/index.php/main/show_sets2/1" {
			_, _ = fmt.Fprint(w, parentListingHTML)
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	s := New(SiteConfig{
		ID: "18videoz", SiteBase: ts.URL,
		PaginationPath: "/index.php/main/show_sets2",
		MatchRe:        regexp.MustCompile(`.*`),
	})
	ch, err := s.ListScenes(context.Background(), ts.URL+"/", scraper.ListOpts{
		KnownIDs: map[string]bool{"2220": true},
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
		t.Errorf("scenes=%d stoppedEarly=%v, want 1 and true", scenes, stoppedEarly)
	}
}

func testNow() time.Time { return time.Date(2026, 5, 29, 0, 0, 0, 0, time.UTC) }
