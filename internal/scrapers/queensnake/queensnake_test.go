package queensnake

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wasylq/FSS/scraper"
)

func TestMatchesURL(t *testing.T) {
	s := New()
	yes := []string{
		"https://queensnake.com",
		"https://queensnake.com/",
		"https://www.queensnake.com",
		"https://queensnake.com/previewmovies/0",
		"https://queensnake.com/previewmovie/some-slug",
	}
	no := []string{
		"https://example.com",
		"https://queensect.com",
	}

	for _, u := range yes {
		if !s.MatchesURL(u) {
			t.Errorf("expected match for %s", u)
		}
	}
	for _, u := range no {
		if s.MatchesURL(u) {
			t.Errorf("unexpected match for %s", u)
		}
	}
}

const testBlock = `<div class="contentFilmNameSeal" data-filmid="sombre-safari-nazryana" data-isonline="1"" data-onlinedate="2026 April 25">
<div class="contentFilmNameSealLink" data-filmid="sombre-safari-nazryana"><span class="contentFilmName">
SOMBRE SAFARI - NAZRYANA</span><span class="contentFileDate">
2026 April 25 • 21 min</span></div></div><div class="contentPreviewWrapper">
<div class="contentPreviewMovieRow">
<div class="contentPreviewMain">
<div class="contentPreviewMainWrapper" title="sombre-safari-nazryana"></div></div>
<div class="contentPreviewRightColumn"><div class="contentPreviewRightImageWrap"><a href="#">
<div class="contentPreviewRightImage contentPreviewImagesWrapper">
<img src="https://cdn.queensnake.com/preview/sombre-safari-nazryana/sombre-safari-nazryana-prev4.jpg?v=7ddf478e" srcset="https://cdn.queensnake.com/preview/sombre-safari-nazryana/sombre-safari-nazryana-prev0-160.jpg?v=7ddf478e 160w, https://cdn.queensnake.com/preview/sombre-safari-nazryana/sombre-safari-nazryana-prev0-2560.jpg?v=7ddf478e 2560w, https://cdn.queensnake.com/preview/sombre-safari-nazryana/sombre-safari-nazryana-prev0.jpg?v=7ddf478e 3840w">
</div></a></div></div></div>
<div class="contentPreviewDescription">
      After Nazryana left my pussy in ruins.<br><br>I grabbed my whip.
    </div><div class="contentPreviewTags">
<a href="https://queensnake.com/tag/Nazryana">Nazryana</a><a href="https://queensnake.com/tag/Queensnake">Queensnake</a><a href="https://queensnake.com/tag/sand">sand</a><a href="https://queensnake.com/tag/whipping">whipping</a><a href="https://queensnake.com/tag/4k-uhd">4k-uhd</a></div></div>`

func TestParseSceneBlocks(t *testing.T) {
	scenes := parseSceneBlocks([]byte(testBlock))
	if len(scenes) != 1 {
		t.Fatalf("got %d scenes, want 1", len(scenes))
	}

	ps := scenes[0]
	if ps.filmID != "sombre-safari-nazryana" {
		t.Errorf("filmID = %q", ps.filmID)
	}
	if ps.title != "SOMBRE SAFARI - NAZRYANA" {
		t.Errorf("title = %q", ps.title)
	}
	if ps.date != "2026 April 25" {
		t.Errorf("date = %q", ps.date)
	}
	if ps.duration != "21 min" {
		t.Errorf("duration = %q", ps.duration)
	}
	if !strings.Contains(ps.description, "After Nazryana left my pussy in ruins.") {
		t.Errorf("description = %q", ps.description)
	}
	if !strings.Contains(ps.description, "I grabbed my whip.") {
		t.Errorf("description missing second paragraph: %q", ps.description)
	}
	if len(ps.tags) != 5 || ps.tags[0] != "Nazryana" {
		t.Errorf("tags = %v", ps.tags)
	}
	if ps.thumbnail != "https://cdn.queensnake.com/preview/sombre-safari-nazryana/sombre-safari-nazryana-prev0-2560.jpg?v=7ddf478e" {
		t.Errorf("thumbnail = %q", ps.thumbnail)
	}
}

