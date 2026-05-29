package carnalplus

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/Wasylq/FSS/scraper"
)

// ---- VariantNATS fixture (funsizeboys-style) ----

const natsListingHTML = `<html><body>
<div class="row videoBlock">

<div class="col-12 col-xs-6 col-sm-6 col-md-6 col-lg-4 col-xl-4 sceneTourBase" data-setid="8458" >
  <a href="https://funsizeboys.com/videos/workday-distraction-kai-vol-1.html" class="control_thumb">
    <div class="test thumbVid">
      <video class="img-fluid video-preview" poster="https://imagecdn.example/contentthumbs/25/77/72577-1x.jpg" muted loop>
        <source src="https://secure-cloud-cdn.example/fsb0203/thumbclip/thumbclip.mp4" type="video/mp4">
      </video>
      <div class="updateInfos">
        <div class="updateSite"><img src="https://cdn.example/minilogo/funsizeboys.png" alt="funsizeboys minilogo"></div>
        <div class="updateDetails">
          <h4>kai vol. 1 | workday distraction &amp; more</h4>
          <span><div>Starring Kai Neolani and Legrand Wolf</div></span>
        </div>
      </div>
    </div>
  </a>
</div>

<div class="col-12 col-xs-6 col-sm-6 col-md-6 col-lg-4 col-xl-4 sceneTourBase" data-setid="8450" >
  <a href="https://funsizeboys.com/videos/little-guy.html" class="control_thumb">
    <div class="test thumbVid">
      <video poster="https://imagecdn.example/contentthumbs/24/58/72458-1x.jpg">
        <source src="https://secure-cloud-cdn.example/fsb0202/thumbclip/thumbclip.mp4">
      </video>
      <div class="updateInfos">
        <div class="updateDetails">
          <h4>little guy big room</h4>
          <span><div>Starring Milo Eros, Sage Reeves &amp; Joe Doe</div></span>
        </div>
      </div>
    </div>
  </a>
</div>

</div>

<a href="movies_2_d.html">2</a>
<a href="movies_5_d.html">5</a>
</body></html>`

// ---- VariantGrid fixture (carnalplus.com style) ----

const gridListingHTML = `<html><body>
<div class="grid-item-eight ">
  <div class="swiper-slide">
    <a title="Watch caught-scout-marcus-vol-2"
       href="https://carnalplus.com/videos/caught-scout-marcus-vol-2.html"
       class="control_thumb">
      <picture data-iesrc="https://imagecdn.example/contentthumbs/25/91/72591-1x.jpg">
        <img loading="lazy" class="img-fluid"
             data-video-src="https://secure-cloud-cdn.example/sbs0152/thumbclip/thumbclip.mp4"
             src="https://imagecdn.example/contentthumbs/25/91/72591-1x.jpg" alt="">
      </picture>
    </a>
    <div class="updateInfos">
      <div class="updateSite">
        <a href="scoutboys/">
          <img src="https://cdn.example/minilogo/scoutboys.png" alt="scoutboys minilogo">
        </a>
      </div>
      <div class="updateDetails">
        <h4 class="capitalize titleBlockF">
          <span class='update-series'>Scout marcus vol. 2</span> |
          <span class='update-title'>caught by the scoutmasters</span>
        </h4>
        <div class='update-sitename'>
          Starring <a href="x">Marcus Daniels</a>, <a href="y">Bishop Angus</a>
        </div>
      </div>
    </div>
  </div>
</div>

<div class="grid-item-eight">
  <a href="https://carnalplus.com/videos/altar-boy-dex-vol-2.html" class="control_thumb">
    <picture>
      <img src="https://imagecdn.example/contentthumbs/26/04/72604-1x.jpg"
           class="img-fluid" data-video-src="https://secure-cloud-cdn.example/cb0203/thumbclip/thumbclip.mp4" alt="">
    </picture>
  </a>
  <div class="updateInfos">
    <div class="updateSite">
      <img src="https://cdn.example/minilogo/catholicboys.png" alt="catholicboys minilogo">
    </div>
    <div class="updateDetails">
      <h4>
        <span class='update-series'>Altar boy dex vol. 2</span> |
        <span class='update-title'>altar boy training</span>
      </h4>
    </div>
  </div>
</div>
</body></html>`

// ---- VariantWordPress fixture (growlboys style) ----

