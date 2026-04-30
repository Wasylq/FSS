package gloryquest

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

// ---- fixtures ----

func listingPageHTML(codes []string) string {
	var sb strings.Builder
	sb.WriteString(`<html><body>`)
	fmt.Fprintf(&sb, `<span class="search_fukidashi">%d件該当</span>`, len(codes))
	sb.WriteString(`<div style="border:6px solid #000;">`)
	for _, code := range codes {
		lc := strings.ToLower(strings.ReplaceAll(code, "-", ""))
		fmt.Fprintf(&sb, `<span class="thumb2" style="margin-left:0.5em;"><a href="item.php?id=%s" class="various" data-fancybox-type="iframe"><img data-original="/package/200/n_%s.jpg" class="lazy" alt="Title for %s" title="Title for %s" src="/images/nowloading.gif" /></a></span>`, code, lc, code, code)
	}
	sb.WriteString(`</div></body></html>`)
	return sb.String()
}

func detailPageHTML(code, title, desc, date, duration, price, cast, series, genres, director string) string {
	var sb strings.Builder
	sb.WriteString(`<html><body class="search_item" itemscope itemtype="http://schema.org/Product">`)
	lc := strings.ToLower(strings.ReplaceAll(code, "-", ""))
	fmt.Fprintf(&sb, `<div class="package"><img src="/package/800/%s.jpg" alt="%s" class="img_hyou1" itemprop="image" /></div>`, lc, title)
	fmt.Fprintf(&sb, `<h1 style="font-size:1.2em;" itemprop="name">%s</h1>`, title)
	sb.WriteString(`<hr noshade size="1" />`)
	fmt.Fprintf(&sb, `<p style="line-height:1.5em;"><span class="mds">出演</span> %s</p>`, cast)
	sb.WriteString(`<hr noshade size="1" />`)
	sb.WriteString(`<dl style="line-height:1.5em;" itemprop="offers" itemscope itemtype="http://schema.org/Offer">`)
	fmt.Fprintf(&sb, `<dt style="float:left; clear:both;"><span class="mds">品番</span></dt><dd style="float:left;">%s</dd>`, code)
	fmt.Fprintf(&sb, `<dt style="float:left; clear:both;"><span class="mds">発売日</span></dt><dd style="float:left;">%s</dd>`, date)
	fmt.Fprintf(&sb, `<dt style="float:left; clear:both;"><span class="mds">定価</span></dt><dd style="float:left;" itemprop="price">%s</dd>`, price)
	fmt.Fprintf(&sb, `<dt style="float:left; clear:both;"><span class="mds">収録時間</span></dt><dd style="float:left;">%s</dd>`, duration)
	fmt.Fprintf(&sb, `<dt style="float:left; clear:both;"><span class="mds">シリーズ</span></dt><dd style="float:left;">%s</dd>`, series)
	fmt.Fprintf(&sb, `<dt style="float:left; clear:both;"><span class="mds">ジャンル</span></dt><dd style="float:left;">%s</dd>`, genres)
	fmt.Fprintf(&sb, `<dt style="float:left; clear:both;"><span class="mds">監督</span></dt><dd style="float:left;">%s</dd>`, director)
	sb.WriteString(`</dl>`)
	fmt.Fprintf(&sb, `<p class="long_comment" itemprop="description">%s</p>`, desc)
	sb.WriteString(`</body></html>`)
	return sb.String()
}

// ---- TestMatchesURL ----

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url   string
		match bool
	}{
		{"https://www.gloryquest.tv/", true},
		{"https://www.gloryquest.tv", true},
		{"https://gloryquest.tv/", true},
		{"https://www.gloryquest.tv/search.php?KeyWord=test", true},
		{"https://www.gloryquest.tv/search.php?KeyWord=", true},
		// Detail pages should NOT match
		{"https://www.gloryquest.tv/item.php?id=GVH-762", false},
		// Other sites
		{"https://example.com/", false},
		{"", false},
	}
	for _, c := range cases {
		got := s.MatchesURL(c.url)
		if got != c.match {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.match)
		}
	}
}

// ---- TestResolveListingURL ----

