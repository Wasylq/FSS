package dirtyflix

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/Wasylq/FSS/scraper"
)

// ---- VariantThumbsItem fixture (dirtyflix parent) ----

const thumbsItemListingHTML = `<html><body>

<div class="thumbs-item" >
  <div class="thumb">
    <a class="thumb-img" onclick="re_add_click_old('465');">
      <img class="im0" src="https://cdn3.dirtyflix.com/images/tbx115-17.jpg">
      <div class="left pic1"><span class="duration pic1">28:25</span></div>
      <div class="right pic4"><span class="resolution  pic4">4k</span></div>
    </a>
    <span class="info2">
      <a class="title">Stepsister thoroughly fucked &amp; loved it</a>
      <p>
        <span class="from">Scene from </span>
        <a class="link">Brutal X</a>
      </p>
    </span>
  </div>
</div>

<div class="thumbs-item" >
  <div class="thumb">
    <a class="thumb-img" onclick="re_add_click_old('1030');">
      <img class="im0" src="https://cdn3.dirtyflix.com/images/kfm047-2.jpg">
      <span class="duration">32:57</span>
    </a>
    <span class="info2">
      <a class="title">Daddy-stepdaughter shenanigans</a>
      <p>
        <span class="from">Scene from </span>
        <a class="link">Kinky Family</a>
      </p>
    </span>
  </div>
</div>

</body></html>`

// ---- VariantBrutalX fixture ----

const brutalXListingHTML = `<html><body>

<div id="354" style="z-index:1;" class="th" onclick="click_me('354');">
  <a>
    <span class="thumb_img">
      <img src="/images/spacer2.gif" class="spacer">
      <img src="https://cdn2.brutalx.com/content/thumbs/tbx254new-1.jpg" class="thumb-img">
      <span class="size">4k</span>
    </span>
    <span class="caption">
      <span class="duration"><em>29:52</em><i class="ico_play"></i></span>
      <h3 class="title_thumb">Fuck-schooled by horny stepdad &amp; friends</h3>
    </span>
  </a>
</div>

<div id="360" class="th" onclick="click_me('360');">
  <a>
    <span class="thumb_img">
      <img src="https://cdn2.brutalx.com/content/thumbs/tbx331c-1.jpg" class="thumb-img">
      <span class="size">hd</span>
    </span>
    <span class="caption">
      <span class="duration"><em>26:28</em></span>
      <h3 class="title_thumb">Good fuck for a slutty stepsis</h3>
    </span>
  </a>
</div>

<a href="/index.php/main/show_sets2/5">5</a>
</body></html>`

// ---- VariantThumbWrap fixtures: caption-text + caption-h3 flavours ----

const thumbWrapKinkyHTML = `<html><body>
<a class="thumb_wrap" onclick="re_add_click('238', '2', 'daddy-shenanigans');">
  <span class="wrap_image">
    <img src="/images/spacer3.gif" class="spacer">
    <img src="https://cdn2.kinkyfamily.com/content/thumbs/twkf047-2.jpg" class="thumb-img">
  </span>
  <span class="tools">
    <span class="caption">Daddy-stepdaughter shenanigans &amp; more</span>
    <span class="sub">
      <span class="box_view">
        <span class="item_box"><i class="icon-clock"></i><em>32:57</em></span>
      </span>
    </span>
  </span>
</a>
</body></html>`

const thumbWrapXSensualHTML = `<html><body>
<a class="thumb_wrap" onclick="re_add_click('484', 'every-girl-is-a-model');">
  <span class="thumbnail-img">
    <img src="https://cdn2.x-sensual.com/images/wxs1299-9.jpg">
    <span class="hd">4k</span>
  </span>
  <span class="caption">
    <span class="duration">25:03</span>
    <h3>Every girl is a model</h3>
  </span>
</a>
</body></html>`

func TestParse_VariantThumbsItem(t *testing.T) {
	items := parseListing([]byte(thumbsItemListingHTML), VariantThumbsItem)
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	first := items[0]
	if first.id != "465" {
		t.Errorf("ID = %q", first.id)
	}
	if first.title != "Stepsister thoroughly fucked & loved it" {
		t.Errorf("Title = %q", first.title)
	}
	if first.series != "Brutal X" {
		t.Errorf("Series = %q", first.series)
	}
	if first.duration != 28*60+25 {
		t.Errorf("Duration = %d", first.duration)
	}
}

func TestParse_VariantBrutalX(t *testing.T) {
	items := parseListing([]byte(brutalXListingHTML), VariantBrutalX)
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	first := items[0]
	if first.id != "354" {
		t.Errorf("ID = %q (expected outer-div id attribute)", first.id)
	}
	if first.title != "Fuck-schooled by horny stepdad & friends" {
		t.Errorf("Title = %q (entity unescape failed?)", first.title)
	}
	if first.duration != 29*60+52 {
		t.Errorf("Duration = %d", first.duration)
	}
	if first.resolution != "4k" {
		t.Errorf("Resolution = %q", first.resolution)
	}
	if first.thumb != "https://cdn2.brutalx.com/content/thumbs/tbx254new-1.jpg" {
		t.Errorf("Thumb = %q (should skip the spacer.gif)", first.thumb)
	}
}