const wpListingJSON = `[
  {
    "id": 17,
    "date_gmt": "2026-05-20T01:18:21",
    "slug": "head-games",
    "link": "https://growlboys.com/head-games/",
    "title": {"rendered": "HEAD GAMES"},
    "content": {"rendered": "<p>A growl scene description.</p>"},
    "_embedded": {
      "wp:featuredmedia": [{"source_url": "https://growlboys.com/wp-content/thumb.webp"}],
      "wp:term": [
        [{"name": "Uncategorized", "taxonomy": "category"}],
        [{"name": "Performer A", "taxonomy": "post_tag"}, {"name": "Performer B", "taxonomy": "post_tag"}]
      ]
    }
  },
  {
    "id": 15,
    "date_gmt": "2026-04-10T00:00:00",
    "slug": "second-scene",
    "link": "https://growlboys.com/second-scene/",
    "title": {"rendered": "Second Scene"},
    "content": {"rendered": ""},
    "_embedded": {}
  }
]`

func TestParseNATSListing(t *testing.T) {
	items := parseNATSListing([]byte(natsListingHTML))
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	first := items[0]
	if first.id != "8458" {
		t.Errorf("NATS ID = %q", first.id)
	}
	if first.title != "kai vol. 1 | workday distraction & more" {
		t.Errorf("NATS title = %q (entity unescape failed?)", first.title)
	}
	if first.url != "https://funsizeboys.com/videos/workday-distraction-kai-vol-1.html" {
		t.Errorf("NATS URL = %q", first.url)
	}
	if first.thumb != "https://imagecdn.example/contentthumbs/25/77/72577-1x.jpg" {
		t.Errorf("NATS thumb = %q", first.thumb)
	}
	if first.preview != "https://secure-cloud-cdn.example/fsb0203/thumbclip/thumbclip.mp4" {
		t.Errorf("NATS preview = %q", first.preview)
	}
	if len(first.performers) != 2 || first.performers[0] != "Kai Neolani" || first.performers[1] != "Legrand Wolf" {
		t.Errorf("NATS performers = %v", first.performers)
	}

	// Second card: 3 performers via "comma + ampersand" split.
	second := items[1]
	if len(second.performers) != 3 {
		t.Errorf("NATS Second performers = %v (want 3)", second.performers)
	}
}

func TestParseGridListing(t *testing.T) {
	items := parseGridListing([]byte(gridListingHTML))
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	first := items[0]
	if first.id != "72591" {
		t.Errorf("Grid ID = %q (expected 72591 from contentthumbs/25/91/72591-1x)", first.id)
	}
	if first.url != "https://carnalplus.com/videos/caught-scout-marcus-vol-2.html" {
		t.Errorf("Grid URL = %q", first.url)
	}
	if first.title != "caught by the scoutmasters" {
		t.Errorf("Grid title = %q", first.title)
	}
	if first.series != "Scout Boys" {
		t.Errorf("Grid series = %q (expected human-readable subsite name from alt)", first.series)
	}
	if first.preview != "https://secure-cloud-cdn.example/sbs0152/thumbclip/thumbclip.mp4" {
		t.Errorf("Grid preview = %q", first.preview)
	}

	second := items[1]
	if second.id != "72604" {
		t.Errorf("Grid Second ID = %q", second.id)
	}
	if second.series != "Catholic Boys" {
		t.Errorf("Grid Second series = %q", second.series)
	}
}

func TestParseWPListing(t *testing.T) {
	h := http.Header{}
	h.Set("X-WP-Total", "44")
	h.Set("X-WP-TotalPages", "1")
	items, total, totalPages := parseWPListing([]byte(wpListingJSON), h)
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	if total != 44 {
		t.Errorf("WP total = %d, want 44", total)
	}
	if totalPages != 1 {
		t.Errorf("WP totalPages = %d, want 1", totalPages)
	}
	first := items[0]
	if first.id != "17" {
		t.Errorf("WP ID = %q", first.id)
	}
	if first.title != "HEAD GAMES" {
		t.Errorf("WP title = %q", first.title)
	}
	if first.url != "https://growlboys.com/head-games/" {
		t.Errorf("WP URL = %q", first.url)
	}
	if first.thumb != "https://growlboys.com/wp-content/thumb.webp" {
		t.Errorf("WP thumb = %q", first.thumb)
	}
	if len(first.performers) != 2 {
		t.Errorf("WP performers = %v", first.performers)
	}
	if first.date.Year() != 2026 || first.date.Month() != 5 || first.date.Day() != 20 {
		t.Errorf("WP date = %v", first.date)
	}
}

func TestSplitPerformers(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"Kai Neolani and Legrand Wolf", []string{"Kai Neolani", "Legrand Wolf"}},
		{"A, B, C", []string{"A", "B", "C"}},
		{"A, B &amp; C", []string{"A", "B", "C"}},
		{"  ", nil},
	}
	for _, c := range cases {
		got := splitPerformers(c.in)
		if len(got) != len(c.want) {
			t.Errorf("splitPerformers(%q) = %v, want %v", c.in, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("splitPerformers(%q)[%d] = %q, want %q", c.in, i, got[i], c.want[i])
			}
		}
	}
}