func TestToScene(t *testing.T) {
	ps := parsedScene{
		filmID:      "sombre-safari-nazryana",
		title:       "SOMBRE SAFARI - NAZRYANA",
		date:        "2026 April 25",
		duration:    "21 min",
		description: "A test description.",
		tags:        []string{"Nazryana", "Queensnake", "sand", "whipping"},
		thumbnail:   "https://cdn.queensnake.com/preview/sombre-safari-nazryana/sombre-safari-nazryana-prev0-2560.jpg?v=abc",
	}
	now := time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC)
	scene := toScene(ps, "https://queensnake.com", "https://queensnake.com/", now)

	if scene.ID != "sombre-safari-nazryana" {
		t.Errorf("ID = %q", scene.ID)
	}
	if scene.SiteID != "queensnake" {
		t.Errorf("SiteID = %q", scene.SiteID)
	}
	if scene.URL != "https://queensnake.com/previewmovie/sombre-safari-nazryana" {
		t.Errorf("URL = %q", scene.URL)
	}
	if scene.Duration != 1260 {
		t.Errorf("Duration = %d, want 1260", scene.Duration)
	}
	if scene.Studio != "Queensnake" {
		t.Errorf("Studio = %q", scene.Studio)
	}
	expected := time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC)
	if !scene.Date.Equal(expected) {
		t.Errorf("Date = %v, want %v", scene.Date, expected)
	}
}

func TestExtractPerformers(t *testing.T) {
	cases := []struct {
		title string
		tags  []string
		want  []string
	}{
		{
			"SOMBRE SAFARI - NAZRYANA",
			[]string{"Nazryana", "Queensnake", "sand", "whipping"},
			[]string{"Nazryana"},
		},
		{
			"KICK CHICK HOLLY",
			[]string{"Holly", "Queensnake", "kicking"},
			[]string{"Holly"},
		},
		{
			"SINGLE WORD",
			[]string{"unrelated", "tags"},
			nil,
		},
	}
	for _, tc := range cases {
		got := extractPerformers(tc.title, tc.tags)
		if len(got) != len(tc.want) {
			t.Errorf("extractPerformers(%q, %v) = %v, want %v", tc.title, tc.tags, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("extractPerformers(%q, %v)[%d] = %q, want %q", tc.title, tc.tags, i, got[i], tc.want[i])
			}
		}
	}
}

func TestParseDate(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"2026 April 25", "2026-04-25"},
		{"2025 December 31", "2025-12-31"},
		{"", "0001-01-01"},
		{"bad date", "0001-01-01"},
	}
	for _, tc := range cases {
		got := parseDate(tc.in)
		if got.Format("2006-01-02") != tc.want {
			t.Errorf("parseDate(%q) = %s, want %s", tc.in, got.Format("2006-01-02"), tc.want)
		}
	}
}

