package karups

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
	"github.com/Wasylq/FSS/scraper"
)

var _ scraper.StudioScraper = (*Scraper)(nil)

const testCard1 = `<div class="item">
    <div class="item-inside">
        <a href="%s/video/test-scene-one-12345.html">
            <div class="thumb">
                <img src="https://media.karups.com/thumbs_pg/012000/012345.jpg" alt="">
                <div class="meta">
                    <span class="title">Test Scene One</span>
                    <span class="date">May 5th, 2026</span>
                </div>
            </div>
        </a>
    </div>
</div>`

const testCard2 = `<div class="item">
    <div class="item-inside">
        <a href="%s/video/another-scene-12346.html">
            <div class="thumb">
                <img src="https://media.karups.com/thumbs_pg/012000/012346.jpg" alt="">
                <div class="meta">
                    <span class="title">Another &amp; Scene</span>
                    <span class="date">Jan 1st, 2026</span>
                </div>
            </div>
        </a>
    </div>
</div>`

const testDetailPage = `<html>
<div class="content-information-meta cf">
    <span class="models">
        <span class="heading">Starring:</span>
        <span class="content"><a href='/models/jane-doe-100.html'>Jane Doe</a></span>
    </span>
</div>
</html>`

const testDetailPage2 = `<html>
<div class="content-information-meta cf">
    <span class="models">
        <span class="heading">Starring:</span>
        <span class="content"><a href='/models/alice-smith-200.html'>Alice Smith</a>, <a href='/models/bob-jones-201.html'>Bob Jones</a></span>
    </span>
</div>
</html>`

func TestParseListingPage(t *testing.T) {
	base := "https://www.karupsow.com"
	body := []byte(fmt.Sprintf(testCard1, base) + fmt.Sprintf(testCard2, base))
	entries := parseListingPage(body)
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}

	e := entries[0]
	if e.id != "12345" {
		t.Errorf("id = %q, want 12345", e.id)
	}
	if e.title != "Test Scene One" {
		t.Errorf("title = %q", e.title)
	}
	if e.thumbnail != "https://media.karups.com/thumbs_pg/012000/012345.jpg" {
		t.Errorf("thumbnail = %q", e.thumbnail)
	}
	if e.date.Format("2006-01-02") != "2026-05-05" {
		t.Errorf("date = %v", e.date)
	}

	e2 := entries[1]
	if e2.title != "Another & Scene" {
		t.Errorf("title = %q, want HTML-unescaped", e2.title)
	}
	if e2.date.Format("2006-01-02") != "2026-01-01" {
		t.Errorf("date = %v", e2.date)
	}
}

func TestParseDate(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"May 5th, 2026", "2026-05-05"},
		{"Jan 1st, 2026", "2026-01-01"},
		{"Mar 22nd, 2026", "2026-03-22"},
		{"Apr 3rd, 2026", "2026-04-03"},
		{"Dec 10th, 2025", "2025-12-10"},
	}
	for _, tt := range tests {
		got := parseDate(tt.input)
		if got.IsZero() {
			t.Errorf("parseDate(%q) = zero", tt.input)
			continue
		}
		if got.Format("2006-01-02") != tt.want {
			t.Errorf("parseDate(%q) = %s, want %s", tt.input, got.Format("2006-01-02"), tt.want)
		}
	}
}

func TestEstimateTotal(t *testing.T) {
	body := []byte(`<a href="page2.html">2</a><a href="page10.html">10</a><a href="page3.html">3</a>`)
	got := estimateTotal(body, 48)
	if got != 480 {
		t.Errorf("estimateTotal = %d, want 480", got)
	}
}

func TestListingURL(t *testing.T) {
	base := "https://www.karupsow.com"
	if got := listingURL(base, 1); got != base+"/videos/" {
		t.Errorf("page 1 = %q", got)
	}
	if got := listingURL(base, 5); got != base+"/videos/page5.html" {
		t.Errorf("page 5 = %q", got)
	}
}

var testPageRe = regexp.MustCompile(`/videos/page(\d+)\.html`)

