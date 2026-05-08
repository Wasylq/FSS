package newsensationsutil

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

var testCfg = SiteConfig{
	SiteID:     "testsite",
	Domain:     "testsite.com",
	SiteBase:   "https://www.testsite.com",
	TourPrefix: "tour_ts",
	StudioName: "Test Studio",
}

var _ scraper.StudioScraper = (*Scraper)(nil)

func TestSplitVideoBlocks(t *testing.T) {
	t.Run("two blocks", func(t *testing.T) {
		html := `<div>header</div>` +
			`<div id="videothumb_100">block one content</div>` +
			`<div id="videothumb_200">block two content</div>` +
			`<div>footer</div>`
		blocks := SplitVideoBlocks(html)
		if len(blocks) != 2 {
			t.Fatalf("got %d blocks, want 2", len(blocks))
		}
		if !strings.Contains(blocks[0], "videothumb_100") {
			t.Errorf("block 0 should contain videothumb_100")
		}
		if !strings.Contains(blocks[1], "videothumb_200") {
			t.Errorf("block 1 should contain videothumb_200")
		}
	})

	t.Run("single block", func(t *testing.T) {
		html := `<div>prefix</div><div id="videothumb_42">only block</div><div>suffix</div>`
		blocks := SplitVideoBlocks(html)
		if len(blocks) != 1 {
			t.Fatalf("got %d blocks, want 1", len(blocks))
		}
		if !strings.Contains(blocks[0], "suffix") {
			t.Errorf("single block should extend to end of page")
		}
	})

	t.Run("no blocks", func(t *testing.T) {
		blocks := SplitVideoBlocks(`<div>nothing here</div>`)
		if blocks != nil {
			t.Errorf("expected nil, got %d blocks", len(blocks))
		}
	})

	t.Run("empty input", func(t *testing.T) {
		blocks := SplitVideoBlocks("")
		if blocks != nil {
			t.Errorf("expected nil for empty input")
		}
	})

	t.Run("three blocks", func(t *testing.T) {
		html := `videothumb_1 aaa videothumb_2 bbb videothumb_3 ccc`
		blocks := SplitVideoBlocks(html)
		if len(blocks) != 3 {
			t.Fatalf("got %d blocks, want 3", len(blocks))
		}
	})
}

func TestParseVideoBlock(t *testing.T) {
	s := New(testCfg)

	t.Run("complete block", func(t *testing.T) {
		block := `videothumb_12345">
<h4><a href="/tour_ts/updates/great-scene.html">Great Scene Title</a></h4>
<span class="tour_update_models"><a href="/models/alice.html">Alice</a>, <a href="/models/bob.html">Bob</a></span>
<img data-src="https://cdn.example.com/thumbs/12345.jpg" />
<video src="https://cdn.example.com/previews/12345.mp4"></video>`

		item := s.parseVideoBlock(block)
		if item.id != "12345" {
			t.Errorf("id = %q, want 12345", item.id)
		}
		if item.url != testCfg.SiteBase+"/tour_ts/updates/great-scene.html" {
			t.Errorf("url = %q", item.url)
		}
		if item.title != "Great Scene Title" {
			t.Errorf("title = %q", item.title)
		}
		if len(item.performers) != 2 || item.performers[0] != "Alice" || item.performers[1] != "Bob" {
			t.Errorf("performers = %v", item.performers)
		}
		if item.thumb != "https://cdn.example.com/thumbs/12345.jpg" {
			t.Errorf("thumb = %q", item.thumb)
		}
		if item.preview != "https://cdn.example.com/previews/12345.mp4" {
			t.Errorf("preview = %q", item.preview)
		}
	})

	t.Run("absolute href", func(t *testing.T) {
		block := `videothumb_999">
<h4><a href="https://other.com/scene.html">Absolute</a></h4>`
		item := s.parseVideoBlock(block)
		if item.url != "https://other.com/scene.html" {
			t.Errorf("url = %q, want absolute URL unchanged", item.url)
		}
	})

	t.Run("no title link", func(t *testing.T) {
		block := `videothumb_888">`
		item := s.parseVideoBlock(block)
		if item.id != "888" {
			t.Errorf("id = %q, want 888", item.id)
		}
		if item.url != "" {
			t.Errorf("url should be empty, got %q", item.url)
		}
	})
}

func TestAbsoluteURL(t *testing.T) {
	s := New(testCfg)
	tests := []struct {
		href, want string
	}{
		{"/tour_ts/updates/scene.html", testCfg.SiteBase + "/tour_ts/updates/scene.html"},
		{"tour_ts/updates/scene.html", testCfg.SiteBase + "/tour_ts/updates/scene.html"},
		{"https://www.testsite.com/tour_ts/updates/scene.html", "https://www.testsite.com/tour_ts/updates/scene.html"},
	}
	for _, tt := range tests {
		if got := s.absoluteURL(tt.href); got != tt.want {
			t.Errorf("absoluteURL(%q) = %q, want %q", tt.href, got, tt.want)
		}
	}
}

