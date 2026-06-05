package mercury

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
	"github.com/Wasylq/FSS/scraper"
)

const fixtureListing = `<html><body>
<script>
var ld_blog_vars = {};
ld_blog_vars.articles = [];
ld_blog_vars.articles.push({
    id: 31419219,
    permalink: "https://mercury.diary.to/archives/31419219.html",
    title: "ホームページ移転のお知らせ",
    categories: ["マーキュリー","2023/04"],
    date: '2023-04-27 00:00:05'
});
ld_blog_vars.articles.push({
    id: 31328743,
    permalink: "https://mercury.diary.to/archives/31328743.html",
    title: "HONB-311 素人ナンパ",
    categories: ["初代渋谷特別特攻本部","2023/03"],
    date: '2023-03-15 00:00:05'
});
</script>
</body></html>`

func TestParseListingPage(t *testing.T) {
	items := parseListingPage([]byte(fixtureListing))
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}

	a := items[0]
	if a.articleID != 31419219 {
		t.Errorf("articleID = %d", a.articleID)
	}
	if a.permalink != "https://mercury.diary.to/archives/31419219.html" {
		t.Errorf("permalink = %q", a.permalink)
	}
	if a.title != "ホームページ移転のお知らせ" {
		t.Errorf("title = %q", a.title)
	}
	if a.label != "マーキュリー" {
		t.Errorf("label = %q", a.label)
	}
	if a.date != "2023-04-27 00:00:05" {
		t.Errorf("date = %q", a.date)
	}

	b := items[1]
	if b.articleID != 31328743 {
		t.Errorf("articleID = %d", b.articleID)
	}
	if b.label != "初代渋谷特別特攻本部" {
		t.Errorf("label = %q", b.label)
	}
}

const fixtureDetail = `<html><head>
<meta property="og:image" content="https://livedoor.blogimg.jp/mercry_av/imgs/a/b/cover.jpg">
</head><body>
<div class="article-body">
<a href="#"><img alt="HONB-311" src="https://livedoor.blogimg.jp/mercry_av/imgs/a/b/cover.jpg"></a>
<br>
品番：HONB-311<br>
発売日：2023年3月15日<br>
収録時間：120分<br>
価格：1,980円+tax (税込2,178円)<br>
<br>
素人ナンパの人気シリーズ最新作。
</div>
<div class="article-footer">
<a href="/archives/tag/M男">#M男</a>
<a href="/archives/tag/ナンパ">#ナンパ</a>
</div>
</body></html>`

func TestFetchDetail(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, fixtureDetail)
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client()}
	item := listItem{
		articleID: 31328743,
		permalink: ts.URL + "/archives/31328743.html",
		title:     "HONB-311 素人ナンパ",
		label:     "初代渋谷特別特攻本部",
		date:      "2023-03-15 00:00:05",
	}

	scene, ok, err := s.fetchDetail(context.Background(), item, "https://mercury.diary.to")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected ok=true for product page")
	}

	if scene.ID != "HONB-311" {
		t.Errorf("ID = %q, want HONB-311", scene.ID)
	}
	if scene.SiteID != "mercury" {
		t.Errorf("SiteID = %q", scene.SiteID)
	}
	if scene.Studio != "Mercury" {
		t.Errorf("Studio = %q", scene.Studio)
	}
	if scene.Series != "初代渋谷特別特攻本部" {
		t.Errorf("Series = %q", scene.Series)
	}
	if scene.Duration != 120*60 {
		t.Errorf("Duration = %d, want %d", scene.Duration, 120*60)
	}
	if scene.Thumbnail != "https://livedoor.blogimg.jp/mercry_av/imgs/a/b/cover.jpg" {
		t.Errorf("Thumbnail = %q", scene.Thumbnail)
	}
	if scene.Date.Format("2006-01-02") != "2023-03-15" {
		t.Errorf("Date = %v", scene.Date)
	}
	if len(scene.Tags) != 2 || scene.Tags[0] != "M男" || scene.Tags[1] != "ナンパ" {
		t.Errorf("Tags = %v", scene.Tags)
	}
	if len(scene.PriceHistory) == 0 || scene.PriceHistory[0].Regular != 2178 {
		t.Errorf("PriceHistory = %v", scene.PriceHistory)
	}
}

func TestFetchDetailSkipsNonProduct(t *testing.T) {
	nonProductHTML := `<html><body>
<div class="article-body">
ホームページ移転のお知らせ。新しいURLは...
</div>
</body></html>`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, nonProductHTML)
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client()}
	item := listItem{
		articleID: 31419219,
		permalink: ts.URL + "/archives/31419219.html",
		title:     "ホームページ移転のお知らせ",
		label:     "マーキュリー",
		date:      "2023-04-27 00:00:05",
	}

	_, ok, err := s.fetchDetail(context.Background(), item, "https://mercury.diary.to")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("expected ok=false for non-product page")
	}
}