func TestParseDuration(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"21 min", 1260},
		{"30 min", 1800},
		{"0 min", 0},
		{"", 0},
	}
	for _, tc := range cases {
		got := parseDuration(tc.in)
		if got != tc.want {
			t.Errorf("parseDuration(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestCleanDescription(t *testing.T) {
	input := "  First paragraph.<br><br>\n      Second paragraph.<br/>\n      Third.  "
	got := cleanDescription(input)
	if !strings.Contains(got, "First paragraph.") {
		t.Errorf("missing first paragraph: %q", got)
	}
	if !strings.Contains(got, "Second paragraph.") {
		t.Errorf("missing second paragraph: %q", got)
	}
	if strings.Contains(got, "<br") {
		t.Errorf("still contains <br>: %q", got)
	}
}

func TestEstimateTotal(t *testing.T) {
	body := []byte(`<div class="pagerWrapper">
<a href="https://queensnake.com/previewmovies/0" class="pagerSelected">1</a>
<a href="https://queensnake.com/previewmovies/1" class="pager">2</a>
<a href="https://queensnake.com/previewmovies/158" class="pager">159</a>
<a href="https://queensnake.com/previewmovies/1" class="pagerarrowRight"></a>
</div>`)
	got := estimateTotal(body, 10)
	if got != 1590 {
		t.Errorf("estimateTotal = %d, want 1590", got)
	}
}

func TestHasNextPage(t *testing.T) {
	with := []byte(`<a href="..." class="pagerarrowRight"></a>`)
	without := []byte(`<a href="..." class="pager">5</a>`)

	if !hasNextPage(with) {
		t.Error("expected hasNextPage = true")
	}
	if hasNextPage(without) {
		t.Error("expected hasNextPage = false")
	}
}

func makeListingPage(scenes int, maxPage int, hasNext bool) string {
	var b strings.Builder
	b.WriteString("<html><body>")
	for i := 0; i < scenes; i++ {
		slug := fmt.Sprintf("scene-%d", i)
		fmt.Fprintf(&b, `<div class="contentFilmNameSeal" data-filmid="%s" data-isonline="1"" data-onlinedate="2026 April %d">
<div class="contentFilmNameSealLink" data-filmid="%s"><span class="contentFilmName">
SCENE %d</span><span class="contentFileDate">
2026 April %d • 20 min</span></div></div>
<div class="contentPreviewWrapper">
<div class="contentPreviewMovieRow">
<div class="contentPreviewMain"><div class="contentPreviewMainWrapper"><img src="https://cdn.queensnake.com/preview/%s/%s-prev0.jpg?v=abc123" srcset="https://cdn.queensnake.com/preview/%s/%s-prev0-2560.jpg?v=abc123 2560w"></div></div>
</div>
<div class="contentPreviewDescription">Description %d.</div>
<div class="contentPreviewTags"><a href="/tag/Tag%d">Tag%d</a></div>
</div>`, slug, i+1, slug, i, i+1, slug, slug, slug, slug, i, i, i)
	}
	b.WriteString(`<div class="pagerWrapper">`)
	for p := 0; p <= maxPage; p++ {
		fmt.Fprintf(&b, `<a href="https://example.com/previewmovies/%d" class="pager">%d</a>`, p, p+1)
	}
	if hasNext {
		b.WriteString(`<a href="..." class="pagerarrowRight"></a>`)
	}
	b.WriteString(`</div></body></html>`)
	return b.String()
}

func TestPaginatedScrape(t *testing.T) {
	page0 := makeListingPage(3, 1, true)
	page1 := makeListingPage(2, 1, false)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/1") {
			_, _ = fmt.Fprint(w, page1)
		} else {
			_, _ = fmt.Fprint(w, page0)
		}
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client(), siteBase: ts.URL}
	ch, _ := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{})

	var scenes int
	var total int
	for r := range ch {
		if r.Total > 0 {
			total = r.Total
			continue
		}
		if r.Err != nil {
			t.Fatal(r.Err)
		}
		scenes++
	}

	if total != 6 {
		t.Errorf("total = %d, want 6", total)
	}
	if scenes != 5 {
		t.Errorf("scenes = %d, want 5", scenes)
	}
}

func TestKnownIDsStopsEarly(t *testing.T) {
	page := makeListingPage(5, 2, true)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, page)
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client(), siteBase: ts.URL}
	ch, _ := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{
		KnownIDs: map[string]bool{"scene-2": true},
	})

	var scenes int
	var stopped bool
	for r := range ch {
		if r.StoppedEarly {
			stopped = true
			continue
		}
		if r.Total > 0 {
			continue
		}
		if r.Err != nil {
			t.Fatal(r.Err)
		}
		scenes++
	}

	if !stopped {
		t.Error("expected StoppedEarly")
	}
	if scenes != 2 {
		t.Errorf("scenes = %d, want 2", scenes)
	}
}

func TestScraperInterface(t *testing.T) {
	var _ scraper.StudioScraper = New()
}
