package takaratv

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
	code  string
	title string
	date  string
}

func listingPageHTML(items []testItem, total int, hasNext bool) string {
	var sb strings.Builder
	sb.WriteString(`<html><body>`)
	fmt.Fprintf(&sb, `<div align="right" class="pagemenu">%d件中、1 ～ %d件目を表示`, total, len(items))
	if hasNext {
		sb.WriteString(`<a href="/search.php?search_flag=top&amp;p=2" title="next page">[次の20件]</a>`)
	}
	sb.WriteString(`</div>`)
	sb.WriteString(`<ul class="new_movies">`)
	for _, item := range items {
		fmt.Fprintf(&sb, "<!--発売日:%s-->\n", item.date)
		fmt.Fprintf(&sb, `<li><a href="https://www.takara-tv.jp/dvd_detail.php?code=%s"><img src="./product/s/%s.jpg"  alt="%s" /></a></li>`+"\n",
			item.code, strings.ToLower(item.code), item.title)
	}
	sb.WriteString(`</ul>`)
	sb.WriteString(`</body></html>`)
	return sb.String()
}

func detailPageHTML(code, title, desc, performer, director string, durationMin int, date string) string {
	var sb strings.Builder
	sb.WriteString(`<html><body>`)
	sb.WriteString(`<table id="product_detail"><tr><td valign="top">`)
	fmt.Fprintf(&sb, `<a href="./product/l/%s.jpg" rel="lightbox[]"><img src="./product/s/%s.jpg"></a>`,
		strings.ToLower(code), strings.ToLower(code))
	sb.WriteString(`</td><td valign="top">`)
	sb.WriteString(`<table class="product_i">`)
	fmt.Fprintf(&sb, `<tr><th>タイトル</th><td align="left">%s</td></tr>`, title)
	fmt.Fprintf(&sb, `<tr><th>モデル名</th><td align="left">%s</td></tr>`, performer)
	fmt.Fprintf(&sb, `<tr><th>監督名</th><td align="left">%s</td></tr>`, director)
	fmt.Fprintf(&sb, `<tr><th>品番</th><td align="left">%s</td></tr>`, code)
	fmt.Fprintf(&sb, `<tr><th>発売日</th><td align="left">%s</td></tr>`, date)
	fmt.Fprintf(&sb, `<tr><th>収録時間</th><td align="left">%d分</td></tr>`, durationMin)
	sb.WriteString(`<tr><th>レーベル</th><td align="left"></td></tr>`)
	sb.WriteString(`<tr><th colspan="2">作品紹介</th></tr>`)
	fmt.Fprintf(&sb, `<tr><th colspan="2">%s</th></tr>`, desc)
	sb.WriteString(`</table></td></tr></table>`)
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
		{"https://www.takara-tv.jp/", true},
		{"https://www.takara-tv.jp", true},
		{"https://takara-tv.jp/", true},
		{"https://www.takara-tv.jp/top_index.php", true},
		{"https://www.takara-tv.jp/catalog.php", true},
		{"https://www.takara-tv.jp/search.php?search_flag=top", true},
		{"https://www.takara-tv.jp/search.php?ac=712&search_flag=top", true},
		{"https://www.takara-tv.jp/search.php?lb=1&search_flag=top", true},
		{"https://www.takara-tv.jp/product_search.php", true},
		// Detail pages should NOT match
		{"https://www.takara-tv.jp/dvd_detail.php?code=ALDN-576", false},
		// Other sites
		{"https://example.com/search.php", false},
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
		{"https://www.takara-tv.jp/", "https://www.takara-tv.jp/search.php?search_flag=top"},
		{"https://www.takara-tv.jp/top_index.php", "https://www.takara-tv.jp/search.php?search_flag=top"},
		{"https://www.takara-tv.jp/catalog.php", "https://www.takara-tv.jp/search.php?search_flag=top"},
		{"https://www.takara-tv.jp/search.php?ac=712&search_flag=top", "https://www.takara-tv.jp/search.php?ac=712&search_flag=top"},
		{"https://www.takara-tv.jp/search.php?ac=712&search_flag=top&p=3", "https://www.takara-tv.jp/search.php?ac=712&search_flag=top"},
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
		{"https://www.takara-tv.jp/search.php?search_flag=top", 1, "https://www.takara-tv.jp/search.php?search_flag=top"},
		{"https://www.takara-tv.jp/search.php?search_flag=top", 2, "https://www.takara-tv.jp/search.php?p=2&search_flag=top"},
		{"https://www.takara-tv.jp/search.php?ac=712&search_flag=top", 3, "https://www.takara-tv.jp/search.php?ac=712&p=3&search_flag=top"},
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
		code      string
		want      string
	}{
		{"https://www.takara-tv.jp/search.php?search_flag=top", "ALDN-576", "https://www.takara-tv.jp/dvd_detail.php?code=ALDN-576"},
		{"http://localhost:12345/search.php", "TEST-001", "http://localhost:12345/dvd_detail.php?code=TEST-001"},
	}
	for _, c := range cases {
		got := buildDetailURL(c.studioURL, c.code)
		if got != c.want {
			t.Errorf("buildDetailURL(%q, %q) = %q, want %q", c.studioURL, c.code, got, c.want)
		}
	}
}