func TestMatchesURL(t *testing.T) {
	s := New(SiteConfig{
		SiteID:     "test",
		Domain:     "example.com",
		SiteBase:   "https://www.example.com",
		TourPrefix: "tour_ex",
		StudioName: "Example",
		AltDomains: []string{"alt.com"},
	})

	tests := []struct {
		url  string
		want bool
	}{
		{"https://www.example.com", true},
		{"https://example.com/tour_ex/categories/movies_1_d.html", true},
		{"http://www.alt.com", true},
		{"https://alt.com/page", true},
		{"https://other.com", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := s.MatchesURL(tt.url); got != tt.want {
			t.Errorf("MatchesURL(%q) = %v, want %v", tt.url, got, tt.want)
		}
	}
}

func TestDetailDateRe(t *testing.T) {
	tests := []struct {
		name     string
		html     string
		wantDate string
		wantDur  int
	}{
		{
			name:     "lowercase min",
			html:     `class="sceneDateP"><span>05/01/2026,</span> 200&nbsp;Photos, 34&nbsp;min&nbsp;of video`,
			wantDate: "05/01/2026",
			wantDur:  34,
		},
		{
			name:     "uppercase Mins",
			html:     `class="sceneDateP"><span>08/28/2017,</span> 62&nbsp;Pics, 34&nbsp;Mins&nbsp;of video`,
			wantDate: "08/28/2017",
			wantDur:  34,
		},
		{
			name:     "space separated",
			html:     `class="sceneDateP"><span>12/25/2025,</span> 45 min`,
			wantDate: "12/25/2025",
			wantDur:  45,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := detailDateRe.FindStringSubmatch(tt.html)
			if m == nil {
				t.Fatal("detailDateRe did not match")
			}
			if m[1] != tt.wantDate {
				t.Errorf("date = %q, want %q", m[1], tt.wantDate)
			}
			dur := 0
			fmt.Sscanf(m[2], "%d", &dur)
			if dur != tt.wantDur {
				t.Errorf("duration = %d, want %d", dur, tt.wantDur)
			}
		})
	}
}

func TestDetailDescRe(t *testing.T) {
	t.Run("h2 variant (main site)", func(t *testing.T) {
		html := `<span>Description:</span>  <h2  style="text-transform: none;">This is the description.</h2>`
		m := detailDescRe.FindStringSubmatch(html)
		if m == nil {
			t.Fatal("detailDescRe did not match")
		}
		if strings.TrimSpace(m[1]) != "This is the description." {
			t.Errorf("description = %q", m[1])
		}
	})

	t.Run("p variant (sub-site)", func(t *testing.T) {
		html := `<p><span>Description:</span> Sub-site description text.</p>`
		m := detailDescRe.FindStringSubmatch(html)
		if m == nil {
			t.Fatal("detailDescRe did not match")
		}
		if strings.TrimSpace(m[1]) != "Sub-site description text." {
			t.Errorf("description = %q", m[1])
		}
	})
}

func TestSeriesExtraction(t *testing.T) {
	s := New(testCfg)
	tests := []struct {
		name       string
		keywords   string
		wantSeries string
	}{
		{"series with number", `<meta name="keywords" content="tag1, My Series #3">`, "My Series"},
		{"no series", `<meta name="keywords" content="tag1, tag2">`, ""},
		{"series first", `<meta name="keywords" content="Cool Series #10, tag1">`, "Cool Series"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, series := s.extractTagsAndSeries(tt.keywords)
			if series != tt.wantSeries {
				t.Errorf("series = %q, want %q", series, tt.wantSeries)
			}
		})
	}
}

func TestTagFiltering(t *testing.T) {
	s := New(SiteConfig{
		SiteID:     "familyxxx",
		Domain:     "familyxxx.com",
		SiteBase:   "https://familyxxx.com",
		TourPrefix: "tour_famxxx",
		StudioName: "FamilyXXX",
	})

	page := `<meta name="keywords" content="4K,FamilyXXX.com,Interactive Toys,Movies,NewSensations.com,Photos,Updates,Family XXX,New Sensations,Axel Haze,Sadie Summers">`
	tags, _ := s.extractTagsAndSeries(page)

	for _, tag := range tags {
		if tag == "4K" || tag == "Updates" || tag == "Movies" || tag == "Photos" || tag == "Interactive Toys" {
			t.Errorf("tag %q should be filtered", tag)
		}
		if strings.HasSuffix(tag, ".com") {
			t.Errorf("domain tag %q should be filtered", tag)
		}
	}

	found := false
	for _, tag := range tags {
		if tag == "Family XXX" || tag == "Axel Haze" || tag == "Sadie Summers" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected performer/studio tags, got %v", tags)
	}
}

const videoBlockTpl = `<div id="videothumb_%d">
<h4><a href="%s/tour_ts/updates/scene-%d.html">Scene %d Title</a></h4>
<span class="tour_update_models"><a href="/models/alice.html">Alice</a>, <a href="/models/bob.html">Bob</a></span>
<img data-src="https://cdn.example.com/thumbs/%d.jpg" />
<video src="https://cdn.example.com/previews/%d.mp4"></video>
</div>`

