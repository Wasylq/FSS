package newsensations

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

var _ scraper.StudioScraper = (*Scraper)(nil)

// ---------------------------------------------------------------------------
// splitVideoBlocks
// ---------------------------------------------------------------------------

func TestSplitVideoBlocks(t *testing.T) {
	t.Run("two blocks", func(t *testing.T) {
		html := `<div>header</div>` +
			`<div id="videothumb_100">block one content</div>` +
			`<div id="videothumb_200">block two content</div>` +
			`<div>footer</div>`
		blocks := splitVideoBlocks(html)
		if len(blocks) != 2 {
			t.Fatalf("got %d blocks, want 2", len(blocks))
		}
		// First block contains "100" and "block one"
		if !contains(blocks[0], "videothumb_100") {
			t.Errorf("block 0 should contain videothumb_100, got %q", blocks[0])
		}
		// Second block contains "200" and "block two"
		if !contains(blocks[1], "videothumb_200") {
			t.Errorf("block 1 should contain videothumb_200, got %q", blocks[1])
		}
	})

	t.Run("single block", func(t *testing.T) {
		html := `<div>prefix</div><div id="videothumb_42">only block</div><div>suffix</div>`
		blocks := splitVideoBlocks(html)
		if len(blocks) != 1 {
			t.Fatalf("got %d blocks, want 1", len(blocks))
		}
		if !contains(blocks[0], "videothumb_42") {
			t.Errorf("block should contain videothumb_42")
		}
		// Single block extends to the end of the page.
		if !contains(blocks[0], "suffix") {
			t.Errorf("single block should extend to end of page")
		}
	})

	t.Run("no blocks", func(t *testing.T) {
		blocks := splitVideoBlocks(`<div>nothing here</div>`)
		if blocks != nil {
			t.Errorf("expected nil, got %d blocks", len(blocks))
		}
	})

	t.Run("empty input", func(t *testing.T) {
		blocks := splitVideoBlocks("")
		if blocks != nil {
			t.Errorf("expected nil for empty input")
		}
	})

	t.Run("three blocks", func(t *testing.T) {
		html := `videothumb_1 aaa videothumb_2 bbb videothumb_3 ccc`
		blocks := splitVideoBlocks(html)
		if len(blocks) != 3 {
			t.Fatalf("got %d blocks, want 3", len(blocks))
		}
		if !contains(blocks[0], "videothumb_1") {
			t.Errorf("block 0 missing id")
		}
		if !contains(blocks[1], "videothumb_2") {
			t.Errorf("block 1 missing id")
		}
		if !contains(blocks[2], "videothumb_3") {
			t.Errorf("block 2 missing id")
		}
		// Last block extends to end of string.
		if !contains(blocks[2], "ccc") {
			t.Errorf("last block should extend to end of page")
		}
	})
}

// ---------------------------------------------------------------------------
// parseVideoBlock
// ---------------------------------------------------------------------------

