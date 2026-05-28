package adultdoorwayutil

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/Wasylq/FSS/scraper"
)

// Fixtures derived from facialabuse.com/tour/ + a sample trailer page.

const listingHTMLVideothumb = `<html><body>
<div class="item-updates">
  <div class="row g-1">

    <div class="item-update no-overlay col-xxl-2 col-xl-3 col-lg-6 col-md-6 col-sm-6 col-xs-12">
      <div class="item-thumb item-thumb-videothumb">
        <div class="item-video-thumb b7132_videothumb_512" data-videosrc="https://cdn.example/v1.mp4" data-videoposter="https://cdn.example/poster1.webp" data-usepriority="1">
          <a href="https://tour5m.facialabuse.com/tour/trailers/pale-in-the-tail.html" title="Pale in the Tail"><span class="item-icon"><i class="fa fa-play-circle"></i></span></a>
          <img src="https://cdn.example/poster1.webp" alt="Pale in the Tail" class="video_placeholder" />
        </div>
      </div>
      <div class="item-footer">
        <div class="item-row">
          <div class="item-title">
            <a href="https://tour5m.facialabuse.com/tour/trailers/pale-in-the-tail.html" title="Pale in the Tail">Pale in the Tail</a>
          </div>
        </div>
        <div class="item-row flex-wrap">
          <div class="item-date">Runtime: 01:02:47 | 991 Photos</div>
        </div>
      </div>
    </div><!--//item-update-->

    <div class="item-update no-overlay col-xxl-2 col-xl-3 col-lg-6 col-md-6 col-sm-6 col-xs-12">
      <div class="item-thumb item-thumb-videothumb">
        <div class="item-video-thumb b7125_videothumb_158" data-videosrc="https://cdn.example/v2.mp4" data-videoposter="https://cdn.example/poster2.webp">
          <a href="https://tour5m.facialabuse.com/tour/trailers/filthiest-depths-of-depravity.html" title="Filthiest Depths of Depravity"><span class="item-icon"><i class="fa fa-play-circle"></i></span></a>
        </div>
      </div>
      <div class="item-footer">
        <div class="item-row">
          <div class="item-title">
            <a href="https://tour5m.facialabuse.com/tour/trailers/filthiest-depths-of-depravity.html" title="Filthiest Depths of Depravity">Filthiest Depths of Depravity</a>
          </div>
        </div>
        <div class="item-row flex-wrap">
          <div class="item-date">Runtime: 45:18</div>
        </div>
      </div>
    </div><!--//item-update-->

  </div>
</div>

<ul class="pagination">
  <li class="active"><a href="movies_1_d.html">1</a></li>
  <li><a href="movies_2_d.html">2</a></li>
  <li><a href="movies_50_d.html">50</a></li>
</ul>
</body></html>`

// Plain-thumb variant — POV Hotel style. The card has no videothumb wrapper;
// the image carries the `stdimage update_thumb thumbs` class.
const listingHTMLPlainThumb = `<html><body>
<div class="item-update no-overlay col-xxl-2 col-xl-3">
  <div class="item-thumb">
    <a href="https://tour5m.povhotel.com/tour/trailers/ph-cheyenne.html" title="Cheyenne">
      <span class="item-icon"><i class="fa fa-play-circle"></i></span>
      <img alt="povhotel_videos_cheyenne" class="stdimage update_thumb thumbs" src="https://cdn.example/cheyenne.jpg" />
    </a>
  </div>
  <div class="item-footer">
    <div class="item-row">
      <div class="item-title">
        <a href="https://tour5m.povhotel.com/tour/trailers/ph-cheyenne.html" title="Cheyenne">Cheyenne</a>
      </div>
    </div>
    <div class="item-row flex-wrap">
      <div class="item-date">Runtime: 19:44</div>
    </div>
  </div>
</div><!--//item-update-->
</body></html>`

const detailHTML = `<html><body>
<div class="update-info-block">
  <h1 class="highlight">Pale in the Tail</h1>
  <div class="update-info-row text-gray">
    <a href="https://tour5m.facialabuse.com/tour/" title="FacialAbuse"><span>FacialAbuse</span></a>
  </div>
  <div class="update-info-row text-gray"><strong>Added:</strong> May 26, 2026 | Runtime: 01:02:47 | 991 Photos</div>
  <div class="update-info-block">The redheaded slut drops to her knees as two thick cocks slide past her mouth. Her tongue laps at the throbbing shafts.</div>
  <div class="update-info-block">
    <ul class="tags">
      <li><a href="https://tour5m.facialabuse.com/tour/categories/Anal_1_d.html">Anal</a></li>
      <li><a href="https://tour5m.facialabuse.com/tour/categories/blowjobs_1_d.html">Blowjobs</a></li>
      <li><a href="https://tour5m.facialabuse.com/tour/categories/deep-throat_1_d.html">Deep Throat</a></li>
    </ul>
  </div>
</div>
</body></html>`

const emptyListingHTML = `<html><body><div class="item-updates"></div></body></html>`

