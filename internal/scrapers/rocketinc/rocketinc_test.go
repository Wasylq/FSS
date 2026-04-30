package rocketinc

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

func listingPageHTML(slugs []string, nextPage bool, maxPage int) string {
	var sb strings.Builder
	sb.WriteString(`<html><body><section class="lg:w-3/4">`)
	sb.WriteString(`<div class="date-wrapper"><h2>2026年5月7日 発売作品</h2>`)
	sb.WriteString(`<div class="grid grid-cols-2 lg:grid-cols-4 gap-4">`)
	for _, slug := range slugs {
		fmt.Fprintf(&sb, `<div class="works-item"><a href="/works/%s/"><img src="/img/%s.jpg" /></a></div>`, slug, slug)
	}
	sb.WriteString(`</div></div>`)
	if nextPage || maxPage > 1 {
		sb.WriteString(`<nav class="navigation pagination"><div class="nav-links">`)
		for i := 1; i <= maxPage; i++ {
			fmt.Fprintf(&sb, `<a class="page-numbers" href="/works/page/%d/">%d</a>`, i, i)
		}
		if nextPage {
			sb.WriteString(`<a class="next page-numbers" href="/works/page/2/">次へ</a>`)
		}
		sb.WriteString(`</div></nav>`)
	}
	sb.WriteString(`</section></body></html>`)
	return sb.String()
}

func detailPageHTML(slug, title string, performers []string, director string, genres []string, series string, durationMin int, dvdDate string) string {
	var sb strings.Builder
	sb.WriteString(`<html><head>`)
	fmt.Fprintf(&sb, `<meta property="og:image" content="https://rocket-inc.net/wp-content/uploads/%s-jacket.jpg" />`, slug)
	sb.WriteString(`</head><body><article class="single-works">`)
	fmt.Fprintf(&sb, `<header><h1>%s</h1></header>`, title)
	sb.WriteString(`<div class="product-info"><table>`)

	if len(performers) > 0 {
		sb.WriteString(`<tr><th>女優名</th><td>`)
		for i, p := range performers {
			if i > 0 {
				sb.WriteString(`, `)
			}
			fmt.Fprintf(&sb, `<a href="/works_actress/%s/" rel="tag">%s</a>`, p, p)
		}
		sb.WriteString(`</td></tr>`)
	}

	if director != "" {
		fmt.Fprintf(&sb, `<tr><th>監督名</th><td><a href="/works_director/%s/" rel="tag">%s</a></td></tr>`, director, director)
	}

	if len(genres) > 0 {
		sb.WriteString(`<tr><th>ジャンル名</th><td>`)
		for i, g := range genres {
			if i > 0 {
				sb.WriteString(`, `)
			}
			fmt.Fprintf(&sb, `<a href="/works_genre/%s/" rel="tag">%s</a>`, g, g)
		}
		sb.WriteString(`</td></tr>`)
	}

	if series != "" {
		fmt.Fprintf(&sb, `<tr><th>シリーズ名</th><td><a href="/works_series/%s/" rel="tag">%s</a></td></tr>`, series, series)
	}

	fmt.Fprintf(&sb, `<tr><th>品番</th><td>%s</td></tr>`, strings.ToUpper(slug))
	fmt.Fprintf(&sb, `<tr><th>収録時間</th><td>%d分</td></tr>`, durationMin)
	fmt.Fprintf(&sb, `<tr><th>DVD発売日</th><td>%s</td></tr>`, dvdDate)
	sb.WriteString(`</table></div>`)
	sb.WriteString(`<div class="work-introduction"><p>Test description.</p></div>`)
	sb.WriteString(`</article></body></html>`)
	return sb.String()
}