func TestParseVideoBlock(t *testing.T) {
	t.Run("complete block", func(t *testing.T) {
		block := `videothumb_12345">
<h4><a href="/tour_ns/trailers/great-scene.html">Great Scene Title</a></h4>
<span class="tour_update_models"><a href="/models/alice.html">Alice</a>, <a href="/models/bob.html">Bob</a></span>
<img data-src="https://cdn.newsensations.com/thumbs/12345.jpg" />
<video src="https://cdn.newsensations.com/previews/12345.mp4"></video>`

		item := parseVideoBlock(block)
		if item.id != "12345" {
			t.Errorf("id = %q, want 12345", item.id)
		}
		if item.url != siteBase+"/tour_ns/trailers/great-scene.html" {
			t.Errorf("url = %q", item.url)
		}
		if item.title != "Great Scene Title" {
			t.Errorf("title = %q", item.title)
		}
		if len(item.performers) != 2 {
			t.Fatalf("performers = %v, want 2", item.performers)
		}
		if item.performers[0] != "Alice" {
			t.Errorf("performers[0] = %q, want Alice", item.performers[0])
		}
		if item.performers[1] != "Bob" {
			t.Errorf("performers[1] = %q, want Bob", item.performers[1])
		}
		if item.thumb != "https://cdn.newsensations.com/thumbs/12345.jpg" {
			t.Errorf("thumb = %q", item.thumb)
		}
		if item.preview != "https://cdn.newsensations.com/previews/12345.mp4" {
			t.Errorf("preview = %q", item.preview)
		}
	})

	t.Run("absolute href", func(t *testing.T) {
		block := `videothumb_999">
<h4><a href="https://www.newsensations.com/tour_ns/trailers/abs.html">Absolute</a></h4>`

		item := parseVideoBlock(block)
		if item.url != "https://www.newsensations.com/tour_ns/trailers/abs.html" {
			t.Errorf("url = %q, want absolute URL unchanged", item.url)
		}
	})

	t.Run("single performer", func(t *testing.T) {
		block := `videothumb_555">
<h4><a href="/scene.html">Single</a></h4>
<span class="tour_update_models"><a href="/models/solo.html">Solo Star</a></span>`

		item := parseVideoBlock(block)
		if len(item.performers) != 1 || item.performers[0] != "Solo Star" {
			t.Errorf("performers = %v, want [Solo Star]", item.performers)
		}
	})

	t.Run("no performers", func(t *testing.T) {
		block := `videothumb_777">
<h4><a href="/scene.html">No Models</a></h4>`

		item := parseVideoBlock(block)
		if len(item.performers) != 0 {
			t.Errorf("performers = %v, want empty", item.performers)
		}
	})

	t.Run("no title link", func(t *testing.T) {
		block := `videothumb_888">`
		item := parseVideoBlock(block)
		if item.id != "888" {
			t.Errorf("id = %q, want 888", item.id)
		}
		if item.url != "" {
			t.Errorf("url should be empty without title link, got %q", item.url)
		}
		if item.title != "" {
			t.Errorf("title should be empty, got %q", item.title)
		}
	})

	t.Run("no thumb", func(t *testing.T) {
		block := `videothumb_111">
<h4><a href="/scene.html">No Thumb</a></h4>`

		item := parseVideoBlock(block)
		if item.thumb != "" {
			t.Errorf("thumb should be empty, got %q", item.thumb)
		}
	})

	t.Run("no preview", func(t *testing.T) {
		block := `videothumb_222">
<h4><a href="/scene.html">No Preview</a></h4>
<img data-src="https://cdn.example.com/thumb.jpg" />`

		item := parseVideoBlock(block)
		if item.preview != "" {
			t.Errorf("preview should be empty, got %q", item.preview)
		}
	})

	t.Run("mp4 in src only", func(t *testing.T) {
		block := `videothumb_333">
<h4><a href="/scene.html">Preview</a></h4>
<source src="https://cdn.example.com/clip.mp4?token=abc" />`

		item := parseVideoBlock(block)
		if item.preview != "https://cdn.example.com/clip.mp4?token=abc" {
			t.Errorf("preview = %q", item.preview)
		}
	})
}

// ---------------------------------------------------------------------------
// absoluteURL
// ---------------------------------------------------------------------------

