package moodyz

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

func listingPageHTML(items []listingItem, total int) string {
	var sb strings.Builder
	sb.WriteString(`<html><body>`)
	if total > 0 {
		fmt.Fprintf(&sb, `<div class="swiper-pagination-02">全%d作品中 1 〜 %d タイトルを表示</div>`, total, len(items))
	}
	for _, item := range items {
		fmt.Fprintf(&sb, `<div class="item"><div class="c-card">`+
			`<a class="img hover" href="https://moodyz.com/works/detail/%s?page_from=series">`+
			`<img class="c-main-bg lazyload" data-src="%s" alt=""/>`+
			`<div class="hover__child"><p class="text">Title for %s</p></div>`+
			`</a></div></div>`, item.code, item.thumb, item.code)
	}
	sb.WriteString(`</body></html>`)
	return sb.String()
}

func actressListingPageHTML(items []listingItem, total int) string {
	var sb strings.Builder
	sb.WriteString(`<html><body>`)
	if total > 0 {
		fmt.Fprintf(&sb, `<div class="swiper-pagination-02">全%d作品中 1 〜 %d タイトルを表示</div>`, total, len(items))
	}
	for _, item := range items {
		fmt.Fprintf(&sb, `<a class="item" href="https://moodyz.com/works/detail/%s?page_from=actress">`+
			`<div class="c-card"><div class="img hover">`+
			`<img class="c-main-bg lazyload" data-src="%s" alt="" />`+
			`<div class="hover__child"><p class="text">Title for %s</p></div>`+
			`</div></div></a>`, item.code, item.thumb, item.code)
	}
	sb.WriteString(`</body></html>`)
	return sb.String()
}

func detailPageHTML(code, title, desc string, performers []string, director string, genres []string, series string, durationMin int, date string) string {
	var sb strings.Builder
	sb.WriteString(`<html><body><div class="p-workPage l-wrap">`)
	fmt.Fprintf(&sb, `<h2 class="p-workPage__title">  %s  </h2>`, title)
	fmt.Fprintf(&sb, `<p class="p-workPage__text">%s</p>`, desc)
	sb.WriteString(`<div class="p-workPage__block"><div class="p-workPage__table">`)

	sb.WriteString(`<div class="item"><div class="th">女優</div><div class="td">`)
	for _, p := range performers {
		fmt.Fprintf(&sb, `<div class="item"><a class="c-tag c-main-bg-hover c-main-font c-main-bd" href="https://moodyz.com/actress/detail/123">%s</a></div>`, p)
	}
	sb.WriteString(`</div></div>`)

	fmt.Fprintf(&sb, `<div class="item"><div class="th">発売日</div><div class="td"><div class="item">`+
		`<a class="c-tag c-main-bg-hover c-main-font c-main-bd" href="https://moodyz.com/works/list/date/%s">%s</a>`+
		`</div></div></div>`, date, date)

	if series != "" {
		fmt.Fprintf(&sb, `<div class="item"><div class="th">シリーズ</div><div class="item">`+
			`<a class="c-tag c-main-bg-hover c-main-font c-main-bd" href="https://moodyz.com/works/list/series/1">%s</a>`+
			`</div><div class="td"></div></div>`, series)
	}

	if len(genres) > 0 {
		sb.WriteString(`<div class="item"><div class="th">ジャンル</div><div class="td">`)
		for _, g := range genres {
			fmt.Fprintf(&sb, `<div class="item"><a class="c-tag c-main-bg-hover c-main-font c-main-bd" href="https://moodyz.com/works/list/genre/1">%s</a></div>`, g)
		}
		sb.WriteString(`</div></div>`)
	}

	if director != "" {
		fmt.Fprintf(&sb, `<div class="item"><div class="th">監督</div><div class="td"><div class="item"><p>%s</p></div></div></div>`, director)
	}

	fmt.Fprintf(&sb, `<div class="item"><div class="th">品番</div><div class="td"><div class="item -minW">`+
		`<p><span class="c-tag02 c-main-bg-hover c-main-bg">DVD</span>%s</p></div></div></div>`, code)
	fmt.Fprintf(&sb, `<div class="item"><div class="th">収録時間</div><div class="td"><div class="item -minW">`+
		`<p><span class="c-tag02 c-main-bg-hover c-main-bg">DVD</span>%d分</p></div></div></div>`, durationMin)

	sb.WriteString(`</div><div class="p-workPage__side"></div></div></div></body></html>`)
	return sb.String()
}

// ---- TestMatchesURL ----

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url   string
		match bool
	}{
		{"https://moodyz.com/works/list/series/3482", true},
		{"https://moodyz.com/works/list/release", true},
		{"https://moodyz.com/works/list/date/2026-04-21", true},
		{"https://moodyz.com/works/list/genre/126", true},
		{"https://moodyz.com/works/list/label/5046", true},
		{"https://moodyz.com/actress/detail/701326", true},
		{"https://www.moodyz.com/works/list/series/3482", true},
		{"https://moodyz.com/works/detail/MIAD491", false},
		{"https://moodyz.com/", false},
		{"https://example.com/works/list/series/1", false},
		{"", false},
	}
	for _, c := range cases {
		got := s.MatchesURL(c.url)
		if got != c.match {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.match)
		}
	}
}