func TestListingURL(t *testing.T) {
	// NATS: page 1 → "/" ; page N → /categories/movies_N_d.html.
	nats := New(SiteConfig{ID: "x", SiteBase: "https://x.example", Variant: VariantNATS})
	if got := nats.listingURL(1); got != "https://x.example/" {
		t.Errorf("NATS page 1 → %q", got)
	}
	if got := nats.listingURL(3); got != "https://x.example/categories/movies_3_d.html" {
		t.Errorf("NATS page 3 → %q", got)
	}
	// Grid root: page 1 → "/" ; page N → /?page=N.
	grid := New(SiteConfig{ID: "p", SiteBase: "https://carnalplus.com", Variant: VariantGrid})
	if got := grid.listingURL(1); got != "https://carnalplus.com/" {
		t.Errorf("Grid root page 1 → %q", got)
	}
	if got := grid.listingURL(2); got != "https://carnalplus.com/?page=2" {
		t.Errorf("Grid root page 2 → %q", got)
	}
	// Grid sub-path: page 1 → /baptistboys/ ; page N → /baptistboys/?page=N.
	sub := New(SiteConfig{
		ID: "bb", SiteBase: "https://carnalplus.com", SubPath: "/baptistboys",
		Variant: VariantGrid,
	})
	if got := sub.listingURL(1); got != "https://carnalplus.com/baptistboys/" {
		t.Errorf("Grid sub page 1 → %q", got)
	}
	if got := sub.listingURL(2); got != "https://carnalplus.com/baptistboys/?page=2" {
		t.Errorf("Grid sub page 2 → %q", got)
	}
	// WP.
	wp := New(SiteConfig{ID: "gb", SiteBase: "https://growlboys.com", Variant: VariantWordPress})
	if got := wp.listingURL(2); got != "https://growlboys.com/wp-json/wp/v2/posts?per_page=100&_embed=1&page=2" {
		t.Errorf("WP page 2 → %q", got)
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
		// NATS site
		{"funsizeboys", "https://funsizeboys.com/", true},
		{"funsizeboys", "https://www.funsizeboys.com/videos/x.html", true},
		{"funsizeboys", "https://other.com/", false},
		// Parent (only root + ?page=, NOT sub-paths)
		{"carnalplus", "https://carnalplus.com/", true},
		{"carnalplus", "https://carnalplus.com/?page=2", true},
		{"carnalplus", "https://carnalplus.com/baptistboys/", false},
		{"carnalplus", "https://carnalplus.com/carnaloriginals/", false},
		// Sub-brand
		{"baptistboys", "https://carnalplus.com/baptistboys/", true},
		{"baptistboys", "https://carnalplus.com/baptistboys/?page=2", true},
		{"baptistboys", "https://carnalplus.com/", false},
		// WP
		{"growlboys", "https://growlboys.com/", true},
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

func TestListScenes_NATSendToEnd(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		switch r.URL.Path {
		case "/":
			_, _ = fmt.Fprint(w, natsListingHTML)
		default:
			_, _ = fmt.Fprint(w, "<html></html>")
		}
	}))
	defer ts.Close()

	s := New(SiteConfig{
		ID: "funsizeboys", SiteBase: ts.URL, SiteName: "Fun-Size Boys",
		Variant: VariantNATS,
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
			if r.Scene.Studio != "Carnal+" {
				t.Errorf("Studio = %q", r.Scene.Studio)
			}
		}
	}
	if scenes != 2 {
		t.Errorf("got %d scenes, want 2", scenes)
	}
}

func TestListScenes_WordPressEndToEnd(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		page := r.URL.Query().Get("page")
		if page == "1" {
			w.Header().Set("X-WP-Total", "2")
			w.Header().Set("X-WP-TotalPages", "1")
			_, _ = fmt.Fprint(w, wpListingJSON)
		} else {
			_, _ = fmt.Fprint(w, `[]`)
		}
	}))
	defer ts.Close()

	s := New(SiteConfig{
		ID: "growlboys", SiteBase: ts.URL, SiteName: "GrowlBoys",
		Variant: VariantWordPress,
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
			if r.Scene.Series != "GrowlBoys" {
				t.Errorf("Series = %q (fallback to SiteName failed?)", r.Scene.Series)
			}
		}
	}
	if scenes != 2 {
		t.Errorf("got %d scenes, want 2", scenes)
	}
}