func TestExtractProductCode(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{`<img alt="HONB-279" src="...">`, "HONB-279"},
		{`品番：GONE-123`, "GONE-123"},
		{`品番:JSTK-045`, "JSTK-045"},
		{`no product code here`, ""},
	}
	for _, c := range cases {
		got := extractProductCode([]byte(c.in))
		if got != c.want {
			t.Errorf("extractProductCode(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestExtractPrice(t *testing.T) {
	cases := []struct {
		in   string
		want float64
	}{
		{"価格：1,980円+tax (税込2,178円)", 2178},
		{"価格：3,300円", 3300},
		{"no price", 0},
	}
	for _, c := range cases {
		got := extractPrice([]byte(c.in))
		if got != c.want {
			t.Errorf("extractPrice(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestParseDate(t *testing.T) {
	d, ok := parseDate("2023-03-15 00:00:05")
	if !ok {
		t.Fatal("parseDate returned false")
	}
	want := time.Date(2023, 3, 15, 0, 0, 5, 0, time.UTC)
	if d != want {
		t.Errorf("date = %v, want %v", d, want)
	}

	_, ok = parseDate("invalid")
	if ok {
		t.Error("expected false for invalid date")
	}
}

func TestResolveListingBase(t *testing.T) {
	cases := []struct {
		url  string
		want string
	}{
		{"https://mercury.diary.to", siteBase},
		{"https://mercury.diary.to/", siteBase},
		{"https://www.mercury-2005.com/", siteBase},
		{"https://mercury.diary.to/archives/cat_12345.html", siteBase + "/archives/cat_12345.html"},
		{"https://mercury.diary.to/archives/cat_12345.html?p=3", siteBase + "/archives/cat_12345.html"},
	}
	for _, c := range cases {
		got := resolveListingBase(c.url)
		if got != c.want {
			t.Errorf("resolveListingBase(%q) = %q, want %q", c.url, got, c.want)
		}
	}
}

func TestPageURL(t *testing.T) {
	cases := []struct {
		base string
		page int
		want string
	}{
		{siteBase, 1, siteBase},
		{siteBase, 2, siteBase + "?p=2"},
		{siteBase + "/archives/cat_123.html", 3, siteBase + "/archives/cat_123.html?p=3"},
	}
	for _, c := range cases {
		got := pageURL(c.base, c.page)
		if got != c.want {
			t.Errorf("pageURL(%q, %d) = %q, want %q", c.base, c.page, got, c.want)
		}
	}
}

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url  string
		want bool
	}{
		{"https://mercury.diary.to", true},
		{"https://mercury.diary.to/archives/31328743.html", true},
		{"https://www.mercury-2005.com/", true},
		{"https://www.mercury-2005.com/category/news/bind", true},
		{"https://example.com/", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.want {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.want)
		}
	}
}

func TestRun(t *testing.T) {
	var tsURL string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("p") != "" && r.URL.Query().Get("p") != "1" {
			_, _ = fmt.Fprint(w, `<html><body><script>var ld_blog_vars = {}; ld_blog_vars.articles = [];</script></body></html>`)
			return
		}
		switch r.URL.Path {
		case "/":
			listing := fmt.Sprintf(`<html><body><script>
var ld_blog_vars = {};
ld_blog_vars.articles = [];
ld_blog_vars.articles.push({
    id: 31419219,
    permalink: "%s/archives/31419219.html",
    title: "Migration notice",
    categories: ["マーキュリー","2023/04"],
    date: '2023-04-27 00:00:05'
});
ld_blog_vars.articles.push({
    id: 31328743,
    permalink: "%s/archives/31328743.html",
    title: "HONB-311 素人ナンパ",
    categories: ["初代渋谷特別特攻本部","2023/03"],
    date: '2023-03-15 00:00:05'
});
</script></body></html>`, tsURL, tsURL)
			_, _ = fmt.Fprint(w, listing)
		case "/archives/31419219.html":
			_, _ = fmt.Fprint(w, `<html><body><div class="article-body">Migration notice.</div></body></html>`)
		case "/archives/31328743.html":
			_, _ = fmt.Fprint(w, fixtureDetail)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()
	tsURL = ts.URL

	s := &Scraper{client: ts.Client()}
	out := make(chan scraper.SceneResult)
	go s.run(context.Background(), ts.URL, scraper.ListOpts{Workers: 1}, out)

	scenes := testutil.CollectScenes(t, out)

	if len(scenes) != 1 {
		t.Errorf("got %d scenes, want 1 (non-product post should be skipped)", len(scenes))
	}
}