// ---- TestNormalizeListURL ----

func TestNormalizeListURL(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"https://moodyz.com/actress/detail/700115", "https://moodyz.com/actress/detail/700115"},
		{"https://moodyz.com/actress/detail/700115?page=3", "https://moodyz.com/actress/detail/700115"},
		{"https://moodyz.com/works/list/series/3482?page=2", "https://moodyz.com/works/list/series/3482"},
	}
	for _, c := range cases {
		got := normalizeListURL(c.input)
		if got != c.want {
			t.Errorf("normalizeListURL(%q) = %q, want %q", c.input, got, c.want)
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
		{"https://moodyz.com/actress/detail/700115", 1, "https://moodyz.com/actress/detail/700115"},
		{"https://moodyz.com/actress/detail/700115", 2, "https://moodyz.com/actress/detail/700115?page=2"},
		{"https://moodyz.com/works/list/series/3482", 3, "https://moodyz.com/works/list/series/3482?page=3"},
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
		{"https://moodyz.com/actress/detail/700115", "MIAD491", "https://moodyz.com/works/detail/MIAD491"},
		{"https://moodyz.com/works/list/series/3482", "MDVR418", "https://moodyz.com/works/detail/MDVR418"},
		{"http://localhost:12345/works/list/release", "TEST001", "http://localhost:12345/works/detail/TEST001"},
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
	items := []listingItem{
		{code: "MIAD491", thumb: "https://cdn.example.com/MIAD491_1.jpg"},
		{code: "MIAD469", thumb: "https://cdn.example.com/MIAD469_1.jpg"},
	}
	body := []byte(listingPageHTML(items, 11))
	got := parseListingItems(body)
	if len(got) != 2 {
		t.Fatalf("parseListingItems returned %d items, want 2", len(got))
	}
	if got[0].code != "MIAD491" || got[1].code != "MIAD469" {
		t.Errorf("codes = [%s, %s], want [MIAD491, MIAD469]", got[0].code, got[1].code)
	}
	if got[0].thumb != "https://cdn.example.com/MIAD491_1.jpg" {
		t.Errorf("thumb = %q", got[0].thumb)
	}
}

func TestParseListingItemsActressFormat(t *testing.T) {
	items := []listingItem{
		{code: "MIBD804", thumb: "https://cdn.example.com/MIBD804_1.jpg"},
		{code: "MIBD749", thumb: "https://cdn.example.com/MIBD749_1.jpg"},
	}
	body := []byte(actressListingPageHTML(items, 3))
	got := parseListingItems(body)
	if len(got) != 2 {
		t.Fatalf("parseListingItems returned %d items, want 2", len(got))
	}
	if got[0].code != "MIBD804" || got[1].code != "MIBD749" {
		t.Errorf("codes = [%s, %s]", got[0].code, got[1].code)
	}
}

func TestParseListingItemsDedup(t *testing.T) {
	body := []byte(`<a href="/works/detail/MIAD491"><img data-src="a.jpg"/></a>` +
		`<a href="/works/detail/MIAD491"><img data-src="b.jpg"/></a>`)
	got := parseListingItems(body)
	if len(got) != 1 {
		t.Errorf("parseListingItems returned %d items, want 1 (dedup)", len(got))
	}
}

// ---- TestExtractTotal ----

func TestExtractTotal(t *testing.T) {
	body := []byte(`<div class="swiper-pagination-02">全152作品中 1 〜 12 タイトルを表示</div>`)
	got := extractTotal(body)
	if got != 152 {
		t.Errorf("extractTotal = %d, want 152", got)
	}
}

func TestExtractTotalNone(t *testing.T) {
	got := extractTotal([]byte(`<html><body>no total here</body></html>`))
	if got != 0 {
		t.Errorf("extractTotal = %d, want 0", got)
	}
}

// ---- TestParseDetail ----

func TestParseDetail(t *testing.T) {
	body := []byte(detailPageHTML(
		"MIAD491",
		"超絶品ボディ",
		"JULIAの肉体の素晴らしさ",
		[]string{"JULIA"},
		"[Jo]Style",
		[]string{"パイズリ", "潮吹き", "巨乳"},
		"超絶品ボディ",
		120,
		"2011-01-13",
	))

	item := listingItem{code: "MIAD491", thumb: "https://cdn.example.com/MIAD491_1.jpg"}
	scene := parseDetail(body, "https://moodyz.com/works/list/series/3482", item, "https://moodyz.com/works/detail/MIAD491")

	if scene.ID != "MIAD491" {
		t.Errorf("ID = %q", scene.ID)
	}
	if scene.SiteID != "moodyz" {
		t.Errorf("SiteID = %q", scene.SiteID)
	}
	if scene.Title != "超絶品ボディ" {
		t.Errorf("Title = %q", scene.Title)
	}
	if scene.Description != "JULIAの肉体の素晴らしさ" {
		t.Errorf("Description = %q", scene.Description)
	}
	if scene.Thumbnail != "https://cdn.example.com/MIAD491_1.jpg" {
		t.Errorf("Thumbnail = %q", scene.Thumbnail)
	}
	if len(scene.Performers) != 1 || scene.Performers[0] != "JULIA" {
		t.Errorf("Performers = %v", scene.Performers)
	}
	if scene.Director != "[Jo]Style" {
		t.Errorf("Director = %q", scene.Director)
	}
	if len(scene.Tags) != 3 {
		t.Fatalf("Tags = %v, want 3", scene.Tags)
	}
	if scene.Series != "超絶品ボディ" {
		t.Errorf("Series = %q", scene.Series)
	}
	if scene.Duration != 7200 {
		t.Errorf("Duration = %d, want 7200", scene.Duration)
	}
	if scene.Date.Year() != 2011 || scene.Date.Month() != 1 || scene.Date.Day() != 13 {
		t.Errorf("Date = %v", scene.Date)
	}
	if scene.Studio != "MOODYZ" {
		t.Errorf("Studio = %q", scene.Studio)
	}
}

// ---- TestListScenes ----

func TestListScenes(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/works/list/series/100":
			if r.URL.Query().Get("page") != "" {
				_, _ = fmt.Fprint(w, listingPageHTML(nil, 0))
				return
			}
			items := []listingItem{
				{code: "MIAD491", thumb: "/img/MIAD491_1.jpg"},
				{code: "MIAD469", thumb: "/img/MIAD469_1.jpg"},
			}
			_, _ = fmt.Fprint(w, listingPageHTML(items, 2))
		case "/works/detail/MIAD491":
			_, _ = fmt.Fprint(w, detailPageHTML("MIAD491", "Title One", "Desc one", []string{"JULIA"}, "Dir A", []string{"Tag1"}, "Series1", 120, "2011-01-13"))
		case "/works/detail/MIAD469":
			_, _ = fmt.Fprint(w, detailPageHTML("MIAD469", "Title Two", "Desc two", []string{"Actress B"}, "Dir B", []string{"Tag2"}, "", 90, "2010-12-01"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client()}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/works/list/series/100", scraper.ListOpts{})
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
	if got["MIAD491"] != "Title One" {
		t.Errorf("MIAD491 title = %q", got["MIAD491"])
	}
	if got["MIAD469"] != "Title Two" {
		t.Errorf("MIAD469 title = %q", got["MIAD469"])
	}
}

// ---- TestListScenesKnownIDs ----

func TestListScenesKnownIDs(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/actress/detail/100":
			if r.URL.Query().Get("page") != "" {
				_, _ = fmt.Fprint(w, actressListingPageHTML(nil, 0))
				return
			}
			items := []listingItem{
				{code: "MIAD491", thumb: "/img/a.jpg"},
				{code: "MIAD469", thumb: "/img/b.jpg"},
				{code: "MIDD633", thumb: "/img/c.jpg"},
			}
			_, _ = fmt.Fprint(w, actressListingPageHTML(items, 3))
		case "/works/detail/MIAD491":
			_, _ = fmt.Fprint(w, detailPageHTML("MIAD491", "Title One", "", []string{"A"}, "", nil, "", 60, "2011-01-01"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client()}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/actress/detail/100", scraper.ListOpts{
		KnownIDs: map[string]bool{"MIAD469": true},
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
	if len(scenes) > 0 && scenes[0].ID != "MIAD491" {
		t.Errorf("scene ID = %q, want MIAD491", scenes[0].ID)
	}
}

// ---- TestListScenesPagination ----

func TestListScenesPagination(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/actress/detail/100":
			page := r.URL.Query().Get("page")
			switch page {
			case "", "1":
				_, _ = fmt.Fprint(w, actressListingPageHTML([]listingItem{
					{code: "MIAD491", thumb: "/img/a.jpg"},
				}, 2))
			case "2":
				_, _ = fmt.Fprint(w, actressListingPageHTML([]listingItem{
					{code: "MIAD469", thumb: "/img/b.jpg"},
				}, 2))
			default:
				_, _ = fmt.Fprint(w, actressListingPageHTML(nil, 0))
			}
		case "/works/detail/MIAD491":
			_, _ = fmt.Fprint(w, detailPageHTML("MIAD491", "T1", "", []string{"A"}, "", nil, "", 60, "2011-01-01"))
		case "/works/detail/MIAD469":
			_, _ = fmt.Fprint(w, detailPageHTML("MIAD469", "T2", "", []string{"B"}, "", nil, "", 90, "2010-12-01"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client()}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/actress/detail/100", scraper.ListOpts{})
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
	if !got["MIAD491"] || !got["MIAD469"] {
		t.Errorf("missing expected scenes: got %v", got)
	}
}
