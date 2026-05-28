package sindrive

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/Wasylq/FSS/scraper"
)

// Trimmed fixture mirroring real sindrive.com / sinx.com markup:
// two cards, one with a "New" badge and one without, plus the page-1
// pagination block that points at page=2 as `last-page`.
const listingHTML = `<html><body>
<div class="container js-load-more-grid">

<figure class=" video_item " data-score="" style="position: relative;">
  <div class="video_item--player">
    <a href="/Bikini-Beach-Balls/movie/14818/bikini-beach-balls-part-2-main-edit" class="let__up-link">
      <img loading="lazy" class="thumb-slide" width="508"
           data-title-image="https://cdn77.example/title.webp"
           src="https://cdn77.example/bbb-14818-main-510x290.jpg"
           alt="Latest deal - Bikini Beach Balls Part 2 - Main Edit">
    </a>
    <a href="/login?return=/videos/all" class="btn-add"><i></i></a>
  </div>
  <div class="video_item--content with-badge">
    <a href="/Bikini-Beach-Balls/movie/14818/bikini-beach-balls-part-2-main-edit" class="link"
       title="Bikini Beach Balls Part 2 - Main Edit">
      <h3 class="title--5" data-fw="400">
        Bikini Beach Balls Part 2 - Main Edit
      </h3>
    </a>
    <div>
      <span class="badge new-badge">New</span>
    </div>
  </div>
  <div class="video_item--channel-link">
    <div class="nc-icon-mini"></div>
    <a href="/Bikini-Beach-Balls" class="link">
      Bikini Beach Balls
    </a>
  </div>
</figure>

<figure class=" video_item " data-score="">
  <div class="video_item--player">
    <a href="/Party-Hardcore-Gone-Crazy-Vol-13/movie/14815/party-hardcore-gone-crazy-vol-13-part-2-main-edit" class="let__up-link">
      <img class="thumb-slide" src="https://cdn77.example/phcc-14815.jpg" alt="...">
    </a>
  </div>
  <div class="video_item--content">
    <a href="/Party-Hardcore-Gone-Crazy-Vol-13/movie/14815/party-hardcore-gone-crazy-vol-13-part-2-main-edit"
       class="link" title="Party Hardcore Gone Crazy Vol. 13 Part 2 - Main Edit">
      <h3 class="title--5">
        Party Hardcore Gone Crazy Vol. 13 Part 2 &amp; More - Main Edit
      </h3>
    </a>
  </div>
  <div class="video_item--channel-link">
    <a href="/Party-Hardcore-Gone-Crazy-Vol-13" class="link">
      Party Hardcore Gone Crazy Vol. 13
    </a>
  </div>
</figure>

</div>

<ul class="pagination clearfix" data-font="mont">
  <li class="current"><a href="#">1</a></li>
  <li class="last-page"><a href="/videos/all?page=2">2</a></li>
</ul>

</body></html>`

// Page-2 fixture: one final card, no last-page link (we're on the last page).
const listing2HTML = `<html><body>
<figure class=" video_item ">
  <div class="video_item--player">
    <a href="/Bikini-Beach-Balls/movie/14755/bikini-beach-balls-part-1-cam-1" class="let__up-link">
      <img class="thumb-slide" src="https://cdn77.example/bbb-14755.jpg">
    </a>
  </div>
  <div class="video_item--content">
    <a href="/Bikini-Beach-Balls/movie/14755/bikini-beach-balls-part-1-cam-1" class="link" title="Bikini Beach Balls Part 1 - Cam 1">
      <h3 class="title--5">Bikini Beach Balls Part 1 - Cam 1</h3>
    </a>
  </div>
  <div class="video_item--channel-link">
    <a href="/Bikini-Beach-Balls" class="link">Bikini Beach Balls</a>
  </div>
</figure>
<ul class="pagination">
  <li class="first-page"><a href="/videos/all?page=1">1</a></li>
  <li class="current"><a href="#">2</a></li>
</ul>
</body></html>`

// Empty page — past the end, zero cards. This is the stop signal.
const emptyHTML = `<html><body><div class="container">no more</div></body></html>`

func TestParseListing(t *testing.T) {
	items := parseListing([]byte(listingHTML))
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}

	first := items[0]
	if first.id != "14818" {
		t.Errorf("ID = %q, want 14818", first.id)
	}
	if first.title != "Bikini Beach Balls Part 2 - Main Edit" {
		t.Errorf("Title = %q", first.title)
	}
	if first.series != "Bikini Beach Balls" {
		t.Errorf("Series = %q, want %q", first.series, "Bikini Beach Balls")
	}
	if first.url != "/Bikini-Beach-Balls/movie/14818/bikini-beach-balls-part-2-main-edit" {
		t.Errorf("URL = %q", first.url)
	}
	if first.thumb != "https://cdn77.example/bbb-14818-main-510x290.jpg" {
		t.Errorf("Thumb = %q", first.thumb)
	}

	// Second card: tests HTML-entity unescape on the title (&amp; → &).
	second := items[1]
	if second.id != "14815" {
		t.Errorf("Second ID = %q, want 14815", second.id)
	}
	if !strings.Contains(second.title, "& More") {
		t.Errorf("Second title = %q, expected &amp; → & unescape", second.title)
	}
}

func TestParseListing_dedupes(t *testing.T) {
	// Duplicated cards should collapse — the listing sometimes repeats the
	// same card markup inside lazy-load placeholders.
	doubled := listingHTML + listingHTML
	items := parseListing([]byte(doubled))
	if len(items) != 2 {
		t.Errorf("got %d items after dedup, want 2", len(items))
	}
}