func TestAbsoluteURL(t *testing.T) {
	tests := []struct {
		name string
		href string
		want string
	}{
		{
			name: "relative with leading slash",
			href: "/tour_ns/trailers/scene.html",
			want: siteBase + "/tour_ns/trailers/scene.html",
		},
		{
			name: "relative without leading slash",
			href: "tour_ns/trailers/scene.html",
			want: siteBase + "/tour_ns/trailers/scene.html",
		},
		{
			name: "already absolute http",
			href: "http://www.newsensations.com/tour_ns/trailers/scene.html",
			want: "http://www.newsensations.com/tour_ns/trailers/scene.html",
		},
		{
			name: "already absolute https",
			href: "https://www.newsensations.com/tour_ns/trailers/scene.html",
			want: "https://www.newsensations.com/tour_ns/trailers/scene.html",
		},
		{
			name: "absolute different domain",
			href: "https://cdn.newsensations.com/img/thumb.jpg",
			want: "https://cdn.newsensations.com/img/thumb.jpg",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := absoluteURL(tt.href)
			if got != tt.want {
				t.Errorf("absoluteURL(%q) = %q, want %q", tt.href, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// MatchesURL
// ---------------------------------------------------------------------------

func TestMatchesURL(t *testing.T) {
	s := New()
	tests := []struct {
		url  string
		want bool
	}{
		{"https://www.newsensations.com", true},
		{"https://www.newsensations.com/tour_ns/categories/movies_1_d.html", true},
		{"https://newsensations.com/tour_ns/models/alice.html", true},
		{"http://www.newsensations.com/tour_ns/trailers/scene.html", true},
		{"https://example.com", false},
		{"https://example.com/newsensations.com", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := s.MatchesURL(tt.url); got != tt.want {
			t.Errorf("MatchesURL(%q) = %v, want %v", tt.url, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// nextPageRe — pagination detection
// ---------------------------------------------------------------------------

func TestNextPageRe(t *testing.T) {
	tests := []struct {
		name  string
		html  string
		match bool
	}{
		{"next link", `<a href="movies_2_d.html">next</a>`, true},
		{"raquo link", `<a href="movies_3_d.html">&raquo;</a>`, true},
		{"rsaquo link", `<a href="movies_4_d.html">›</a>`, true},
		{"next with whitespace", `<a href="movies_5_d.html">  next  </a>`, true},
		{"numbered but no next", `<a href="movies_2_d.html">2</a>`, false},
		{"no pagination", `<div>content only</div>`, false},
		{"different sort", `<a href="movies_2_n.html">next</a>`, true},
		{"sort o", `<a href="movies_2_o.html">&raquo;</a>`, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := nextPageRe.MatchString(tt.html); got != tt.match {
				t.Errorf("nextPageRe.MatchString(%q) = %v, want %v", tt.html, got, tt.match)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// httptest-based run tests
// ---------------------------------------------------------------------------

// videoBlockTpl produces a videothumb block with hrefs pointing to the test server.
const videoBlockTpl = `<div id="videothumb_%d">
<h4><a href="%s/tour_ns/trailers/scene-%d.html">Scene %d Title</a></h4>
<span class="tour_update_models"><a href="/models/alice.html">Alice</a>, <a href="/models/bob.html">Bob</a></span>
<img data-src="https://cdn.newsensations.com/thumbs/%d.jpg" />
<video src="https://cdn.newsensations.com/previews/%d.mp4"></video>
</div>`

const detailPageTpl = `<html>
<div class="sceneDateP"><span>05/01/2026,</span> 30 min</div>
<div class="sceneRight"><h1>Scene %d Title</h1><h2>A great description for scene %d.</h2></div>
<meta name="keywords" content="tag1, tag2, My Series #3">
</html>`

func newNSTestServer(base *string, sceneIDs []int) *httptest.Server {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")

		switch r.URL.Path {
		case "/tour_ns/models/test-model.html":
			var sb fmt.Stringer = buildVideoBlocks(*base, sceneIDs)
			_, _ = fmt.Fprint(w, sb)

		default:
			// Detail pages: /tour_ns/trailers/scene-{id}.html
			for _, id := range sceneIDs {
				if r.URL.Path == fmt.Sprintf("/tour_ns/trailers/scene-%d.html", id) {
					_, _ = fmt.Fprintf(w, detailPageTpl, id, id)
					return
				}
			}
			http.NotFound(w, r)
		}
	}))
	*base = ts.URL
	return ts
}

type htmlString string

func (h htmlString) String() string { return string(h) }

func buildVideoBlocks(base string, ids []int) htmlString {
	var s string
	for _, id := range ids {
		s += fmt.Sprintf(videoBlockTpl, id, base, id, id, id, id)
	}
	return htmlString(s)
}

func TestRun(t *testing.T) {
	var base string
	ts := newNSTestServer(&base, []int{100, 200})
	defer ts.Close()

	s := &Scraper{client: ts.Client()}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/tour_ns/models/test-model.html", scraper.ListOpts{
		Delay:   time.Millisecond,
		Workers: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	got := testutil.CollectScenes(t, ch)
	if len(got) != 2 {
		t.Fatalf("got %d scenes, want 2", len(got))
	}

	byID := make(map[string]int)
	for i, scene := range got {
		byID[scene.ID] = i
		if scene.SiteID != siteID {
			t.Errorf("scene %d: siteID = %q, want %q", i, scene.SiteID, siteID)
		}
		if scene.Studio != studioName {
			t.Errorf("scene %d: studio = %q, want %q", i, scene.Studio, studioName)
		}
	}

	if idx, ok := byID["100"]; ok {
		scene := got[idx]
		if scene.Title != "Scene 100 Title" {
			t.Errorf("title = %q, want Scene 100 Title", scene.Title)
		}
		if len(scene.Performers) != 2 || scene.Performers[0] != "Alice" || scene.Performers[1] != "Bob" {
			t.Errorf("performers = %v, want [Alice Bob]", scene.Performers)
		}
		wantDate := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
		if !scene.Date.Equal(wantDate) {
			t.Errorf("date = %v, want %v", scene.Date, wantDate)
		}
		if scene.Duration != 1800 {
			t.Errorf("duration = %d, want 1800 (30 min * 60)", scene.Duration)
		}
		if scene.Description != "A great description for scene 100." {
			t.Errorf("description = %q", scene.Description)
		}
		if scene.Thumbnail != "https://cdn.newsensations.com/thumbs/100.jpg" {
			t.Errorf("thumbnail = %q", scene.Thumbnail)
		}
		if scene.Preview != "https://cdn.newsensations.com/previews/100.mp4" {
			t.Errorf("preview = %q", scene.Preview)
		}
		if scene.Series != "My Series" {
			t.Errorf("series = %q, want My Series", scene.Series)
		}
		// Tags should include tag1, tag2, My Series #3 (minus filtered keywords).
		if len(scene.Tags) < 2 {
			t.Errorf("tags = %v, want at least [tag1 tag2]", scene.Tags)
		}
		if scene.ScrapedAt.IsZero() {
			t.Error("scrapedAt should not be zero")
		}
		wantURL := ts.URL + "/tour_ns/trailers/scene-100.html"
		if scene.URL != wantURL {
			t.Errorf("url = %q, want %q", scene.URL, wantURL)
		}
	} else {
		t.Error("missing scene 100")
	}

	if _, ok := byID["200"]; !ok {
		t.Error("missing scene 200")
	}
}

func TestKnownIDs(t *testing.T) {
	var base string
	ts := newNSTestServer(&base, []int{100, 200, 300})
	defer ts.Close()

	s := &Scraper{client: ts.Client()}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/tour_ns/models/test-model.html", scraper.ListOpts{
		KnownIDs: map[string]bool{"200": true},
		Delay:    time.Millisecond,
		Workers:  1,
	})
	if err != nil {
		t.Fatal(err)
	}

	got, stopped := testutil.CollectScenesWithStop(t, ch)
	if !stopped {
		t.Error("expected StoppedEarly signal")
	}
	// Scene 100 is before 200 in the listing, so it should be fetched.
	// Scene 200 is the known ID, so scraping stops before fetching it.
	if len(got) != 1 {
		t.Fatalf("got %d scenes, want 1 (only scene before the known ID)", len(got))
	}
	if got[0].ID != "100" {
		t.Errorf("expected scene 100, got %q", got[0].ID)
	}
}

func TestRunEmptyPage(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, "<html><body>No videos here</body></html>")
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client()}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/tour_ns/models/empty.html", scraper.ListOpts{
		Delay:   time.Millisecond,
		Workers: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	got := testutil.CollectScenes(t, ch)
	if len(got) != 0 {
		t.Errorf("got %d scenes from empty page, want 0", len(got))
	}
}

// ---------------------------------------------------------------------------
// Detail page regex tests
// ---------------------------------------------------------------------------

func TestDetailDateRe(t *testing.T) {
	tests := []struct {
		name     string
		html     string
		wantDate string
		wantDur  int
	}{
		{
			name:     "standard format",
			html:     `<div class="sceneDateP"><span>05/01/2026,</span> 30 min</div>`,
			wantDate: "2026-05-01",
			wantDur:  30,
		},
		{
			name:     "nbsp between number and min",
			html:     `<div class="sceneDateP"><span>12/25/2025,</span> 45&nbsp;min</div>`,
			wantDate: "2025-12-25",
			wantDur:  45,
		},
		{
			name:     "single digit duration",
			html:     `<div class="sceneDateP"><span>01/15/2026,</span> 5 min</div>`,
			wantDate: "2026-01-15",
			wantDur:  5,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := detailDateRe.FindStringSubmatch(tt.html)
			if m == nil {
				t.Fatal("detailDateRe did not match")
			}
			if m[1] != tt.wantDate[:5]+"/"+tt.wantDate[8:]+"/"+tt.wantDate[:4] {
				// Parse with the scraper's format to verify.
				parsed, err := time.Parse("01/02/2006", m[1])
				if err != nil {
					t.Fatalf("failed to parse date %q: %v", m[1], err)
				}
				if parsed.Format("2006-01-02") != tt.wantDate {
					t.Errorf("date = %s, want %s", parsed.Format("2006-01-02"), tt.wantDate)
				}
			}
			if m[2] != fmt.Sprintf("%d", tt.wantDur) {
				t.Errorf("duration = %q, want %d", m[2], tt.wantDur)
			}
		})
	}
}

func TestDetailDescRe(t *testing.T) {
	t.Run("with description", func(t *testing.T) {
		html := `<div class="sceneRight"><h1>Title</h1><h2>This is the description.</h2></div>`
		m := detailDescRe.FindStringSubmatch(html)
		if m == nil {
			t.Fatal("detailDescRe did not match")
		}
		if m[1] != "This is the description." {
			t.Errorf("description = %q", m[1])
		}
	})

	t.Run("without h2", func(t *testing.T) {
		html := `<div class="sceneRight"><h1>Title Only</h1></div>`
		m := detailDescRe.FindStringSubmatch(html)
		// Should match but m[1] should be empty.
		if m != nil && m[1] != "" {
			t.Errorf("expected empty description, got %q", m[1])
		}
	})
}

func TestDetailKeywordsRe(t *testing.T) {
	html := `<meta name="keywords" content="tag1, tag2, My Series #3, New Sensations">`
	m := detailKeywordsRe.FindStringSubmatch(html)
	if m == nil {
		t.Fatal("detailKeywordsRe did not match")
	}
	if m[1] != "tag1, tag2, My Series #3, New Sensations" {
		t.Errorf("keywords = %q", m[1])
	}
}

func TestSeriesExtraction(t *testing.T) {
	// Simulating the series extraction logic from fetchDetail.
	tests := []struct {
		name       string
		keywords   string
		wantSeries string
	}{
		{
			name:       "series with number",
			keywords:   "tag1, tag2, My Series #3",
			wantSeries: "My Series",
		},
		{
			name:       "no series",
			keywords:   "tag1, tag2, tag3",
			wantSeries: "",
		},
		{
			name:       "series first",
			keywords:   "Cool Series #10, tag1",
			wantSeries: "Cool Series",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var series string
			parts := splitKeywords(tt.keywords)
			for _, p := range parts {
				if contains(p, "#") {
					idx := indexOf(p, "#")
					series = trimSpace(p[:idx])
					break
				}
			}
			if series != tt.wantSeries {
				t.Errorf("series = %q, want %q", series, tt.wantSeries)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func contains(s, sub string) bool {
	return len(s) >= len(sub) && searchString(s, sub)
}

func searchString(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func indexOf(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func splitKeywords(s string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	parts = append(parts, s[start:])
	return parts
}

func trimSpace(s string) string {
	i, j := 0, len(s)
	for i < j && s[i] == ' ' {
		i++
	}
	for j > i && s[j-1] == ' ' {
		j--
	}
	return s[i:j]
}