func newTestServer(base *string) *httptest.Server {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		switch {
		case r.URL.Path == "/videos/" || r.URL.Path == "/videos":
			_, _ = fmt.Fprintf(w, `%s%s<a href="page2.html">2</a>`,
				fmt.Sprintf(testCard1, *base), fmt.Sprintf(testCard2, *base))
		case testPageRe.MatchString(r.URL.Path):
			_, _ = fmt.Fprint(w, "<html></html>")
		case r.URL.Path == "/video/test-scene-one-12345.html":
			_, _ = fmt.Fprint(w, testDetailPage)
		case r.URL.Path == "/video/another-scene-12346.html":
			_, _ = fmt.Fprint(w, testDetailPage2)
		case r.URL.Path == "/model/ellie-nova-6952.html":
			gallery1 := fmt.Sprintf(`<div class="item">
    <div class="item-inside">
        <a href="%s/gallery/gallery-one-99901.html">
            <div class="thumb">
                <img src="https://media.karups.com/thumbs_pg/099000/099901.jpg" alt="">
                <div class="meta">
                    <span class="title">Gallery One</span>
                    <span class="date">May 1st, 2026</span>
                </div>
            </div>
        </a>
    </div>
</div>`, *base)
			video1 := fmt.Sprintf(testCard1, *base)
			video2 := fmt.Sprintf(testCard2, *base)
			_, _ = fmt.Fprintf(w, `<div class="listing-photos cf">%s</div><div class="listing-videos cf">%s%s</div>`,
				gallery1, video1, video2)
		default:
			http.NotFound(w, r)
		}
	}))
	*base = ts.URL
	return ts
}

func TestRun(t *testing.T) {
	var base string
	ts := newTestServer(&base)
	defer ts.Close()

	s := &Scraper{
		client: ts.Client(),
		cfg:    sites[0],
	}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/videos/", scraper.ListOpts{
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
		if scene.SiteID != "karupsow" {
			t.Errorf("scene %d: siteID = %q", i, scene.SiteID)
		}
	}

	if idx, ok := byID["12345"]; ok {
		scene := got[idx]
		if scene.Title != "Test Scene One" {
			t.Errorf("title = %q", scene.Title)
		}
		if len(scene.Performers) != 1 || scene.Performers[0] != "Jane Doe" {
			t.Errorf("performers = %v", scene.Performers)
		}
	} else {
		t.Error("missing scene 12345")
	}

	if idx, ok := byID["12346"]; ok {
		scene := got[idx]
		if len(scene.Performers) != 2 {
			t.Errorf("performers = %v, want 2", scene.Performers)
		}
	} else {
		t.Error("missing scene 12346")
	}
}

func TestKnownIDs(t *testing.T) {
	var base string
	ts := newTestServer(&base)
	defer ts.Close()

	s := &Scraper{
		client: ts.Client(),
		cfg:    sites[0],
	}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/videos/", scraper.ListOpts{
		KnownIDs: map[string]bool{"12346": true},
		Delay:    time.Millisecond,
		Workers:  1,
	})
	if err != nil {
		t.Fatal(err)
	}

	got, stopped := testutil.CollectScenesWithStop(t, ch)
	if len(got) != 1 {
		t.Fatalf("got %d scenes, want 1", len(got))
	}
	if !stopped {
		t.Error("expected StoppedEarly")
	}
}

func TestModelPage(t *testing.T) {
	var base string
	ts := newTestServer(&base)
	defer ts.Close()

	s := &Scraper{
		client: ts.Client(),
		cfg:    sites[0],
	}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/model/ellie-nova-6952.html", scraper.ListOpts{
		Delay:   time.Millisecond,
		Workers: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	got := testutil.CollectScenes(t, ch)
	if len(got) != 2 {
		t.Fatalf("got %d scenes, want 2 (videos only, no galleries)", len(got))
	}
	for _, scene := range got {
		if strings.Contains(scene.URL, "/gallery/") {
			t.Errorf("model page should only return videos, got gallery URL: %s", scene.URL)
		}
	}
}

func TestIsModelURL(t *testing.T) {
	if !isModelURL("https://www.karupsow.com/model/ellie-nova-6952.html") {
		t.Error("should match model URL")
	}
	if isModelURL("https://www.karupsow.com/videos/") {
		t.Error("should not match videos URL")
	}
}

func TestMatchesURL(t *testing.T) {
	tests := []struct {
		id  string
		url string
	}{
		{"karupsow", "https://www.karupsow.com/videos/"},
		{"karupsow", "https://karupsow.com/videos/"},
		{"karupspc", "https://www.karupspc.com/videos/"},
		{"karupsha", "https://www.karupsha.com/videos/"},
	}
	scrapers := make(map[string]*Scraper)
	for _, cfg := range sites {
		scrapers[cfg.id] = newScraper(cfg)
	}
	for _, tt := range tests {
		s := scrapers[tt.id]
		if !s.MatchesURL(tt.url) {
			t.Errorf("%s should match %s", tt.id, tt.url)
		}
	}
}
