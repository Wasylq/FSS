package groobyutil

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Wasylq/FSS/internal/parseutil"
	"github.com/Wasylq/FSS/internal/scrapers/testutil"
	"github.com/Wasylq/FSS/scraper"
)

var _ scraper.StudioScraper = (*Scraper)(nil)

var testCfg = SiteConfig{
	SiteID:     "testsite",
	Domain:     "testsite.com",
	StudioName: "Test Studio",
	TourPrefix: "/tour",
}

const testCardHTML = `<div class="sexyvideo">
	<div class="setbg">
		<a href="https://www.testsite.com/tour/trailers/my-scene.html">
			<img id="set-target-12345" class="mainThumb thumbs stdimage" src="/content/thumbs/12345.jpg">
		</a>
	</div>
	<h4><a href="https://www.testsite.com/tour/trailers/my-scene.html" title="Amazing Scene Title">Amazing Scene Title</a></h4>
	<div class="modelname"><a href="/tour/models/jane-doe.html"><span class="text-center">Jane Doe</span></a></div>
	<div class="modelname"><a href="/tour/models/john-smith.html"><span class="text-center">John Smith</span></a></div>
	<p class="photodesc">A great scene description here.</p>
	<p class="dateadded"><span><i class='fas fa-video'></i> <div class="duration-div">16:56&nbsp;HD&nbsp;Video</div></span> <i class='far fa-calendar' style='margin-left:10px;'></i> 8th May 2026</p>
</div>`

const testCardMinimal = `<div class="sexyvideo">
	<div class="setbg">
		<a href="/tour/trailers/solo.html">
			<img id="set-target-999" class="mainThumb thumbs stdimage" src="/content/thumbs/999.jpg">
		</a>
	</div>
	<h4><a href="/tour/trailers/solo.html" title="Solo Scene">Solo Scene</a></h4>
	<div class="modelname"><a href="/tour/models/performer.html"><span class="text-center">Solo Performer</span></a></div>
	<p class="dateadded"><span><i class='fas fa-video'></i> <div class="duration-div">1:05:30&nbsp;HD&nbsp;Video</div></span> <i class='far fa-calendar' style='margin-left:10px;'></i> 23rd January 2025</p>
</div>`