func TestEstimateTotal(t *testing.T) {
	// Page-1 fixture has last-page=2 → 2 × 2 cards = 4.
	got := estimateTotal([]byte(listingHTML), 2)
	if got != 4 {
		t.Errorf("estimateTotal(page1) = %d, want 4", got)
	}
	// No last-page marker (final page) → falls back to perPage.
	got = estimateTotal([]byte(listing2HTML), 1)
	if got != 1 {
		t.Errorf("estimateTotal(lastpage) = %d, want 1", got)
	}
}

func TestResolveListingPath(t *testing.T) {
	tests := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"", "/videos/all", false},
		{"/", "/videos/all", false},
		{"/videos/all", "/videos/all", false},
		{"/videos/sale", "/videos/sale", false},
		{"/Backstage-Bangers", "/Backstage-Bangers", false},
		{"/channel/Backstage-Bangers/all", "/Backstage-Bangers", false},
		{"/channel/Bash-Bastards/all/", "/Bash-Bastards", false},
		// Detail URL → reject.
		{"/Bikini-Beach-Balls/movie/14818/bikini-beach-balls-part-2-main-edit", "", true},
	}
	for _, tc := range tests {
		got, err := resolveListingPath(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("resolveListingPath(%q) → no error, want one", tc.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("resolveListingPath(%q) → unexpected error: %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("resolveListingPath(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url string
		ok  bool
	}{
		{"https://www.sindrive.com/", true},
		{"https://sindrive.com/videos/all", true},
		{"https://www.sinx.com/Backstage-Bangers", true},
		{"https://www.sinx.com/channel/Allwam/all", true},
		{"http://madsexparty.com/", true},
		{"https://example.com/", false},
		{"https://www.pissfilm.com/", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.ok {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.ok)
		}
	}
}

func TestListingURL_preservesQuery(t *testing.T) {
	s := New()
	got, err := s.listingURL("https://www.sinx.com/videos/all?sort=newest", "/videos/all", 3)
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := url.Parse(got)
	if parsed.Path != "/videos/all" {
		t.Errorf("path = %q", parsed.Path)
	}
	q := parsed.Query()
	if q.Get("page") != "3" {
		t.Errorf("page = %q, want 3", q.Get("page"))
	}
	if q.Get("sort") != "newest" {
		t.Errorf("sort = %q, want newest (preserved from input)", q.Get("sort"))
	}
}

func TestListScenes_endToEnd(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		page := r.URL.Query().Get("page")
		switch {
		case r.URL.Path == "/videos/all" && (page == "" || page == "1"):
			_, _ = fmt.Fprint(w, listingHTML)
		case r.URL.Path == "/videos/all" && page == "2":
			_, _ = fmt.Fprint(w, listing2HTML)
		case r.URL.Path == "/videos/all":
			_, _ = fmt.Fprint(w, emptyHTML)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	s := New()
	ch, err := s.ListScenes(context.Background(), ts.URL+"/", scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}

	var (
		scenes int
		total  int
	)
	for r := range ch {
		switch r.Kind {
		case scraper.KindScene:
			scenes++
			if r.Scene.Studio != "SinDrive" {
				t.Errorf("Studio = %q", r.Scene.Studio)
			}
			if !strings.HasPrefix(r.Scene.URL, ts.URL+"/") {
				t.Errorf("URL = %q (expected scheme+host prefix)", r.Scene.URL)
			}
		case scraper.KindTotal:
			total = r.Total
		case scraper.KindError:
			t.Errorf("unexpected error: %v", r.Err)
		}
	}

	if scenes != 3 {
		t.Errorf("got %d scenes, want 3 (2 from page 1 + 1 from page 2)", scenes)
	}
	if total != 4 {
		t.Errorf("total = %d, want 4 (page1 last-page=2 × 2 cards)", total)
	}
}

func TestListScenes_knownIDsStopsEarly(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		if r.URL.Path == "/videos/all" {
			_, _ = fmt.Fprint(w, listingHTML)
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	s := New()
	ch, err := s.ListScenes(context.Background(), ts.URL+"/videos/all", scraper.ListOpts{
		// Second scene's ID — first scene should pass through, then stop.
		KnownIDs: map[string]bool{"14815": true},
	})
	if err != nil {
		t.Fatal(err)
	}

	var scenes int
	var stoppedEarly bool
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

func TestListScenes_legacyChannelURL(t *testing.T) {
	// /channel/X/all must be rewritten to /X for the actual fetch.
	var fetchedPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fetchedPath = r.URL.Path
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, emptyHTML)
	}))
	defer ts.Close()

	s := New()
	ch, err := s.ListScenes(context.Background(), ts.URL+"/channel/Backstage-Bangers/all", scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}
	for range ch {
	}
	if fetchedPath != "/Backstage-Bangers" {
		t.Errorf("server saw path %q, want /Backstage-Bangers (legacy rewrite failed)", fetchedPath)
	}
}

func TestListScenes_rejectsDetailURL(t *testing.T) {
	s := New()
	ch, err := s.ListScenes(context.Background(),
		"https://www.sinx.com/Bikini-Beach-Balls/movie/14818/bikini-beach-balls-part-2-main-edit",
		scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}
	var sawErr bool
	for r := range ch {
		if r.Kind == scraper.KindError {
			sawErr = true
		}
	}
	if !sawErr {
		t.Error("expected error for detail-page URL input")
	}
}
