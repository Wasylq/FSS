package next11

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
	"github.com/Anastylosis/FSS/scraper"
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

// newTestServer serves the product listing, paginating `pages` the way the real
// site does: by the POSTed form1 fields, NOT by the query string. A server that
// ignored pageno could not tell a working scraper from the broken one this
// replaced, which walked every page and got page 1 back each time.
func newTestServer(pages [][]listingItem) *httptest.Server {
	total := 0
	for _, p := range pages {
		total += len(p)
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")

		switch r.URL.Path {
		case "/products/list.php":
			if r.Method != http.MethodPost {
				// The real site ignores ?pageno= on GET and always returns page
				// one; refuse outright so a regression to GET fails loudly.
				http.Error(w, "listing must be POSTed", http.StatusMethodNotAllowed)
				return
			}
			_ = r.ParseForm()
			page, _ := strconv.Atoi(r.PostFormValue("pageno"))
			if page < 1 {
				page = 1
			}
			var items []listingItem
			if page <= len(pages) {
				items = pages[page-1]
			}
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
			_, _ = fmt.Fprintf(w, `<span class="pagenumber">%d</span>件%s`, total, listing)
		default:
			// Serve a detail page whose product code matches the requested
			// product. A single shared fixture would give every scene the same
			// ID, hiding the fact that the ID comes from the detail page.
			body := detailHTML
			pid := r.URL.Query().Get("product_id")
			for _, page := range pages {
				for _, item := range page {
					if item.productID == pid {
						body = strings.ReplaceAll(detailHTML, "DOKI-033", item.code)
					}
				}
			}
			_, _ = fmt.Fprint(w, body)
		}
	}))
}

func TestRun(t *testing.T) {
	items := []listingItem{
		{productID: "100", code: "ABC-001"},
		{productID: "200", code: "DEF-002"},
	}
	ts := newTestServer([][]listingItem{items})
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

	// The ID comes from the detail page's productID, so each product must
	// yield its own — a shared detail fixture previously gave both the same.
	gotIDs := map[string]bool{scenes[0].ID: true, scenes[1].ID: true}
	if !gotIDs["ABC-001"] || !gotIDs["DEF-002"] {
		t.Errorf("scene IDs = %q/%q, want ABC-001 and DEF-002", scenes[0].ID, scenes[1].ID)
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
	ts := newTestServer([][]listingItem{items})
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

// The regression test for the pagination bug: the listing paginates by POSTed
// form fields, not by `?pageno=`. Before the fix the scraper GET-walked every
// page and the site returned page one each time, so a --full run emitted the
// same products repeatedly — they collapsed at Save (keyed on id, site_id) and
// the store kept only the first page.
func TestRunWalksAllPages(t *testing.T) {
	// The page count comes from the site's reported total divided by pageSize,
	// so a multi-page fixture needs a full first page — anything smaller is
	// legitimately one page and would prove nothing.
	mkPage := func(prefix string, base, n int) []listingItem {
		out := make([]listingItem, n)
		for i := range out {
			// productID must be numeric — pidRe is product_id=(\d+).
			out[i] = listingItem{
				productID: fmt.Sprintf("%d%03d", base, i),
				code:      fmt.Sprintf("%s-%03d", prefix, i),
			}
		}
		return out
	}
	pages := [][]listingItem{mkPage("AAA", 1, pageSize), mkPage("BBB", 2, 3)}
	ts := newTestServer(pages)
	defer ts.Close()

	s := &Scraper{client: ts.Client(), siteBase: ts.URL}
	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{Workers: 1})
	if err != nil {
		t.Fatal(err)
	}

	scenes := testutil.CollectScenes(t, ch)
	// CollectScenes also fails on duplicate IDs, so a scraper that re-read page
	// one would trip that as well as the count.
	want := pageSize + 3
	if len(scenes) != want {
		t.Fatalf("got %d scenes, want %d across 2 pages", len(scenes), want)
	}
	got := map[string]bool{}
	for _, sc := range scenes {
		got[sc.ID] = true
	}
	// One from each page: page 2 is only reached if pagination works.
	for _, id := range []string{"AAA-000", "BBB-000", "BBB-002"} {
		if !got[id] {
			t.Errorf("scene %q missing — the page walk did not reach it", id)
		}
	}
}

// A GET to the listing must not be how pages are fetched: the real site ignores
// the query string there, which is what made the old walk silently useless.
func TestListingIsFetchedByPost(t *testing.T) {
	var methods []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/products/list.php" {
			methods = append(methods, r.Method)
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprint(w, `<span class="pagenumber">0</span>件`)
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client(), siteBase: ts.URL}
	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{Workers: 1})
	if err != nil {
		t.Fatal(err)
	}
	for range ch {
	}

	if len(methods) == 0 {
		t.Fatal("listing was never fetched")
	}
	for _, m := range methods {
		if m != http.MethodPost {
			t.Errorf("listing fetched with %s, want POST", m)
		}
	}
}