func TestParseListingPage(t *testing.T) {
	body := []byte(testCardHTML + testCardMinimal)
	scenes := parseListingPage(body)
	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2", len(scenes))
	}

	s := scenes[0]
	if s.id != "12345" {
		t.Errorf("id = %q, want 12345", s.id)
	}
	if s.title != "Amazing Scene Title" {
		t.Errorf("title = %q, want %q", s.title, "Amazing Scene Title")
	}
	if s.url != "https://www.testsite.com/tour/trailers/my-scene.html" {
		t.Errorf("url = %q", s.url)
	}
	if s.thumb != "/content/thumbs/12345.jpg" {
		t.Errorf("thumb = %q", s.thumb)
	}
	if s.date.Format("2006-01-02") != "2026-05-08" {
		t.Errorf("date = %v, want 2026-05-08", s.date)
	}
	if s.duration != 1016 {
		t.Errorf("duration = %d, want 1016", s.duration)
	}
	if len(s.performers) != 2 || s.performers[0] != "Jane Doe" || s.performers[1] != "John Smith" {
		t.Errorf("performers = %v, want [Jane Doe John Smith]", s.performers)
	}
	if s.description != "A great scene description here." {
		t.Errorf("description = %q", s.description)
	}

	s2 := scenes[1]
	if s2.id != "999" {
		t.Errorf("id = %q, want 999", s2.id)
	}
	if s2.title != "Solo Scene" {
		t.Errorf("title = %q, want Solo Scene", s2.title)
	}
	if s2.duration != 3930 {
		t.Errorf("duration = %d, want 3930 (1:05:30)", s2.duration)
	}
	if len(s2.performers) != 1 || s2.performers[0] != "Solo Performer" {
		t.Errorf("performers = %v, want [Solo Performer]", s2.performers)
	}
	if s2.date.Format("2006-01-02") != "2025-01-23" {
		t.Errorf("date = %v, want 2025-01-23", s2.date)
	}
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		in   string
		want int
	}{
		{"16:56", 1016},
		{"5:00", 300},
		{"0:30", 30},
		{"1:05:30", 3930},
		{"2:00:00", 7200},
	}
	for _, tt := range tests {
		if got := parseutil.ParseDurationColon(tt.in); got != tt.want {
			t.Errorf("parseutil.ParseDurationColon(%q) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestParseGroobyDate(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"8th May 2026", "2026-05-08"},
		{"1st January 2025", "2025-01-01"},
		{"2nd February 2024", "2024-02-02"},
		{"3rd March 2023", "2023-03-03"},
		{"23rd January 2025", "2025-01-23"},
		{"11th November 2020", "2020-11-11"},
		{"21st December 2019", "2019-12-21"},
	}
	for _, tt := range tests {
		got := parseGroobyDate(tt.in)
		if got.Format("2006-01-02") != tt.want {
			t.Errorf("parseGroobyDate(%q) = %v, want %s", tt.in, got, tt.want)
		}
	}
}

func TestEstimateTotal(t *testing.T) {
	body := []byte(`<a href="movies_5_d.html">5</a><a href="movies_20_d.html">20</a>`)
	if got := estimateTotal(body, 12); got != 240 {
		t.Errorf("estimateTotal = %d, want 240", got)
	}
}

func TestMatchesURL(t *testing.T) {
	s := New(SiteConfig{
		SiteID:     "test",
		Domain:     "black-tgirls.com",
		StudioName: "Black TGirls",
		TourPrefix: "/tour",
	})

	tests := []struct {
		url  string
		want bool
	}{
		{"https://www.black-tgirls.com/tour/", true},
		{"https://black-tgirls.com/tour/", true},
		{"http://www.black-tgirls.com/", true},
		{"https://other-site.com/", false},
	}
	for _, tt := range tests {
		if got := s.MatchesURL(tt.url); got != tt.want {
			t.Errorf("MatchesURL(%q) = %v, want %v", tt.url, got, tt.want)
		}
	}
}

func TestMatchesURLAltDomains(t *testing.T) {
	s := New(SiteConfig{
		SiteID:     "test",
		Domain:     "groobyvr.com",
		StudioName: "Grooby VR",
		TourPrefix: "/tour",
		AltDomains: []string{"justvr.xxx"},
	})

	if !s.MatchesURL("https://www.justvr.xxx/tour/") {
		t.Error("expected alt domain justvr.xxx to match")
	}
	if !s.MatchesURL("https://www.groobyvr.com/tour/") {
		t.Error("expected primary domain to match")
	}
}

const cardTpl = `<div class="sexyvideo">
	<div class="setbg">
		<a href="/tour/trailers/scene-%d.html">
			<img id="set-target-%d" class="mainThumb thumbs stdimage" src="/content/thumbs/%d.jpg">
		</a>
	</div>
	<h4><a href="/tour/trailers/scene-%d.html" title="Scene %d">Scene %d</a></h4>
	<div class="modelname"><a href="/tour/models/test.html"><span class="text-center">Test</span></a></div>
	<p class="dateadded"><span><i class='fas fa-video'></i> <div class="duration-div">10:00&nbsp;HD&nbsp;Video</div></span> <i class='far fa-calendar' style='margin-left:10px;'></i> 1st January 2025</p>
</div>`

func buildTestPage(ids []int, maxPage int) []byte {
	var sb strings.Builder
	pager := ""
	for p := 2; p <= maxPage; p++ {
		pager += fmt.Sprintf(`<a href="movies_%d_d.html">%d</a>`, p, p)
	}
	sb.WriteString(pager)
	for _, id := range ids {
		fmt.Fprintf(&sb, cardTpl, id, id, id, id, id, id)
	}
	return []byte(sb.String())
}

var testPageNumRe = regexp.MustCompile(`_(\d+)_d\.html`)

func extractPageNum(path string) int {
	if m := testPageNumRe.FindStringSubmatch(path); m != nil {
		p, _ := strconv.Atoi(m[1])
		return p
	}
	return 1
}

func newTestServer(pages [][]int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")

		switch {
		case strings.Contains(r.URL.Path, "/models/"):
			_, _ = w.Write(buildTestPage(pages[0], 1))
		default:
			pageNum := extractPageNum(r.URL.Path)
			idx := pageNum - 1
			if idx >= 0 && idx < len(pages) {
				_, _ = w.Write(buildTestPage(pages[idx], len(pages)))
			} else {
				_, _ = fmt.Fprint(w, `<div>empty</div>`)
			}
		}
	}))
}

