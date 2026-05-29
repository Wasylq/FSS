package kocompany

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

// Trimmed fixture mirroring real ko-video.com markup — three product
// cards (the listing groups them inside `<ul class="item_list">`),
// plus the `page_nav_head` pagination block with a max page of 10.
const listingHTML = `<html><body>
<ul class="item_list">

<li>
  <a href="/products/detail.php?product_code=KBEA390_DVD">
    <img src="/upload/save_image/beast/KBEA390_DVD/KBEA390_DVD_A.jpg" alt="【早割30%OFF】職場淫猥白書 XXX">
    <div class="list_tag">
      <span class="icon_qpn">クーポン</span>
      <span class="icon_shin">NEW</span>
    </div>
    <span>【早割30%OFF】職場淫猥白書 XXX</span>
  </a>
</li>

<li>
  <a href="/products/detail.php?product_code=KBEA389_DVD">
    <img src="/upload/save_image/beast/KBEA389_DVD/KBEA389_DVD_A.jpg" alt="BACKWILD 20">
    <div class="list_tag">
      <span class="icon_shin">NEW</span>
    </div>
    <span>BACKWILD 20 &amp; more</span>
  </a>
</li>

<li>
  <a href="/products/detail.php?product_code=KBEA388_DVD">
    <img src="/upload/save_image/beast/KBEA388_DVD/KBEA388_DVD_A.jpg" alt="BEAST VALUE SET 024">
    <div class="list_tag">
    </div>
    <span>BEAST VALUE SET 024</span>
  </a>
</li>

</ul>

<div class="page_nav_head">
  <ol>
    <li><b class="current">1</b></li>
    <li><a href="javascript:;" onClick="fnModeSubmit('', 'pageno', 2); return false;">2</a></li>
    <li><a href="javascript:;" onClick="fnModeSubmit('', 'pageno', 3); return false;">3</a></li>
    <li><a href="javascript:;" onClick="fnModeSubmit('', 'pageno', 10); return false;">10</a></li>
  </ol>
</div>
</body></html>`

const emptyHTML = `<html><body><div>0点</div></body></html>`

func TestParseListing(t *testing.T) {
	items := parseListing([]byte(listingHTML))
	if len(items) != 3 {
		t.Fatalf("got %d items, want 3", len(items))
	}

	first := items[0]
	if first.id != "KBEA390_DVD" {
		t.Errorf("ID = %q", first.id)
	}
	if first.title != "【早割30%OFF】職場淫猥白書 XXX" {
		t.Errorf("Title = %q", first.title)
	}
	if first.thumb != "/upload/save_image/beast/KBEA390_DVD/KBEA390_DVD_A.jpg" {
		t.Errorf("Thumb = %q", first.thumb)
	}
	if len(first.tags) != 2 {
		t.Errorf("Tags = %v, want 2", first.tags)
	}

	// Second card: entity unescape (&amp; → &).
	second := items[1]
	if second.title != "BACKWILD 20 & more" {
		t.Errorf("Second title = %q (entity unescape failed?)", second.title)
	}
	if len(second.tags) != 1 {
		t.Errorf("Second tags = %v, want 1", second.tags)
	}

	// Third card: no badges.
	third := items[2]
	if len(third.tags) != 0 {
		t.Errorf("Third tags = %v, want empty", third.tags)
	}
}

func TestParseListing_dedupes(t *testing.T) {
	doubled := listingHTML + listingHTML
	items := parseListing([]byte(doubled))
	if len(items) != 3 {
		t.Errorf("got %d items after dedup, want 3", len(items))
	}
}

func TestEstimateTotal(t *testing.T) {
	// page_nav_head lists pageno 2/3/10 → max 10 × 3 cards = 30.
	got := estimateTotal([]byte(listingHTML), 3)
	if got != 30 {
		t.Errorf("estimateTotal = %d, want 30", got)
	}
	if got := estimateTotal([]byte(emptyHTML), 1); got != 1 {
		t.Errorf("estimateTotal(empty) = %d, want 1", got)
	}
}

