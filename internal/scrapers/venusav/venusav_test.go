package venusav

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
	"github.com/Wasylq/FSS/scraper"
)

// ---- fixtures ----

type testItem struct {
	code      string
	title     string
	performer string
	date      string
}

func listingPageHTML(items []testItem, lastPage int, hasNext bool) string {
	var sb strings.Builder
	sb.WriteString(`<html><body>`)
	sb.WriteString(`<div class="wp-pagenavi">`)
	if lastPage > 0 {
		fmt.Fprintf(&sb, `<a class="last" href="/all/page/%d/">最後へ &raquo;</a>`, lastPage)
	}
	if hasNext {
		sb.WriteString(`<a class="nextpostslink" href="/all/page/2/" rel="next">&raquo;</a>`)
	}
	sb.WriteString(`</div>`)
	sb.WriteString(`<ul>`)
	for _, item := range items {
		fmt.Fprintf(&sb, `<li><a href="/products/%s/">`, item.code)
		fmt.Fprintf(&sb, `<div class="topNewreleaseListImg"><div><img src="/wp-content/uploads/old/%s.jpg" alt=""></div></div>`, item.code)
		sb.WriteString(`<div class="topNewreleaseListDetail">`)
		fmt.Fprintf(&sb, `<p class="topNewreleaseListTtl">%s</p>`, item.title)
		fmt.Fprintf(&sb, `<div class="topNewreleaseListName"><div>%s</div></div>`, item.performer)
		fmt.Fprintf(&sb, `<p>発売日：%s</p>`, item.date)
		sb.WriteString(`</div></a></li>`)
	}
	sb.WriteString(`</ul></body></html>`)
	return sb.String()
}

func detailPageHTML(code, title, desc, performer, label, date string) string {
	var sb strings.Builder
	sb.WriteString(`<html><body>`)
	sb.WriteString(`<main id="main" class="products clearfix">`)
	fmt.Fprintf(&sb, `<div class="hero"><h1>%s</h1></div>`, title)
	fmt.Fprintf(&sb, `<div class="productsImg"><div class="hero"><img src="/wp-content/uploads/old/%s.jpg" alt=""></div></div>`, code)
	sb.WriteString(`<div class="productsData"><div class="productsDataDetail"><div class="hero">`)
	fmt.Fprintf(&sb, `<dl><dt>作品紹介</dt><dd>%s</dd></dl>`, desc)
	fmt.Fprintf(&sb, `<dl><dt>出演女優</dt><dd>%s</dd></dl>`, performer)
	fmt.Fprintf(&sb, `<dl><dt>品番・分数</dt><dd>%s</dd></dl>`, code)
	fmt.Fprintf(&sb, `<dl><dt>レーベル</dt><dd>%s</dd></dl>`, label)
	fmt.Fprintf(&sb, `<dl><dt>発売日</dt><dd>%s</dd></dl>`, date)
	sb.WriteString(`</div></div></div>`)
	sb.WriteString(`</main></body></html>`)
	return sb.String()
}