func testScraper(ts *httptest.Server) *Scraper {
	s := New(testCfg)
	s.base = ts.URL
	return s
}

func TestRun(t *testing.T) {
	ts := newTestServer([][]int{{100, 200}})
	defer ts.Close()

	s := testScraper(ts)

	ch, err := s.ListScenes(context.Background(), ts.URL+"/tour/", scraper.ListOpts{
		Delay: time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}

	got := testutil.CollectScenes(t, ch)
	if len(got) != 2 {
		t.Fatalf("got %d scenes, want 2", len(got))
	}
	if got[0].Title != "Scene 100" {
		t.Errorf("title = %q, want Scene 100", got[0].Title)
	}
	if got[0].SiteID != "testsite" {
		t.Errorf("siteID = %q, want testsite", got[0].SiteID)
	}
	if got[0].Studio != "Test Studio" {
		t.Errorf("studio = %q, want Test Studio", got[0].Studio)
	}
}

func TestKnownIDs(t *testing.T) {
	ts := newTestServer([][]int{{1, 2, 3, 4}})
	defer ts.Close()

	s := testScraper(ts)

	ch, err := s.ListScenes(context.Background(), ts.URL+"/tour/", scraper.ListOpts{
		KnownIDs: map[string]bool{"3": true},
		Delay:    time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}

	got, stopped := testutil.CollectScenesWithStop(t, ch)
	if len(got) != 2 {
		t.Fatalf("got %d scenes, want 2", len(got))
	}
	if !stopped {
		t.Error("expected StoppedEarly")
	}
}

func TestPagination(t *testing.T) {
	page1 := []int{10, 20, 30}
	page2 := []int{40, 50}

	ts := newTestServer([][]int{page1, page2})
	defer ts.Close()

	s := testScraper(ts)

	ch, err := s.ListScenes(context.Background(), ts.URL+"/tour/", scraper.ListOpts{
		Delay: time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}

	got := testutil.CollectScenes(t, ch)
	if len(got) != 5 {
		t.Fatalf("got %d scenes, want 5", len(got))
	}
}

func TestModelPage(t *testing.T) {
	ts := newTestServer([][]int{{10, 20, 30}})
	defer ts.Close()

	s := testScraper(ts)

	ch, err := s.ListScenes(context.Background(), ts.URL+"/tour/models/TestModel.html", scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}

	got := testutil.CollectScenes(t, ch)
	if len(got) != 3 {
		t.Fatalf("got %d scenes, want 3", len(got))
	}
}

func TestToSceneRelativeURLs(t *testing.T) {
	item := sceneItem{
		id:    "42",
		title: "Test",
		url:   "/tour/trailers/test.html",
		thumb: "/content/thumbs/42.jpg",
	}
	scene := item.toScene("testid", "Test Studio", "https://www.example.com", time.Now())
	if scene.URL != "https://www.example.com/tour/trailers/test.html" {
		t.Errorf("URL = %q, want absolute", scene.URL)
	}
	if scene.Thumbnail != "https://www.example.com/content/thumbs/42.jpg" {
		t.Errorf("Thumbnail = %q, want absolute", scene.Thumbnail)
	}
}

func TestToSceneAbsoluteURLs(t *testing.T) {
	item := sceneItem{
		id:    "42",
		title: "Test",
		url:   "https://www.example.com/tour/trailers/test.html",
		thumb: "https://cdn.example.com/thumbs/42.jpg",
	}
	scene := item.toScene("testid", "Test Studio", "https://www.example.com", time.Now())
	if scene.URL != "https://www.example.com/tour/trailers/test.html" {
		t.Errorf("URL = %q, should not be modified", scene.URL)
	}
	if scene.Thumbnail != "https://cdn.example.com/thumbs/42.jpg" {
		t.Errorf("Thumbnail = %q, should not be modified", scene.Thumbnail)
	}
}