// ---- TestMatchesURL ----

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url   string
		match bool
	}{
		{"https://rocket-inc.net/works/", true},
		{"https://rocket-inc.net/works", true},
		{"https://www.rocket-inc.net/works/", true},
		{"https://rocket-inc.net/works_actress/%e6%96%b0%e6%9d%91%e3%81%82%e3%81%8b%e3%82%8a/", true},
		{"https://rocket-inc.net/works_actress/test-actress/", true},
		{"https://rocket-inc.net/works/rctd-728/", false},
		{"https://rocket-inc.net/", false},
		{"https://example.com/works/", false},
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
		{"https://rocket-inc.net/works/", "https://rocket-inc.net/works/"},
		{"https://rocket-inc.net/works", "https://rocket-inc.net/works/"},
		{"https://rocket-inc.net/works/page/3/", "https://rocket-inc.net/works/"},
		{"https://rocket-inc.net/works_actress/test/page/2/", "https://rocket-inc.net/works_actress/test/"},
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
		{"https://rocket-inc.net/works/", 1, "https://rocket-inc.net/works/"},
		{"https://rocket-inc.net/works/", 2, "https://rocket-inc.net/works/page/2/"},
		{"https://rocket-inc.net/works_actress/test/", 3, "https://rocket-inc.net/works_actress/test/page/3/"},
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
		slug      string
		want      string
	}{
		{"https://rocket-inc.net/works/", "rctd-728", "https://rocket-inc.net/works/rctd-728/"},
		{"https://rocket-inc.net/works_actress/test/", "rcts-050", "https://rocket-inc.net/works/rcts-050/"},
		{"http://localhost:12345/works/", "rctd-728", "http://localhost:12345/works/rctd-728/"},
	}
	for _, c := range cases {
		got := buildDetailURL(c.studioURL, c.slug)
		if got != c.want {
			t.Errorf("buildDetailURL(%q, %q) = %q, want %q", c.studioURL, c.slug, got, c.want)
		}
	}
}

// ---- TestParseListingSlugs ----

func TestParseListingSlugs(t *testing.T) {
	body := []byte(listingPageHTML([]string{"rctd-728", "rcts-050", "rctd-727"}, true, 3))
	slugs := parseListingSlugs(body)
	if len(slugs) != 3 {
		t.Fatalf("parseListingSlugs returned %d items, want 3", len(slugs))
	}
	want := []string{"rctd-728", "rcts-050", "rctd-727"}
	for i, s := range slugs {
		if s != want[i] {
			t.Errorf("slug[%d] = %q, want %q", i, s, want[i])
		}
	}
}

func TestParseListingSlugsDedup(t *testing.T) {
	body := []byte(`<div class="works-item"><a href="/works/rctd-728/"><img /></a></div>
		<div class="works-item"><a href="/works/rctd-728/"><img /></a></div>`)
	slugs := parseListingSlugs(body)
	if len(slugs) != 1 {
		t.Errorf("parseListingSlugs returned %d items, want 1 (dedup)", len(slugs))
	}
}

// ---- TestExtractMaxPage ----

func TestExtractMaxPage(t *testing.T) {
	body := []byte(listingPageHTML([]string{"rctd-728"}, true, 64))
	got := extractMaxPage(body)
	if got != 64 {
		t.Errorf("extractMaxPage = %d, want 64", got)
	}
}

func TestExtractMaxPageNoPagination(t *testing.T) {
	body := []byte(listingPageHTML([]string{"rctd-728"}, false, 0))
	got := extractMaxPage(body)
	if got != 1 {
		t.Errorf("extractMaxPage = %d, want 1", got)
	}
}

// ---- TestHasNextPage ----

func TestHasNextPage(t *testing.T) {
	withNext := []byte(listingPageHTML([]string{"rctd-728"}, true, 3))
	if !hasNextPage(withNext) {
		t.Error("hasNextPage should be true when next page link is present")
	}

	withoutNext := []byte(listingPageHTML([]string{"rctd-728"}, false, 0))
	if hasNextPage(withoutNext) {
		t.Error("hasNextPage should be false when no next page link")
	}
}

// ---- TestParseDetail ----

func TestParseDetail(t *testing.T) {
	body := []byte(detailPageHTML(
		"rctd-728",
		"素人ガチレズビアン",
		[]string{"iqura", "新村あかり", "有村のぞみ"},
		"神戸たろう",
		[]string{"レズ", "企画"},
		"素人ガチレズビアンシリーズ",
		140,
		"2026年05月07日",
	))

	scene := parseDetail(body, "https://rocket-inc.net/works/", "rctd-728", "https://rocket-inc.net/works/rctd-728/")

	if scene.ID != "rctd-728" {
		t.Errorf("ID = %q, want %q", scene.ID, "rctd-728")
	}
	if scene.SiteID != "rocketinc" {
		t.Errorf("SiteID = %q, want %q", scene.SiteID, "rocketinc")
	}
	if scene.Title != "素人ガチレズビアン" {
		t.Errorf("Title = %q, want %q", scene.Title, "素人ガチレズビアン")
	}
	if scene.Thumbnail != "https://rocket-inc.net/wp-content/uploads/rctd-728-jacket.jpg" {
		t.Errorf("Thumbnail = %q", scene.Thumbnail)
	}
	if scene.Description != "Test description." {
		t.Errorf("Description = %q, want %q", scene.Description, "Test description.")
	}
	if len(scene.Performers) != 3 {
		t.Fatalf("Performers = %v, want 3", scene.Performers)
	}
	if scene.Performers[0] != "iqura" || scene.Performers[1] != "新村あかり" || scene.Performers[2] != "有村のぞみ" {
		t.Errorf("Performers = %v", scene.Performers)
	}
	if scene.Director != "神戸たろう" {
		t.Errorf("Director = %q, want %q", scene.Director, "神戸たろう")
	}
	if len(scene.Tags) != 2 {
		t.Fatalf("Tags = %v, want 2", scene.Tags)
	}
	if scene.Tags[0] != "レズ" || scene.Tags[1] != "企画" {
		t.Errorf("Tags = %v", scene.Tags)
	}
	if scene.Series != "素人ガチレズビアンシリーズ" {
		t.Errorf("Series = %q", scene.Series)
	}
	if scene.Duration != 8400 {
		t.Errorf("Duration = %d, want 8400 (140min)", scene.Duration)
	}
	if scene.Date.Year() != 2026 || scene.Date.Month() != 5 || scene.Date.Day() != 7 {
		t.Errorf("Date = %v, want 2026-05-07", scene.Date)
	}
	if scene.Studio != "Rocket" {
		t.Errorf("Studio = %q, want %q", scene.Studio, "Rocket")
	}
}