// ---- TestParseListingItems ----

func TestParseListingItems(t *testing.T) {
	items := []testItem{
		{"ALDN-586", "お義母さん　水野優香", "2026-05-26"},
		{"ALDN-587", "今から妻を献上します", "2026-05-26"},
		{"ALDN-576", "義姉の中出し　通野未帆", "2026-04-14"},
	}
	body := []byte(listingPageHTML(items, 100, true))
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
		wantDate := want.date
		gotDate := got[i].date.Format("2006-01-02")
		if gotDate != wantDate {
			t.Errorf("item[%d].date = %q, want %q", i, gotDate, wantDate)
		}
	}
}

func TestParseListingItemsDedup(t *testing.T) {
	items := []testItem{
		{"ALDN-586", "Title A", "2026-05-26"},
		{"ALDN-586", "Title A", "2026-05-26"},
	}
	body := []byte(listingPageHTML(items, 2, false))
	got := parseListingItems(body)
	if len(got) != 1 {
		t.Errorf("parseListingItems returned %d items, want 1 (dedup)", len(got))
	}
}

func TestParseListingItemsNoImage(t *testing.T) {
	body := []byte(`<!--発売日:2026-05-26-->
<li><a href="https://www.takara-tv.jp/dvd_detail.php?code=ALDN-586"><img src="./img/no_image_s.jpg"  alt="Test Title" /></a></li>`)
	got := parseListingItems(body)
	if len(got) != 1 {
		t.Fatalf("parseListingItems returned %d items, want 1", len(got))
	}
	if got[0].thumb != "" {
		t.Errorf("thumb should be empty for no_image, got %q", got[0].thumb)
	}
}

// ---- TestExtractTotal ----

func TestExtractTotal(t *testing.T) {
	items := []testItem{{"ALDN-586", "Test", "2026-05-26"}}
	body := []byte(listingPageHTML(items, 1094, false))
	got := extractTotal(body)
	if got != 1094 {
		t.Errorf("extractTotal = %d, want 1094", got)
	}
}

func TestExtractTotalNotPresent(t *testing.T) {
	body := []byte(`<html><body>no total here</body></html>`)
	got := extractTotal(body)
	if got != 0 {
		t.Errorf("extractTotal = %d, want 0", got)
	}
}

// ---- TestHasNextPage ----

func TestHasNextPage(t *testing.T) {
	items := []testItem{{"ALDN-586", "Test", "2026-05-26"}}

	withNext := []byte(listingPageHTML(items, 100, true))
	if !hasNextPage(withNext) {
		t.Error("hasNextPage should be true when next link is present")
	}

	withoutNext := []byte(listingPageHTML(items, 1, false))
	if hasNextPage(withoutNext) {
		t.Error("hasNextPage should be false when no next link")
	}
}