func TestResolveListingURL(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"https://www.gloryquest.tv/", "https://www.gloryquest.tv/search.php?KeyWord="},
		{"https://www.gloryquest.tv", "https://www.gloryquest.tv/search.php?KeyWord="},
		{"https://www.gloryquest.tv/search.php?KeyWord=test", "https://www.gloryquest.tv/search.php?KeyWord=test"},
		{"https://www.gloryquest.tv/search.php?KeyWord=", "https://www.gloryquest.tv/search.php?KeyWord="},
	}
	for _, c := range cases {
		got := resolveListingURL(c.input)
		if got != c.want {
			t.Errorf("resolveListingURL(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

// ---- TestBuildDetailURL ----

func TestBuildDetailURL(t *testing.T) {
	got := buildDetailURL("https://www.gloryquest.tv/", "GVH-762")
	want := "https://www.gloryquest.tv/item.php?id=GVH-762"
	if got != want {
		t.Errorf("buildDetailURL = %q, want %q", got, want)
	}
}

// ---- TestParseListingItems ----

func TestParseListingItems(t *testing.T) {
	codes := []string{"GVH-801", "GVH-802", "MVG-147"}
	body := []byte(listingPageHTML(codes))
	got := parseListingItems(body)

	if len(got) != 3 {
		t.Fatalf("parseListingItems returned %d items, want 3", len(got))
	}
	for i, want := range codes {
		if got[i].code != want {
			t.Errorf("item[%d].code = %q, want %q", i, got[i].code, want)
		}
	}
}

func TestParseListingItemsDedup(t *testing.T) {
	codes := []string{"GVH-801", "GVH-801"}
	body := []byte(listingPageHTML(codes))
	got := parseListingItems(body)
	if len(got) != 1 {
		t.Errorf("parseListingItems returned %d items, want 1 (dedup)", len(got))
	}
}

// ---- TestExtractTotal ----

func TestExtractTotal(t *testing.T) {
	body := []byte(listingPageHTML([]string{"GVH-801", "GVH-802", "GVH-803"}))
	got := extractTotal(body)
	if got != 3 {
		t.Errorf("extractTotal = %d, want 3", got)
	}
}

func TestExtractTotalNone(t *testing.T) {
	body := []byte(`<html><body>no count here</body></html>`)
	got := extractTotal(body)
	if got != 0 {
		t.Errorf("extractTotal = %d, want 0", got)
	}
}

// ---- TestParseDetail ----

func TestParseDetail(t *testing.T) {
	body := []byte(detailPageHTML(
		"GVH-762",
		"女の武器を最大限に活かす挑発的美脚オフィスレディ",
		"テスト説明文です。",
		"2025年7月22日",
		"160分",
		"3,498円(+税)",
		`<a href="/search.php?KeyWord=黒川すみれ" target="_parent">黒川すみれ</a><a href="/search.php?KeyWord=" target="_parent"></a>`,
		`<a href="/search.php?KeyWord=" target="_parent"></a>`,
		`<a href="/search.php?KeyWord=美脚" target="_parent">美脚</a><a href="/search.php?KeyWord=OL" target="_parent">OL</a><a href="/search.php?KeyWord=" target="_parent"></a>`,
		`<a href="/search.php?KeyWord=笠井貴人" target="_parent">笠井貴人</a>`,
	))

	item := listingItem{code: "GVH-762"}
	scene := parseDetail(body, "https://www.gloryquest.tv/", item, "https://www.gloryquest.tv/item.php?id=GVH-762")

	if scene.ID != "GVH-762" {
		t.Errorf("ID = %q, want %q", scene.ID, "GVH-762")
	}
	if scene.SiteID != "gloryquest" {
		t.Errorf("SiteID = %q, want %q", scene.SiteID, "gloryquest")
	}
	if scene.Studio != "Glory Quest" {
		t.Errorf("Studio = %q, want %q", scene.Studio, "Glory Quest")
	}
	if scene.Title != "女の武器を最大限に活かす挑発的美脚オフィスレディ" {
		t.Errorf("Title = %q", scene.Title)
	}
	if scene.Thumbnail != "https://www.gloryquest.tv/package/800/gvh762.jpg" {
		t.Errorf("Thumbnail = %q", scene.Thumbnail)
	}
	if scene.Description != "テスト説明文です。" {
		t.Errorf("Description = %q", scene.Description)
	}
	if scene.Date.Year() != 2025 || scene.Date.Month() != 7 || scene.Date.Day() != 22 {
		t.Errorf("Date = %v, want 2025-07-22", scene.Date)
	}
	if scene.Duration != 9600 {
		t.Errorf("Duration = %d, want 9600 (160*60)", scene.Duration)
	}
	if scene.Director != "笠井貴人" {
		t.Errorf("Director = %q, want %q", scene.Director, "笠井貴人")
	}
	if len(scene.Performers) != 1 || scene.Performers[0] != "黒川すみれ" {
		t.Errorf("Performers = %v, want [黒川すみれ]", scene.Performers)
	}
	if len(scene.Tags) != 2 || scene.Tags[0] != "美脚" || scene.Tags[1] != "OL" {
		t.Errorf("Tags = %v, want [美脚 OL]", scene.Tags)
	}
	if len(scene.PriceHistory) != 1 || scene.PriceHistory[0].Regular != 3498 {
		t.Errorf("PriceHistory = %v, want [{Regular:3498}]", scene.PriceHistory)
	}
}

func TestParseDetailMultiPerformer(t *testing.T) {
	body := []byte(detailPageHTML(
		"MVG-143",
		"Test Title",
		"",
		"2025年1月1日",
		"120分",
		"2,980円(+税)",
		`<a href="/search.php?KeyWord=黒川すみれ" target="_parent">黒川すみれ</a><a href="/search.php?KeyWord=紗々原ゆり" target="_parent">紗々原ゆり</a><a href="/search.php?KeyWord=" target="_parent"></a>`,
		"", "", "",
	))

	item := listingItem{code: "MVG-143"}
	scene := parseDetail(body, "https://www.gloryquest.tv/", item, "https://www.gloryquest.tv/item.php?id=MVG-143")

	if len(scene.Performers) != 2 {
		t.Fatalf("Performers count = %d, want 2", len(scene.Performers))
	}
	if scene.Performers[0] != "黒川すみれ" || scene.Performers[1] != "紗々原ゆり" {
		t.Errorf("Performers = %v", scene.Performers)
	}
}

func TestParseDetailNoOptionalFields(t *testing.T) {
	body := []byte(detailPageHTML(
		"GVH-500",
		"Minimal Title",
		"",
		"2023年1月24日",
		"125分",
		"2,980円(+税)",
		`<a href="/search.php?KeyWord=" target="_parent"></a>`,
		`<a href="/search.php?KeyWord=" target="_parent"></a>`,
		`<a href="/search.php?KeyWord=" target="_parent"></a>`,
		`<a href="/search.php?KeyWord=" target="_parent"></a>`,
	))

	item := listingItem{code: "GVH-500"}
	scene := parseDetail(body, "https://www.gloryquest.tv/", item, "https://www.gloryquest.tv/item.php?id=GVH-500")

	if len(scene.Performers) != 0 {
		t.Errorf("Performers should be empty, got %v", scene.Performers)
	}
	if scene.Director != "" {
		t.Errorf("Director should be empty, got %q", scene.Director)
	}
	if scene.Series != "" {
		t.Errorf("Series should be empty, got %q", scene.Series)
	}
	if len(scene.Tags) != 0 {
		t.Errorf("Tags should be empty, got %v", scene.Tags)
	}
}

// ---- TestListScenes ----

func TestListScenes(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search.php":
			codes := []string{"GVH-801", "GVH-802"}
			_, _ = fmt.Fprint(w, listingPageHTML(codes))
		case "/item.php":
			switch r.URL.Query().Get("id") {
			case "GVH-801":
				_, _ = fmt.Fprint(w, detailPageHTML("GVH-801", "Title One", "Desc", "2025年12月9日", "160分", "3,498円(+税)",
					`<a href="/search.php?KeyWord=Perf">Perf</a>`, "", "", `<a href="/search.php?KeyWord=Dir">Dir</a>`))
			case "GVH-802":
				_, _ = fmt.Fprint(w, detailPageHTML("GVH-802", "Title Two", "Desc 2", "2025年11月25日", "120分", "2,980円(+税)",
					"", "", "", ""))
			default:
				http.NotFound(w, r)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client()}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/search.php?KeyWord=", scraper.ListOpts{})
	if err != nil {
		t.Fatalf("ListScenes error: %v", err)
	}
	scenes := testutil.CollectScenes(t, ch)

	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2", len(scenes))
	}

	got := map[string]string{}
	for _, sc := range scenes {
		got[sc.ID] = sc.Title
	}
	if got["GVH-801"] != "Title One" {
		t.Errorf("GVH-801 title = %q", got["GVH-801"])
	}
	if got["GVH-802"] != "Title Two" {
		t.Errorf("GVH-802 title = %q", got["GVH-802"])
	}
}

// ---- TestListScenesKnownIDs ----

func TestListScenesKnownIDs(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search.php":
			codes := []string{"GVH-801", "GVH-802", "GVH-803"}
			_, _ = fmt.Fprint(w, listingPageHTML(codes))
		case "/item.php":
			switch r.URL.Query().Get("id") {
			case "GVH-801":
				_, _ = fmt.Fprint(w, detailPageHTML("GVH-801", "Title One", "", "2025年12月9日", "90分", "2,980円(+税)", "", "", "", ""))
			default:
				http.NotFound(w, r)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client()}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/search.php?KeyWord=", scraper.ListOpts{
		KnownIDs: map[string]bool{"GVH-802": true},
	})
	if err != nil {
		t.Fatalf("ListScenes error: %v", err)
	}
	scenes, stopped := testutil.CollectScenesWithStop(t, ch)

	if !stopped {
		t.Error("expected StoppedEarly signal")
	}
	if len(scenes) != 1 {
		t.Errorf("got %d scenes, want 1", len(scenes))
	}
	if len(scenes) > 0 && scenes[0].ID != "GVH-801" {
		t.Errorf("scene ID = %q, want %q", scenes[0].ID, "GVH-801")
	}
}

// ---- TestListScenesBareDomain ----

func TestListScenesBareDomain(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search.php":
			codes := []string{"GVH-801"}
			_, _ = fmt.Fprint(w, listingPageHTML(codes))
		case "/item.php":
			_, _ = fmt.Fprint(w, detailPageHTML("GVH-801", "Title", "", "2025年12月9日", "90分", "2,980円(+税)", "", "", "", ""))
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client()}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/", scraper.ListOpts{})
	if err != nil {
		t.Fatalf("ListScenes error: %v", err)
	}
	scenes := testutil.CollectScenes(t, ch)

	if len(scenes) != 1 {
		t.Fatalf("got %d scenes, want 1", len(scenes))
	}
	if scenes[0].ID != "GVH-801" {
		t.Errorf("scene ID = %q, want %q", scenes[0].ID, "GVH-801")
	}
}