func TestParse_VariantThumbWrap_kinky(t *testing.T) {
	items := parseListing([]byte(thumbWrapKinkyHTML), VariantThumbWrap)
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	it := items[0]
	if it.id != "238" {
		t.Errorf("ID = %q", it.id)
	}
	if it.title != "Daddy-stepdaughter shenanigans & more" {
		t.Errorf("Title = %q (caption-text flavour)", it.title)
	}
	if it.duration != 32*60+57 {
		t.Errorf("Duration = %d (expected item_box em form)", it.duration)
	}
	if !strings.Contains(it.thumb, "twkf047-2.jpg") {
		t.Errorf("Thumb = %q (should skip the spacer.gif)", it.thumb)
	}
}

func TestParse_VariantThumbWrap_xsensual(t *testing.T) {
	items := parseListing([]byte(thumbWrapXSensualHTML), VariantThumbWrap)
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	it := items[0]
	if it.id != "484" {
		t.Errorf("ID = %q", it.id)
	}
	if it.title != "Every girl is a model" {
		t.Errorf("Title = %q (expected h3-in-caption flavour)", it.title)
	}
	if it.duration != 25*60+3 {
		t.Errorf("Duration = %d (expected bare span form)", it.duration)
	}
	if it.resolution != "4k" {
		t.Errorf("Resolution = %q", it.resolution)
	}
}

func TestEstimateTotal_picksHighestShowSetsLink(t *testing.T) {
	got := estimateTotal([]byte(brutalXListingHTML), 2)
	if got != 10 {
		t.Errorf("estimateTotal = %d, want 10 (page 5 × 2 cards)", got)
	}
}

func TestListingURL(t *testing.T) {
	// Single-page (parent).
	parent := New(SiteConfig{
		ID: "dirtyflix", SiteBase: "https://dirtyflix.com",
		Paginated: false,
	})
	if got := parent.listingURL(1); got != "https://dirtyflix.com/" {
		t.Errorf("parent page 1 → %q", got)
	}
	if got := parent.listingURL(5); got != "https://dirtyflix.com/" {
		t.Errorf("parent page 5 → %q (single-page should always return /)", got)
	}
	// Paginated.
	child := New(SiteConfig{
		ID: "brutalx", SiteBase: "https://brutalx.com",
		Paginated: true,
	})
	if got := child.listingURL(1); got != "https://brutalx.com/" {
		t.Errorf("paginated page 1 → %q", got)
	}
	if got := child.listingURL(3); got != "https://brutalx.com/index.php/main/show_sets2/3" {
		t.Errorf("paginated page 3 → %q", got)
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
		{"dirtyflix", "https://dirtyflix.com/", true},
		{"dirtyflix", "https://brutalx.com/", false},
		{"brutalx", "https://brutalx.com/", true},
		{"kinkyfamily", "https://kinkyfamily.com/", true},
		{"xsensual", "https://x-sensual.com/", true},
		{"privatecastingx", "https://privatecasting-x.com/", true},
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
	if len(sites) != 16 {
		t.Errorf("expected 16 sites, got %d", len(sites))
	}
}

func TestListScenes_parentEndToEnd(t *testing.T) {
	hits := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, thumbsItemListingHTML)
	}))
	defer ts.Close()

	s := New(SiteConfig{
		ID: "dirtyflix", SiteBase: ts.URL, Variant: VariantThumbsItem,
		Paginated: false, MatchRe: regexp.MustCompile(`.*`),
	})
	ch, err := s.ListScenes(context.Background(), ts.URL+"/", scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}
	var scenes int
	for r := range ch {
		if r.Kind == scraper.KindScene {
			scenes++
		}
	}
	if scenes != 2 {
		t.Errorf("got %d scenes, want 2", scenes)
	}
	if hits != 1 {
		t.Errorf("single-page should fetch once, got %d hits", hits)
	}
}

func TestListScenes_paginatedStopsOnEmpty(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		switch r.URL.Path {
		case "/":
			_, _ = fmt.Fprint(w, brutalXListingHTML)
		default:
			_, _ = fmt.Fprint(w, "<html></html>")
		}
	}))
	defer ts.Close()

	s := New(SiteConfig{
		ID: "brutalx", SiteBase: ts.URL, Variant: VariantBrutalX,
		Paginated: true, MatchRe: regexp.MustCompile(`.*`),
	})
	ch, err := s.ListScenes(context.Background(), ts.URL+"/", scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}
	var scenes int
	for r := range ch {
		if r.Kind == scraper.KindScene {
			scenes++
		}
	}
	if scenes != 2 {
		t.Errorf("got %d scenes, want 2", scenes)
	}
}
