package next11

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
	"github.com/Wasylq/FSS/scraper"
)

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url  string
		want bool
	}{
		{"https://next11.co.jp/", true},
		{"https://www.next11.co.jp/products/list.php", true},
		{"https://next11.co.jp/products/detail.php?product_id=123", true},
		{"https://example.com", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.want {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.want)
		}
	}
}

func TestParseListingPage(t *testing.T) {
	body := []byte(`<span class="pagenumber">42</span>件の商品がございます。
<div class="listphoto">
<a href="/products/detail.php?product_id=100" class="over"><img src="/upload/save_image/jacket/m_ABC-001.jpg" alt="Title One" class="picture" /></a>
</div>
<div class="listrightblock">
<table class="listproducts">
<tr><td>商品：</td><td class="center">ABC-001</td></tr>
</table>
</div>
</li>
<div class="listphoto">
<a href="/products/detail.php?product_id=200" class="over"><img src="/upload/save_image/jacket/m_DEF-002.jpg" alt="Title Two" class="picture" /></a>
</div>
<div class="listrightblock">
<table class="listproducts">
<tr><td>商品：</td><td class="center">DEF-002</td></tr>
</table>
</div>
</li>`)

	items, total := parseListingPage(body)
	if total != 42 {
		t.Errorf("total = %d, want 42", total)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	if items[0].productID != "100" || items[0].code != "ABC-001" {
		t.Errorf("items[0] = %+v", items[0])
	}
	if items[1].productID != "200" || items[1].code != "DEF-002" {
		t.Errorf("items[1] = %+v", items[1])
	}
}

const detailHTML = `<div id="listtitle"><h2>[<span itemprop="productID" content="DOKI-033">DOKI-033</span>] <span itemprop="name">Test Title</span></h2></div>
<div id="detail1">
<img src="/upload/save_image/jacket/DOKI-033.jpg" width="794" alt="Test Title" class="picture">
</div>
<dl>
<dt>出演：</dt>
<dd><a href="/products/list.php?category_id=4504">Performer One</a><a href="/products/list.php?category_id=4615">Performer Two</a></dd>
<dt>監督：</dt>
<dd>&nbsp;</dd>
<dt>ジャンル：</dt>
<dd>
<span itemprop="category" content="Genre1Genre2">
<a href="/products/list.php?category_id=14">Genre1</a><a href="/products/list.php?category_id=53">Genre2</a></span>
</dd>
<dt>レーベル：</dt>
<dd><a href="/products/list.php?category_id=4357">DOKI!</a></dd>
<dt>シリーズ：</dt>
<dd><a href="/products/list.php?category_id=1234">Test Series</a></dd>
<dt>収録時間：</dt>
<dd>130分</dd>
<dt>発売日：</dt>
<dd>2026-03-20</dd>
<dt>品番：</dt>
<dd>DOKI-033</dd>
</dl>
<div class="price" style="display:none;">販売価格(税込)：
<span style="font-size:120%;" itemprop="price">
		2,604
	円</span></div>`

func TestParseDetailPage(t *testing.T) {
	d := parseDetailPage([]byte(detailHTML))

	if d.title != "Test Title" {
		t.Errorf("title = %q", d.title)
	}
	if d.code != "DOKI-033" {
		t.Errorf("code = %q", d.code)
	}
	if d.date.Format("2006-01-02") != "2026-03-20" {
		t.Errorf("date = %v", d.date)
	}
	if d.duration != 7800 {
		t.Errorf("duration = %d, want 7800", d.duration)
	}
	if len(d.performers) != 2 || d.performers[0] != "Performer One" || d.performers[1] != "Performer Two" {
		t.Errorf("performers = %v", d.performers)
	}
	if len(d.tags) != 2 || d.tags[0] != "Genre1" || d.tags[1] != "Genre2" {
		t.Errorf("tags = %v", d.tags)
	}
	if d.label != "DOKI!" {
		t.Errorf("label = %q", d.label)
	}
	if d.series != "Test Series" {
		t.Errorf("series = %q", d.series)
	}
	if d.thumbnail != "/upload/save_image/jacket/DOKI-033.jpg" {
		t.Errorf("thumbnail = %q", d.thumbnail)
	}
	if d.price != 2604 {
		t.Errorf("price = %d", d.price)
	}
}

func TestParseDetailPageMissingFields(t *testing.T) {
	body := []byte(`<span itemprop="name">Title Only</span>`)
	d := parseDetailPage(body)
	if d.title != "Title Only" {
		t.Errorf("title = %q", d.title)
	}
	if !d.date.IsZero() {
		t.Errorf("expected zero date, got %v", d.date)
	}
	if len(d.performers) != 0 {
		t.Errorf("expected no performers, got %v", d.performers)
	}
}

func newTestServer(items []listingItem) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")

		switch r.URL.Path {
		case "/products/list.php":
			var listing string
			for _, item := range items {
				listing += fmt.Sprintf(`<div class="listphoto">
<a href="/products/detail.php?product_id=%s"><img/></a>
</div>
<div class="listrightblock">
<table><tr><td>商品：</td><td class="center">%s</td></tr></table>
</div>
</li>`, item.productID, item.code)
			}
			_, _ = fmt.Fprintf(w, `<span class="pagenumber">%d</span>件%s`, len(items), listing)
		default:
			_, _ = fmt.Fprint(w, detailHTML)
		}
	}))
}

func TestRun(t *testing.T) {
	items := []listingItem{
		{productID: "100", code: "ABC-001"},
		{productID: "200", code: "DEF-002"},
	}
	ts := newTestServer(items)
	defer ts.Close()

	s := &Scraper{client: ts.Client(), siteBase: ts.URL}
	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{Workers: 1})
	if err != nil {
		t.Fatal(err)
	}

	scenes := testutil.CollectScenes(t, ch)
	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2", len(scenes))
	}

	for _, sc := range scenes {
		if sc.SiteID != siteID {
			t.Errorf("SiteID = %q", sc.SiteID)
		}
		if sc.Studio != studioName {
			t.Errorf("Studio = %q", sc.Studio)
		}
		if sc.Title == "" {
			t.Error("empty title")
		}
	}
}

func TestRunKnownIDs(t *testing.T) {
	items := []listingItem{
		{productID: "100", code: "ABC-001"},
		{productID: "200", code: "DEF-002"},
		{productID: "300", code: "GHI-003"},
	}
	ts := newTestServer(items)
	defer ts.Close()

	s := &Scraper{client: ts.Client(), siteBase: ts.URL}
	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{
		Workers:  1,
		KnownIDs: map[string]bool{"DEF-002": true},
	})
	if err != nil {
		t.Fatal(err)
	}

	scenes, stoppedEarly := testutil.CollectScenesWithStop(t, ch)
	if !stoppedEarly {
		t.Error("expected StoppedEarly signal")
	}
	if len(scenes) != 1 {
		t.Fatalf("got %d scenes, want 1", len(scenes))
	}
}