// ---- TestMatchesURL ----

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url   string
		match bool
	}{
		{"https://venus-av.com/", true},
		{"https://venus-av.com", true},
		{"https://www.venus-av.com/", true},
		{"https://venus-av.com/all", true},
		{"https://venus-av.com/all/", true},
		{"https://venus-av.com/all/page/2/", true},
		{"https://venus-av.com/new-release/", true},
		{"https://venus-av.com/new-release", true},
		// Detail pages should NOT match
		{"https://venus-av.com/products/some-title/", false},
		// Other sites
		{"https://example.com/all", false},
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
		{"https://venus-av.com/", "https://venus-av.com/all/"},
		{"https://venus-av.com", "https://venus-av.com/all/"},
		{"https://venus-av.com/all", "https://venus-av.com/all/"},
		{"https://venus-av.com/all/", "https://venus-av.com/all/"},
		{"https://venus-av.com/new-release/", "https://venus-av.com/new-release/"},
		{"https://venus-av.com/new-release", "https://venus-av.com/new-release/"},
	}
	for _, c := range cases {
		got := resolveListingURL(c.input)
		if got != c.want {
			t.Errorf("resolveListingURL(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

// ---- TestBuildPageURL ----

func TestBuildPageURL(t *testing.T) {
	cases := []struct {
		base string
		page int
		want string
	}{
		{"https://venus-av.com/all/", 1, "https://venus-av.com/all/"},
		{"https://venus-av.com/all/", 2, "https://venus-av.com/all/page/2/"},
		{"https://venus-av.com/all/", 170, "https://venus-av.com/all/page/170/"},
	}
	for _, c := range cases {
		got := buildPageURL(c.base, c.page)
		if got != c.want {
			t.Errorf("buildPageURL(%q, %d) = %q, want %q", c.base, c.page, got, c.want)
		}
	}
}

// ---- TestBuildDetailURL ----

func TestBuildDetailURL(t *testing.T) {
	cases := []struct {
		studioURL string
		path      string
		want      string
	}{
		{"https://venus-av.com/all/", "/products/some-title/", "https://venus-av.com/products/some-title/"},
		{"http://localhost:12345/all/", "/products/test/", "http://localhost:12345/products/test/"},
	}
	for _, c := range cases {
		got := buildDetailURL(c.studioURL, c.path)
		if got != c.want {
			t.Errorf("buildDetailURL(%q, %q) = %q, want %q", c.studioURL, c.path, got, c.want)
		}
	}
}

// ---- TestParseListingItems ----

func TestParseListingItems(t *testing.T) {
	items := []testItem{
		{"VENX-371", "スリルでテンションが爆上がりする母と息子", "平岡里枝子", "2026年5月19日"},
		{"VENX-372", "義母と内緒の肉体関係", "水野優香", "2026年5月19日"},
		{"VEC-770", "友人の母親", "通野未帆", "2026年4月14日"},
	}
	body := []byte(listingPageHTML(items, 170, true))
	got := parseListingItems(body)

	if len(got) != 3 {
		t.Fatalf("parseListingItems returned %d items, want 3", len(got))
	}
	for i, want := range items {
		if got[i].code != want.code {
			t.Errorf("item[%d].code = %q, want %q", i, got[i].code, want.code)
		}
		if got[i].title != want.title {
			t.Errorf("item[%d].title = %q, want %q", i, got[i].title, want.title)
		}
		if got[i].performer != want.performer {
			t.Errorf("item[%d].performer = %q, want %q", i, got[i].performer, want.performer)
		}
		if got[i].path != "/products/"+want.code+"/" {
			t.Errorf("item[%d].path = %q, want %q", i, got[i].path, "/products/"+want.code+"/")
		}
	}
}

func TestParseListingItemsDedup(t *testing.T) {
	items := []testItem{
		{"VENX-371", "Title A", "Performer", "2026年5月19日"},
		{"VENX-371", "Title A", "Performer", "2026年5月19日"},
	}
	body := []byte(listingPageHTML(items, 0, false))
	got := parseListingItems(body)
	if len(got) != 1 {
		t.Errorf("parseListingItems returned %d items, want 1 (dedup)", len(got))
	}
}

func TestParseListingItemsNoPerformer(t *testing.T) {
	items := []testItem{
		{"VENX-371", "Title", "", "2026年5月19日"},
	}
	body := []byte(listingPageHTML(items, 0, false))
	got := parseListingItems(body)
	if len(got) != 1 {
		t.Fatalf("parseListingItems returned %d items, want 1", len(got))
	}
	if got[0].performer != "" {
		t.Errorf("performer should be empty, got %q", got[0].performer)
	}
}

// ---- TestParseDate ----

func TestParseDate(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"2026年5月19日", "2026-05-19"},
		{"2026年12月1日", "2026-12-01"},
		{"2026月4月7日", "2026-04-07"}, // malformed (missing 年)
		{"no date here", "0001-01-01"},
	}
	for _, c := range cases {
		got := parseDate(c.input)
		gotStr := got.Format("2006-01-02")
		if gotStr != c.want {
			t.Errorf("parseDate(%q) = %q, want %q", c.input, gotStr, c.want)
		}
	}
}

// ---- TestExtractTotal ----

func TestExtractTotal(t *testing.T) {
	items := []testItem{{"VENX-371", "Test", "Performer", "2026年5月19日"}}
	body := []byte(listingPageHTML(items, 170, true))
	got := extractTotal(body)
	if got != 3400 {
		t.Errorf("extractTotal = %d, want 3400 (170 * 20)", got)
	}
}

func TestExtractTotalNoLastPage(t *testing.T) {
	items := []testItem{{"VENX-371", "Test", "Performer", "2026年5月19日"}}
	body := []byte(listingPageHTML(items, 0, false))
	got := extractTotal(body)
	if got != 0 {
		t.Errorf("extractTotal = %d, want 0", got)
	}
}

// ---- TestHasNextPage ----

func TestHasNextPage(t *testing.T) {
	items := []testItem{{"VENX-371", "Test", "", "2026年5月19日"}}

	withNext := []byte(listingPageHTML(items, 170, true))
	if !hasNextPage(withNext) {
		t.Error("hasNextPage should be true when nextpostslink is present")
	}

	withoutNext := []byte(listingPageHTML(items, 0, false))
	if hasNextPage(withoutNext) {
		t.Error("hasNextPage should be false when no nextpostslink")
	}
}

// ---- TestParseDetail ----

func TestParseDetail(t *testing.T) {
	body := []byte(detailPageHTML(
		"VENX-371",
		"スリルでテンションが爆上がりする母と息子が父の留守中に何度も何度も中出しSEX",
		"Test description text.",
		"平岡里枝子",
		"INCEST",
		"2026年5月19日",
	))

	item := listingItem{code: "VENX-371", title: "listing title", performer: "平岡里枝子", date: time.Date(2026, 5, 19, 0, 0, 0, 0, time.UTC)}
	scene := parseDetail(body, "https://venus-av.com/all/", item, "https://venus-av.com/products/test/")

	if scene.ID != "VENX-371" {
		t.Errorf("ID = %q, want %q", scene.ID, "VENX-371")
	}
	if scene.SiteID != "venusav" {
		t.Errorf("SiteID = %q, want %q", scene.SiteID, "venusav")
	}
	if scene.Title != "スリルでテンションが爆上がりする母と息子が父の留守中に何度も何度も中出しSEX" {
		t.Errorf("Title = %q", scene.Title)
	}
	if scene.Thumbnail != "https://venus-av.com/wp-content/uploads/old/VENX-371.jpg" {
		t.Errorf("Thumbnail = %q", scene.Thumbnail)
	}
	if scene.Description != "Test description text." {
		t.Errorf("Description = %q", scene.Description)
	}
	if len(scene.Performers) != 1 || scene.Performers[0] != "平岡里枝子" {
		t.Errorf("Performers = %v, want [平岡里枝子]", scene.Performers)
	}
	if scene.Series != "INCEST" {
		t.Errorf("Series = %q, want %q", scene.Series, "INCEST")
	}
	if scene.Date.Year() != 2026 || scene.Date.Month() != 5 || scene.Date.Day() != 19 {
		t.Errorf("Date = %v, want 2026-05-19", scene.Date)
	}
	if scene.Studio != "VENUS" {
		t.Errorf("Studio = %q, want %q", scene.Studio, "VENUS")
	}
}

func TestParseDetailPerformerFromListing(t *testing.T) {
	body := []byte(detailPageHTML(
		"VENX-371",
		"Title Text",
		"",
		"",
		"",
		"2026年1月1日",
	))

	item := listingItem{code: "VENX-371", title: "listing title", performer: "水野優香", date: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	scene := parseDetail(body, "https://venus-av.com/all/", item, "https://venus-av.com/products/test/")

	if len(scene.Performers) != 1 || scene.Performers[0] != "水野優香" {
		t.Errorf("Performers from listing = %v, want [水野優香]", scene.Performers)
	}
}

// ---- TestListScenes ----

func TestListScenes(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/all/":
			items := []testItem{
				{"VENX-371", "Title One", "Performer A", "2026年5月19日"},
				{"VEC-770", "Title Two", "Performer B", "2026年4月14日"},
			}
			_, _ = fmt.Fprint(w, listingPageHTML(items, 0, false))
		case "/products/VENX-371/":
			_, _ = fmt.Fprint(w, detailPageHTML("VENX-371", "Title One", "Desc one", "Performer A", "INCEST", "2026年5月19日"))
		case "/products/VEC-770/":
			_, _ = fmt.Fprint(w, detailPageHTML("VEC-770", "Title Two", "Desc two", "Performer B", "女神", "2026年4月14日"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client()}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/all/", scraper.ListOpts{})
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
	if got["VENX-371"] != "Title One" {
		t.Errorf("VENX-371 title = %q", got["VENX-371"])
	}
	if got["VEC-770"] != "Title Two" {
		t.Errorf("VEC-770 title = %q", got["VEC-770"])
	}
}

// ---- TestListScenesKnownIDs ----

func TestListScenesKnownIDs(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/all/":
			items := []testItem{
				{"VENX-371", "Title One", "Performer A", "2026年5月19日"},
				{"VENX-372", "Title Two", "Performer B", "2026年5月19日"},
				{"VENX-373", "Title Three", "Performer C", "2026年5月19日"},
			}
			_, _ = fmt.Fprint(w, listingPageHTML(items, 0, false))
		case "/products/VENX-371/":
			_, _ = fmt.Fprint(w, detailPageHTML("VENX-371", "Title One", "", "Performer A", "", "2026年5月19日"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client()}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/all/", scraper.ListOpts{
		KnownIDs: map[string]bool{"VENX-372": true},
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
	if len(scenes) > 0 && scenes[0].ID != "VENX-371" {
		t.Errorf("scene ID = %q, want %q", scenes[0].ID, "VENX-371")
	}
}

// ---- TestListScenesPagination ----

func TestListScenesPagination(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/all/":
			items := []testItem{{"VENX-371", "Title One", "Performer A", "2026年5月19日"}}
			_, _ = fmt.Fprint(w, listingPageHTML(items, 2, true))
		case "/all/page/2/":
			items := []testItem{{"VEC-770", "Title Two", "Performer B", "2026年4月14日"}}
			_, _ = fmt.Fprint(w, listingPageHTML(items, 2, false))
		case "/products/VENX-371/":
			_, _ = fmt.Fprint(w, detailPageHTML("VENX-371", "Title One", "", "Performer A", "", "2026年5月19日"))
		case "/products/VEC-770/":
			_, _ = fmt.Fprint(w, detailPageHTML("VEC-770", "Title Two", "", "Performer B", "", "2026年4月14日"))
		default:
			_, _ = fmt.Fprint(w, listingPageHTML(nil, 0, false))
		}
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client()}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/all/", scraper.ListOpts{})
	if err != nil {
		t.Fatalf("ListScenes error: %v", err)
	}
	scenes := testutil.CollectScenes(t, ch)

	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2", len(scenes))
	}

	got := map[string]bool{}
	for _, sc := range scenes {
		got[sc.ID] = true
	}
	if !got["VENX-371"] || !got["VEC-770"] {
		t.Errorf("missing expected scenes: got %v", got)
	}
}

// ---- TestListScenesMainPage ----

func TestListScenesMainPage(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/all/":
			items := []testItem{{"VENX-371", "Title", "Performer", "2026年5月19日"}}
			_, _ = fmt.Fprint(w, listingPageHTML(items, 0, false))
		case "/products/VENX-371/":
			_, _ = fmt.Fprint(w, detailPageHTML("VENX-371", "Title", "", "Performer", "", "2026年5月19日"))
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
	if scenes[0].ID != "VENX-371" {
		t.Errorf("scene ID = %q, want %q", scenes[0].ID, "VENX-371")
	}
}