const detailPageTpl = `<html>
<div class="sceneDateP"><span>05/01/2026,</span> 30 min</div>
<span>Description:</span>  <h2  style="text-transform: none;">A great description for scene %d.</h2>
<meta name="keywords" content="tag1, tag2, My Series #3">
</html>`

func buildVideoBlocks(base string, ids []int) string {
	var sb strings.Builder
	for _, id := range ids {
		_, _ = fmt.Fprintf(&sb, videoBlockTpl, id, base, id, id, id, id)
	}
	return sb.String()
}

func newTestServer(base *string, sceneIDs []int) *httptest.Server {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")

		switch {
		case strings.HasSuffix(r.URL.Path, "/models/test-model.html"):
			_, _ = fmt.Fprint(w, buildVideoBlocks(*base, sceneIDs))
		default:
			for _, id := range sceneIDs {
				if r.URL.Path == fmt.Sprintf("/tour_ts/updates/scene-%d.html", id) {
					_, _ = fmt.Fprintf(w, detailPageTpl, id)
					return
				}
			}
			http.NotFound(w, r)
		}
	}))
	*base = ts.URL
	return ts
}

func TestRun(t *testing.T) {
	var base string
	ts := newTestServer(&base, []int{100, 200})
	defer ts.Close()

	cfg := testCfg
	cfg.SiteBase = ts.URL
	s := &Scraper{
		config:  cfg,
		client:  ts.Client(),
		matchRe: New(cfg).matchRe,
	}

	ch, err := s.ListScenes(context.Background(), ts.URL+"/tour_ts/models/test-model.html", scraper.ListOpts{
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
		if scene.SiteID != cfg.SiteID {
			t.Errorf("scene %d: siteID = %q, want %q", i, scene.SiteID, cfg.SiteID)
		}
		if scene.Studio != cfg.StudioName {
			t.Errorf("scene %d: studio = %q, want %q", i, scene.Studio, cfg.StudioName)
		}
	}

	if idx, ok := byID["100"]; ok {
		scene := got[idx]
		if scene.Title != "Scene 100 Title" {
			t.Errorf("title = %q", scene.Title)
		}
		wantDate := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
		if !scene.Date.Equal(wantDate) {
			t.Errorf("date = %v, want %v", scene.Date, wantDate)
		}
		if scene.Duration != 1800 {
			t.Errorf("duration = %d, want 1800", scene.Duration)
		}
		if scene.Description != "A great description for scene 100." {
			t.Errorf("description = %q", scene.Description)
		}
		if scene.Series != "My Series" {
			t.Errorf("series = %q, want My Series", scene.Series)
		}
		if scene.ScrapedAt.IsZero() {
			t.Error("scrapedAt should not be zero")
		}
	} else {
		t.Error("missing scene 100")
	}
}

func TestKnownIDs(t *testing.T) {
	var base string
	ts := newTestServer(&base, []int{100, 200, 300})
	defer ts.Close()

	cfg := testCfg
	cfg.SiteBase = ts.URL
	s := &Scraper{
		config:  cfg,
		client:  ts.Client(),
		matchRe: New(cfg).matchRe,
	}

	ch, err := s.ListScenes(context.Background(), ts.URL+"/tour_ts/models/test-model.html", scraper.ListOpts{
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
	if len(got) != 1 {
		t.Fatalf("got %d scenes, want 1", len(got))
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

	cfg := testCfg
	cfg.SiteBase = ts.URL
	s := &Scraper{
		config:  cfg,
		client:  ts.Client(),
		matchRe: New(cfg).matchRe,
	}

	ch, err := s.ListScenes(context.Background(), ts.URL+"/tour_ts/models/empty.html", scraper.ListOpts{
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

func TestPagination(t *testing.T) {
	page1IDs := []int{100, 200}
	page2IDs := []int{300}

	var base string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")

		switch r.URL.Path {
		case "/tour_ts/categories/movies_1_d.html":
			_, _ = fmt.Fprint(w, buildVideoBlocks(base, page1IDs))
		case "/tour_ts/categories/movies_2_d.html":
			_, _ = fmt.Fprint(w, buildVideoBlocks(base, page2IDs))
		case "/tour_ts/categories/movies_3_d.html":
			_, _ = fmt.Fprint(w, "<html>empty</html>")
		default:
			for _, id := range append(page1IDs, page2IDs...) {
				if r.URL.Path == fmt.Sprintf("/tour_ts/updates/scene-%d.html", id) {
					_, _ = fmt.Fprintf(w, detailPageTpl, id)
					return
				}
			}
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()
	base = ts.URL

	cfg := testCfg
	cfg.SiteBase = ts.URL
	s := &Scraper{
		config:  cfg,
		client:  ts.Client(),
		matchRe: New(cfg).matchRe,
	}

	ch, err := s.ListScenes(context.Background(), ts.URL+"/tour_ts/categories/movies_1_d.html", scraper.ListOpts{
		Delay:   time.Millisecond,
		Workers: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	got := testutil.CollectScenes(t, ch)
	if len(got) != 3 {
		t.Fatalf("got %d scenes, want 3 (2 from page 1 + 1 from page 2)", len(got))
	}
}