func TestParseDetailStreamingDate(t *testing.T) {
	body := []byte(`<html><head>
		<meta property="og:image" content="https://example.com/img.jpg" />
		</head><body>
		<h1>Test Title</h1>
		<table>
		<tr><th>収録時間</th><td>90分</td></tr>
		<tr><th>DVD発売日</th><td>2026年05月07日</td></tr>
		<tr><th>先行配信日</th><td>2026年04月09日</td></tr>
		</table>
		</body></html>`)
	scene := parseDetail(body, "https://rocket-inc.net/works/", "test-123", "https://rocket-inc.net/works/test-123/")

	if scene.Date.Month() != 4 || scene.Date.Day() != 9 {
		t.Errorf("Date = %v, want 2026-04-09 (streaming date preferred)", scene.Date)
	}
}

// ---- TestListScenes ----

func TestListScenes(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/works/":
			_, _ = fmt.Fprint(w, listingPageHTML([]string{"rctd-728", "rcts-050"}, false, 1))
		case "/works/rctd-728/":
			_, _ = fmt.Fprint(w, detailPageHTML("rctd-728", "Title One", []string{"Actress A"}, "Dir A", []string{"Tag1"}, "", 120, "2026年05月07日"))
		case "/works/rcts-050/":
			_, _ = fmt.Fprint(w, detailPageHTML("rcts-050", "Title Two", []string{"Actress B"}, "Dir B", []string{"Tag2"}, "", 90, "2026年04月01日"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client()}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/works/", scraper.ListOpts{})
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
	if got["rctd-728"] != "Title One" {
		t.Errorf("rctd-728 title = %q, want %q", got["rctd-728"], "Title One")
	}
	if got["rcts-050"] != "Title Two" {
		t.Errorf("rcts-050 title = %q, want %q", got["rcts-050"], "Title Two")
	}
}

// ---- TestListScenesKnownIDs ----

func TestListScenesKnownIDs(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/works/":
			_, _ = fmt.Fprint(w, listingPageHTML([]string{"rctd-728", "rcts-050", "rctd-727"}, false, 1))
		case "/works/rctd-728/":
			_, _ = fmt.Fprint(w, detailPageHTML("rctd-728", "Title One", []string{"A"}, "", nil, "", 60, "2026年01月01日"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client()}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/works/", scraper.ListOpts{
		KnownIDs: map[string]bool{"rcts-050": true},
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
	if len(scenes) > 0 && scenes[0].ID != "rctd-728" {
		t.Errorf("scene ID = %q, want %q", scenes[0].ID, "rctd-728")
	}
}

// ---- TestListScenesPagination ----

func TestListScenesPagination(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/works/":
			_, _ = fmt.Fprint(w, listingPageHTML([]string{"rctd-728"}, true, 2))
		case "/works/page/2/":
			_, _ = fmt.Fprint(w, listingPageHTML([]string{"rcts-050"}, false, 2))
		case "/works/rctd-728/":
			_, _ = fmt.Fprint(w, detailPageHTML("rctd-728", "Title One", []string{"A"}, "", nil, "", 60, "2026年01月01日"))
		case "/works/rcts-050/":
			_, _ = fmt.Fprint(w, detailPageHTML("rcts-050", "Title Two", []string{"B"}, "", nil, "", 90, "2026年02月01日"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client()}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/works/", scraper.ListOpts{})
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
	if !got["rctd-728"] || !got["rcts-050"] {
		t.Errorf("missing expected scenes: got %v", got)
	}
}