// ---- TestExtractPerformer ----

func TestExtractPerformer(t *testing.T) {
	cases := []struct {
		title string
		want  string
	}{
		{"義姉の中出しＳＥＸ代行　通野未帆", "通野未帆"},
		{"お義母さん、にょっ女房よりずっといいよ　水野優香", "水野優香"},
		{"ナンパ！SPECIAL BEST 240分", ""},
		{"", ""},
	}
	for _, c := range cases {
		got := extractPerformer(c.title)
		if got != c.want {
			t.Errorf("extractPerformer(%q) = %q, want %q", c.title, got, c.want)
		}
	}
}

// ---- TestParseDetail ----

func TestParseDetail(t *testing.T) {
	body := []byte(detailPageHTML(
		"ALDN-576",
		"義姉の中出しＳＥＸ代行　通野未帆",
		"Test description text.",
		"通野未帆",
		"テスト監督",
		100,
		"2026年4月14日",
	))

	item := listingItem{code: "ALDN-576", title: "listing title", date: time.Date(2026, 4, 14, 0, 0, 0, 0, time.UTC)}
	scene := parseDetail(body, "https://www.takara-tv.jp/search.php?search_flag=top", item, "https://www.takara-tv.jp/dvd_detail.php?code=ALDN-576")

	if scene.ID != "ALDN-576" {
		t.Errorf("ID = %q, want %q", scene.ID, "ALDN-576")
	}
	if scene.SiteID != "takaratv" {
		t.Errorf("SiteID = %q, want %q", scene.SiteID, "takaratv")
	}
	if scene.Title != "義姉の中出しＳＥＸ代行　通野未帆" {
		t.Errorf("Title = %q", scene.Title)
	}
	if scene.Thumbnail != "https://www.takara-tv.jp/product/l/aldn-576.jpg" {
		t.Errorf("Thumbnail = %q", scene.Thumbnail)
	}
	if scene.Description != "Test description text." {
		t.Errorf("Description = %q, want %q", scene.Description, "Test description text.")
	}
	if len(scene.Performers) != 1 || scene.Performers[0] != "通野未帆" {
		t.Errorf("Performers = %v, want [通野未帆]", scene.Performers)
	}
	if scene.Director != "テスト監督" {
		t.Errorf("Director = %q, want %q", scene.Director, "テスト監督")
	}
	if scene.Duration != 6000 {
		t.Errorf("Duration = %d, want 6000 (100min)", scene.Duration)
	}
	if scene.Date.Year() != 2026 || scene.Date.Month() != 4 || scene.Date.Day() != 14 {
		t.Errorf("Date = %v, want 2026-04-14", scene.Date)
	}
	if scene.Studio != "Takara" {
		t.Errorf("Studio = %q, want %q", scene.Studio, "Takara")
	}
}