func testConfig(base string) SiteConfig {
	return SiteConfig{
		ID:       "facialabuse",
		SiteBase: base,
		Studio:   "Facial Abuse",
		Patterns: []string{"facialabuse.com"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?facialabuse\.com`),
	}
}

func TestParseListing_videothumbCards(t *testing.T) {
	items := parseListing([]byte(listingHTMLVideothumb))
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}

	first := items[0]
	if first.id != "pale-in-the-tail" {
		t.Errorf("ID = %q, want pale-in-the-tail", first.id)
	}
	if first.title != "Pale in the Tail" {
		t.Errorf("Title = %q", first.title)
	}
	if first.url != "https://tour5m.facialabuse.com/tour/trailers/pale-in-the-tail.html" {
		t.Errorf("URL = %q", first.url)
	}
	if first.thumb != "https://cdn.example/poster1.webp" {
		t.Errorf("Thumb = %q", first.thumb)
	}
	// 01:02:47 = 3767s
	if first.duration != 3767 {
		t.Errorf("Duration = %d, want 3767", first.duration)
	}

	second := items[1]
	if second.id != "filthiest-depths-of-depravity" {
		t.Errorf("Second ID = %q", second.id)
	}
	// 45:18 = 2718s
	if second.duration != 2718 {
		t.Errorf("Second Duration = %d, want 2718", second.duration)
	}
}

func TestParseListing_plainThumbCards(t *testing.T) {
	items := parseListing([]byte(listingHTMLPlainThumb))
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	got := items[0]
	if got.id != "ph-cheyenne" {
		t.Errorf("ID = %q", got.id)
	}
	if got.thumb != "https://cdn.example/cheyenne.jpg" {
		t.Errorf("Thumb = %q, want plain stdimage src", got.thumb)
	}
}

func TestParseListing_dedupesRepeatedSlugs(t *testing.T) {
	// Detail pages and listing pages both render "More Updates" carousels that
	// re-emit the same item-update cards. Make sure we don't double-count.
	doubled := listingHTMLVideothumb + listingHTMLVideothumb
	items := parseListing([]byte(doubled))
	if len(items) != 2 {
		t.Fatalf("got %d items after dedup, want 2", len(items))
	}
}

func TestEnrichFromDetail(t *testing.T) {
	var item sceneItem
	item.id = "pale-in-the-tail"
	item.title = "Pale in the Tail" // listing's title — should be preserved
	enrichFromDetail([]byte(detailHTML), &item)

	if item.title != "Pale in the Tail" {
		t.Errorf("Title = %q", item.title)
	}
	if item.date.Year() != 2026 || item.date.Month() != 5 || item.date.Day() != 26 {
		t.Errorf("Date = %v, want 2026-05-26", item.date)
	}
	if item.duration != 3767 {
		t.Errorf("Duration = %d", item.duration)
	}
	if !strings.HasPrefix(item.description, "The redheaded slut drops to her knees") {
		t.Errorf("Description prefix wrong: %q", item.description)
	}
	want := []string{"Anal", "Blowjobs", "Deep Throat"}
	if len(item.tags) != len(want) {
		t.Fatalf("Tags = %v, want %v", item.tags, want)
	}
	for i, w := range want {
		if item.tags[i] != w {
			t.Errorf("Tags[%d] = %q, want %q", i, item.tags[i], w)
		}
	}
}

func TestEstimateTotal(t *testing.T) {
	// 2 cards × max-page 50 = 100
	got := estimateTotal([]byte(listingHTMLVideothumb), 2)
	if got != 100 {
		t.Errorf("estimateTotal = %d, want 100", got)
	}
}

func TestParseStudioURL(t *testing.T) {
	tests := []struct {
		url      string
		wantMode listMode
		wantSlug string
	}{
		{"https://facialabuse.com/", modeFullCatalog, ""},
		{"https://tour5m.facialabuse.com/tour/", modeFullCatalog, ""},
		{"https://tour5m.facialabuse.com/tour/categories/movies.html", modeFullCatalog, ""},
		{"https://tour5m.facialabuse.com/tour/categories/Anal_1_d.html", modeCategory, "Anal"},
		{"https://tour5m.facialabuse.com/tour/categories/throat-fucking_3_d.html", modeCategory, "throat-fucking"},
	}
	for _, c := range tests {
		t.Run(c.url, func(t *testing.T) {
			got := parseStudioURL(c.url)
			if got.mode != c.wantMode || got.slug != c.wantSlug {
				t.Errorf("got {mode=%d, slug=%q}, want {mode=%d, slug=%q}", got.mode, got.slug, c.wantMode, c.wantSlug)
			}
		})
	}
}

func TestListConfig_pageURL(t *testing.T) {
	tests := []struct {
		lc   listConfig
		page int
		want string
	}{
		{listConfig{mode: modeFullCatalog}, 1, "https://example.com/tour/categories/movies_1_d.html"},
		{listConfig{mode: modeFullCatalog}, 50, "https://example.com/tour/categories/movies_50_d.html"},
		{listConfig{mode: modeCategory, slug: "Anal"}, 2, "https://example.com/tour/categories/Anal_2_d.html"},
	}
	for _, c := range tests {
		got := c.lc.pageURL("https://example.com", c.page)
		if got != c.want {
			t.Errorf("got %q, want %q", got, c.want)
		}
	}
}

func TestMatchesURL(t *testing.T) {
	s := New(testConfig("https://facialabuse.com"))
	cases := []struct {
		url   string
		match bool
	}{
		{"https://facialabuse.com/tour/", true},
		{"https://www.facialabuse.com", true},
		{"https://tour5m.facialabuse.com/tour/categories/movies_1_d.html", false}, // tour5m. host — handled by redirect, not matched directly
		{"https://example.com/", false},
	}
	for _, c := range cases {
		t.Run(c.url, func(t *testing.T) {
			got := s.MatchesURL(c.url)
			if got != c.match {
				t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.match)
			}
		})
	}
}

// TestListScenes_endToEnd exercises the full pagination + worker-pool flow
// against an in-process server.
func TestListScenes_endToEnd(t *testing.T) {
	hits := struct {
		listingPages map[string]int
		details      map[string]int
	}{
		listingPages: map[string]int{},
		details:      map[string]int{},
	}

	var ts *httptest.Server
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		// Fixture HTML has absolute URLs pointing at the real site host
		// (tour5m.facialabuse.com). Rewrite them to ts.URL so the worker
		// pool's detail-fetch stays in-process.
		rewrite := func(s string) string {
			return strings.ReplaceAll(s, "https://tour5m.facialabuse.com", ts.URL)
		}
		switch {
		case strings.HasPrefix(r.URL.Path, "/tour/categories/movies_1_d.html"):
			hits.listingPages[r.URL.Path]++
			_, _ = fmt.Fprint(w, rewrite(listingHTMLVideothumb))
		case strings.HasPrefix(r.URL.Path, "/tour/categories/movies_2_d.html"):
			hits.listingPages[r.URL.Path]++
			_, _ = fmt.Fprint(w, emptyListingHTML)
		case strings.HasPrefix(r.URL.Path, "/tour/trailers/"):
			hits.details[r.URL.Path]++
			_, _ = fmt.Fprint(w, detailHTML)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	s := New(SiteConfig{
		ID:       "facialabuse",
		SiteBase: ts.URL,
		Studio:   "Facial Abuse",
		Patterns: []string{"facialabuse.com"},
		MatchRe:  regexp.MustCompile(`.*`),
	})

	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{Workers: 2})
	if err != nil {
		t.Fatal(err)
	}

	var (
		scenes   int
		titles   []string
		sawTotal bool
	)
	for r := range ch {
		switch r.Kind {
		case scraper.KindScene:
			scenes++
			titles = append(titles, r.Scene.Title)
			if r.Scene.Description == "" {
				t.Errorf("scene %q has empty description — detail fetch didn't run", r.Scene.Title)
			}
			if r.Scene.Date.Year() != 2026 {
				t.Errorf("scene %q has zero date — detail fetch didn't populate", r.Scene.Title)
			}
		case scraper.KindTotal:
			sawTotal = true
		case scraper.KindError:
			t.Errorf("unexpected error: %v", r.Err)
		}
	}

	if scenes != 2 {
		t.Errorf("got %d scenes, want 2 (titles=%v)", scenes, titles)
	}
	if !sawTotal {
		t.Error("expected a Progress (Total) message")
	}
	if hits.details["/tour/trailers/pale-in-the-tail.html"] == 0 {
		t.Error("detail page for pale-in-the-tail was never fetched")
	}
}

func TestListScenes_knownIDsStopsEarly(t *testing.T) {
	var ts *httptest.Server
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		rewrite := func(s string) string {
			return strings.ReplaceAll(s, "https://tour5m.facialabuse.com", ts.URL)
		}
		switch {
		case strings.HasPrefix(r.URL.Path, "/tour/categories/movies_1_d.html"):
			_, _ = fmt.Fprint(w, rewrite(listingHTMLVideothumb))
		case strings.HasPrefix(r.URL.Path, "/tour/trailers/"):
			_, _ = fmt.Fprint(w, detailHTML)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	s := New(SiteConfig{
		ID:       "facialabuse",
		SiteBase: ts.URL,
		Studio:   "Facial Abuse",
		Patterns: []string{"facialabuse.com"},
		MatchRe:  regexp.MustCompile(`.*`),
	})

	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{
		Workers:  1,
		KnownIDs: map[string]bool{"filthiest-depths-of-depravity": true},
	})
	if err != nil {
		t.Fatal(err)
	}

	var (
		scenes       int
		stoppedEarly bool
	)
	for r := range ch {
		switch r.Kind {
		case scraper.KindScene:
			scenes++
		case scraper.KindStoppedEarly:
			stoppedEarly = true
		case scraper.KindError:
			t.Errorf("unexpected error: %v", r.Err)
		}
	}

	if scenes != 1 {
		t.Errorf("got %d scenes, want 1 (stopped before known ID)", scenes)
	}
	if !stoppedEarly {
		t.Error("expected StoppedEarly signal")
	}
}