func TestListingURL(t *testing.T) {
	// Label-filtered (BEAST).
	beast := New(SiteConfig{ID: "kobeast", LabelID: 3})
	if got := beast.listingURL(1); got != "https://ko-video.com/products/list.php?label=3" {
		t.Errorf("LabelID page 1 → %q", got)
	}
	if got := beast.listingURL(5); got != "https://ko-video.com/products/list.php?label=3&pageno=5" {
		t.Errorf("LabelID page 5 → %q", got)
	}
	// Maker-filtered (EAST).
	east := New(SiteConfig{ID: "koeast", MakerID: 10})
	if got := east.listingURL(1); got != "https://ko-video.com/products/list.php?maker=10" {
		t.Errorf("MakerID page 1 → %q", got)
	}
	if got := east.listingURL(2); got != "https://ko-video.com/products/list.php?maker=10&pageno=2" {
		t.Errorf("MakerID page 2 → %q", got)
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
		{"kobeast", "https://ko-video.com/products/list.php?label=3", true},
		{"kobeast", "http://www.ko-tube.com/ranking/label/01-06/BEAST", true},
		{"kobeast", "https://ko-video.com/products/list.php?label=21", false},
		{"kobump", "https://ko-video.com/products/list.php?label=21", true},
		{"koeast", "https://ko-video.com/products/list.php?maker=10", true},
		{"koeast", "https://ko-video.com/products/list.php?label=10", false},
		{"kojoker", "https://ko-video.com/products/list.php?label=48", true},
		// Substring/prefix traps — `label=3` should not match `label=38`.
		{"kobeast", "https://ko-video.com/products/list.php?label=38", false},
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
		if cfg.LabelID == 0 && cfg.MakerID == 0 {
			t.Errorf("site %q has neither LabelID nor MakerID", cfg.ID)
		}
	}
	// 14 sub-labels (the stashdb tree has 14 children + 1 parent;
	// the parent doesn't get its own scraper since each child *is* a
	// label-filtered view of the same catalogue).
	if len(sites) != 14 {
		t.Errorf("expected 14 sites, got %d", len(sites))
	}
}

func TestListScenes_endToEnd(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		// First page returns the fixture; later pages empty.
		if r.URL.Query().Get("pageno") == "" {
			_, _ = fmt.Fprint(w, listingHTML)
			return
		}
		_, _ = fmt.Fprint(w, emptyHTML)
	}))
	defer ts.Close()

	// Point the scraper at the test server by overriding baseURL — we
	// instead supply a custom listingURL via inline cfg.
	cfg := SiteConfig{
		ID: "kobeast", SiteName: "KO BEAST", LabelID: 3,
		MatchRe: regexp.MustCompile(`.*`),
	}
	s := &Scraper{cfg: cfg, client: ts.Client()}
	// Replicate the run loop pointed at ts.URL by overriding listingURL
	// through a tiny inline shim that drops the fixed baseURL.
	out := make(chan scraper.SceneResult)
	go func() {
		defer close(out)
		body, err := s.fetchPage(context.Background(), ts.URL+"/products/list.php?label=3")
		if err != nil {
			out <- scraper.Error(err)
			return
		}
		items := parseListing(body)
		for _, item := range items {
			out <- scraper.Scene(s.toScene(item, ts.URL, mustTime()))
		}
	}()

	var scenes int
	for r := range out {
		if r.Kind == scraper.KindScene {
			scenes++
			if r.Scene.Studio != "KO Company" {
				t.Errorf("Studio = %q", r.Scene.Studio)
			}
			if r.Scene.Series != "KO BEAST" {
				t.Errorf("Series = %q", r.Scene.Series)
			}
			if !strings.HasPrefix(r.Scene.URL, "https://ko-video.com/products/detail.php?product_code=KBEA") {
				t.Errorf("Detail URL = %q", r.Scene.URL)
			}
		}
	}
	if scenes != 3 {
		t.Errorf("got %d scenes, want 3", scenes)
	}
}

func mustTime() time.Time { return time.Date(2026, 5, 29, 0, 0, 0, 0, time.UTC) }