func TestParseDetailPerformerFromTitle(t *testing.T) {
	body := []byte(detailPageHTML(
		"ALDN-576",
		"義姉の代行　通野未帆",
		"",
		"",
		"",
		90,
		"2026年1月1日",
	))

	item := listingItem{code: "ALDN-576", title: "listing title", date: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	scene := parseDetail(body, "https://www.takara-tv.jp/search.php?search_flag=top", item, "https://www.takara-tv.jp/dvd_detail.php?code=ALDN-576")

	if len(scene.Performers) != 1 || scene.Performers[0] != "通野未帆" {
		t.Errorf("Performers from title = %v, want [通野未帆]", scene.Performers)
	}
}

// ---- TestListScenes ----

func TestListScenes(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search.php":
			items := []testItem{
				{"ALDN-576", "Title One　Performer A", "2026-04-14"},
				{"ALDN-577", "Title Two　Performer B", "2026-04-14"},
			}
			_, _ = fmt.Fprint(w, listingPageHTML(items, 2, false))
		case "/dvd_detail.php":
			switch r.URL.Query().Get("code") {
			case "ALDN-576":
				_, _ = fmt.Fprint(w, detailPageHTML("ALDN-576", "Title One　Performer A", "Desc one", "Performer A", "", 100, "2026年4月14日"))
			case "ALDN-577":
				_, _ = fmt.Fprint(w, detailPageHTML("ALDN-577", "Title Two　Performer B", "Desc two", "Performer B", "", 90, "2026年4月14日"))
			default:
				http.NotFound(w, r)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client()}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/search.php?search_flag=top", scraper.ListOpts{})
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
	if got["ALDN-576"] != "Title One　Performer A" {
		t.Errorf("ALDN-576 title = %q", got["ALDN-576"])
	}
	if got["ALDN-577"] != "Title Two　Performer B" {
		t.Errorf("ALDN-577 title = %q", got["ALDN-577"])
	}
}

// ---- TestListScenesKnownIDs ----

func TestListScenesKnownIDs(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search.php":
			items := []testItem{
				{"ALDN-576", "Title One", "2026-04-14"},
				{"ALDN-577", "Title Two", "2026-04-14"},
				{"ALDN-578", "Title Three", "2026-04-14"},
			}
			_, _ = fmt.Fprint(w, listingPageHTML(items, 3, false))
		case "/dvd_detail.php":
			code := r.URL.Query().Get("code")
			_, _ = fmt.Fprint(w, detailPageHTML(code, "Title", "", "", "", 60, "2026年4月14日"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client()}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/search.php?search_flag=top", scraper.ListOpts{
		KnownIDs: map[string]bool{"ALDN-577": true},
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
	if len(scenes) > 0 && scenes[0].ID != "ALDN-576" {
		t.Errorf("scene ID = %q, want %q", scenes[0].ID, "ALDN-576")
	}
}

// ---- TestListScenesPagination ----

func TestListScenesPagination(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search.php":
			page := r.URL.Query().Get("p")
			switch page {
			case "", "1":
				items := []testItem{{"ALDN-576", "Title One", "2026-04-14"}}
				_, _ = fmt.Fprint(w, listingPageHTML(items, 2, true))
			case "2":
				items := []testItem{{"ALDN-577", "Title Two", "2026-03-01"}}
				_, _ = fmt.Fprint(w, listingPageHTML(items, 2, false))
			default:
				_, _ = fmt.Fprint(w, listingPageHTML(nil, 0, false))
			}
		case "/dvd_detail.php":
			code := r.URL.Query().Get("code")
			_, _ = fmt.Fprint(w, detailPageHTML(code, "Title "+code, "", "", "", 60, "2026年1月1日"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client()}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/search.php?search_flag=top", scraper.ListOpts{})
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
	if !got["ALDN-576"] || !got["ALDN-577"] {
		t.Errorf("missing expected scenes: got %v", got)
	}
}

// ---- TestListScenesMainPage ----

func TestListScenesMainPage(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search.php":
			items := []testItem{{"ALDN-576", "Title", "2026-04-14"}}
			_, _ = fmt.Fprint(w, listingPageHTML(items, 1, false))
		case "/dvd_detail.php":
			_, _ = fmt.Fprint(w, detailPageHTML("ALDN-576", "Title", "", "", "", 60, "2026年4月14日"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client()}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/top_index.php", scraper.ListOpts{})
	if err != nil {
		t.Fatalf("ListScenes error: %v", err)
	}
	scenes := testutil.CollectScenes(t, ch)

	if len(scenes) != 1 {
		t.Fatalf("got %d scenes, want 1", len(scenes))
	}
	if scenes[0].ID != "ALDN-576" {
		t.Errorf("scene ID = %q, want %q", scenes[0].ID, "ALDN-576")
	}
}
