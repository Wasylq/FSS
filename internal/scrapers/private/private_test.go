package private

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wasylq/FSS/scraper"
)

// Fixture derived from real private.com /scenes markup. Two cards with all
// fields populated plus a pagination block showing 50 as the highest page.
const listingHTML = `<html><body>
<ul class='thumb-list thumb-grid scenes'>

<li class="card">
  <div class="scene">
    <a data-track-action="NAVIGATION" data-track="SCENE_LINK" href="https://www.private.com/scene/kore-fox-starts-a-threesome/27142?utm=foo" title="…">
      <picture>
        <img loading="lazy" src="https://pcom77.st-content.com/content/contentthumbs/506037.jpg" alt="…" class="thumbs_onhover">
      </picture>
      <span class="overThumb mobile_trailer">
        <video class="mini_video_player">
          <source src="https://pcoms77.st-content.com/content/upload/SPE/SPE504/SPE504_s03/trailers/SPE504_s03_trailer_02.mp4" type="video/mp4">
        </video>
      </span>
    </a>
    <ul class="scene-details clearfix">
      <li class="ultrahdlabel"><a><span>4K</span></a></li>
    </ul>
    <div class="desc-scene">
      <h3>
        <a data-track-action="NAVIGATION" data-track="TITLE_LINK" href="https://www.private.com/scene/kore-fox-starts-a-threesome/27142">Kore Fox Starts A Threesome &amp; More</a>
      </h3>
      <ul class="scene-models">
        <li><a data-track="PORNSTAR_LINK" href="/pornstar/8391-kore-fox/">Kore Fox</a></li>
      </ul>
      <span class="scene-date">04/04/2026</span>
    </div>
  </div>
</li>

<li class="card">
  <div class="scene">
    <a data-track-action="NAVIGATION" data-track="SCENE_LINK" href="https://www.private.com/scene/two-girls-one-cock/27139">
      <picture>
        <img loading="lazy" src="https://pcom77.st-content.com/content/contentthumbs/506100.jpg">
      </picture>
    </a>
    <div class="desc-scene">
      <h3>
        <a data-track-action="NAVIGATION" data-track="TITLE_LINK" href="https://www.private.com/scene/two-girls-one-cock/27139">Two Girls One Cock</a>
      </h3>
      <ul class="scene-models">
        <li><a data-track="PORNSTAR_LINK" href="/pornstar/1812-sara/">Sara Smith</a></li>
        <li><a data-track="PORNSTAR_LINK" href="/pornstar/1813-mai/">Mai Jones</a></li>
      </ul>
      <span class="scene-date">03/29/2026</span>
    </div>
  </div>
</li>

</ul>

<div class="pager-wrapper">
  <ul class="pagination">
    <li><span class="current">1</span></li>
    <li><a data-track="PAGINATION" href="https://www.private.com/scenes/2">2</a></li>
    <li><a data-track="PAGINATION" href="https://www.private.com/scenes/3">3</a></li>
  </ul>
  <div class="pag_groups">
    <ul id="centenes">
      <li><a data-track="PAGINATION" href="https://www.private.com/scenes/10">10</a></li>
      <li><a data-track="PAGINATION" href="https://www.private.com/scenes/50">50</a></li>
    </ul>
  </div>
</div>
</body></html>`

// Page-2 fixture — one final scene; the next page returns empty.
const listing2HTML = `<html><body>
<li class="card">
  <div class="scene">
    <a data-track="SCENE_LINK" href="https://www.private.com/scene/last-one/27100">
      <img src="https://pcom77.st-content.com/content/contentthumbs/506200.jpg">
    </a>
    <div class="desc-scene">
      <h3><a data-track="TITLE_LINK" href="https://www.private.com/scene/last-one/27100">Last One</a></h3>
      <ul class="scene-models">
        <li><a data-track="PORNSTAR_LINK" href="/pornstar/1900-x/">Nameless</a></li>
      </ul>
      <span class="scene-date">02/01/2026</span>
    </div>
  </div>
</li>
</body></html>`

const emptyHTML = `<html><body><div class="container">no scenes</div></body></html>`

func TestParseListing(t *testing.T) {
	items := parseListing([]byte(listingHTML))
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}

	first := items[0]
	if first.id != "27142" {
		t.Errorf("ID = %q, want 27142", first.id)
	}
	if first.url != "https://www.private.com/scene/kore-fox-starts-a-threesome/27142" {
		t.Errorf("URL = %q (expected query-string stripped)", first.url)
	}
	if first.title != "Kore Fox Starts A Threesome & More" {
		t.Errorf("Title = %q (entity unescape failed?)", first.title)
	}
	if len(first.performers) != 1 || first.performers[0] != "Kore Fox" {
		t.Errorf("Performers = %v", first.performers)
	}
	if first.thumb != "https://pcom77.st-content.com/content/contentthumbs/506037.jpg" {
		t.Errorf("Thumb = %q", first.thumb)
	}
	if first.preview == "" {
		t.Errorf("Preview empty, want trailer URL")
	}
	if first.date.Year() != 2026 || first.date.Month() != 4 || first.date.Day() != 4 {
		t.Errorf("Date = %v, want 2026-04-04", first.date)
	}

	second := items[1]
	if second.id != "27139" {
		t.Errorf("Second ID = %q", second.id)
	}
	if len(second.performers) != 2 {
		t.Errorf("Second performers = %v", second.performers)
	}
}

func TestParseListing_dedupes(t *testing.T) {
	doubled := listingHTML + listingHTML
	items := parseListing([]byte(doubled))
	if len(items) != 2 {
		t.Errorf("got %d items after dedup, want 2", len(items))
	}
}

func TestEstimateTotal(t *testing.T) {
	// pagination block lists pages 1, 2, 3, 10, 50 → max 50 × 2 = 100.
	got := estimateTotal([]byte(listingHTML), 2)
	if got != 100 {
		t.Errorf("estimateTotal = %d, want 100", got)
	}
	if got := estimateTotal([]byte(emptyHTML), 1); got != 1 {
		t.Errorf("estimateTotal(empty) = %d, want 1", got)
	}
}

func TestResolveInput(t *testing.T) {
	cases := []struct {
		in      string
		want    resolved
		wantErr bool
	}{
		{"https://www.private.com/", resolved{canonicalBase, "/scenes"}, false},
		{"https://private.com", resolved{canonicalBase, "/scenes"}, false},
		{"https://www.private.com/scenes", resolved{canonicalBase, "/scenes"}, false},
		{"https://www.private.com/scenes/", resolved{canonicalBase, "/scenes"}, false},
		// Page-N input — page comes from the loop, not the URL.
		{"https://www.private.com/scenes/5", resolved{canonicalBase, "/scenes"}, false},
		{"https://www.private.com/movies", resolved{canonicalBase, "/movies"}, false},
		{"https://www.private.com/site/private-stars/", resolved{canonicalBase, "/site/private-stars/"}, false},
		{"https://www.private.com/site/private-stars/3", resolved{canonicalBase, "/site/private-stars/"}, false},
		{"https://www.private.com/pornstar/8391-kore-fox/", resolved{canonicalBase, "/pornstar/8391-kore-fox/"}, false},
		// Sister-domain rewrite.
		{"https://analintroductions.com/", resolved{canonicalBase, "/site/anal-introductions/"}, false},
		{"https://www.privatemilfs.com/", resolved{canonicalBase, "/site/private-milfs/"}, false},
		// Detail page is rejected.
		{"https://www.private.com/scene/foo/12345", resolved{}, true},
		// Wrong host.
		{"https://example.com/scenes", resolved{}, true},
	}
	for _, c := range cases {
		got, err := resolveInput(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("resolveInput(%q) → no error, want one", c.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("resolveInput(%q) → unexpected error: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("resolveInput(%q) = %+v, want %+v", c.in, got, c.want)
		}
	}
}

func TestListingURL(t *testing.T) {
	cases := []struct {
		path string
		page int
		want string
	}{
		// /scenes uses "/scenes/2" (no trailing slash on the path).
		{"/scenes", 1, "https://www.private.com/scenes"},
		{"/scenes", 2, "https://www.private.com/scenes/2"},
		// /site/X/ uses "/site/X/2/" — keeps the trailing slash.
		{"/site/private-stars/", 1, "https://www.private.com/site/private-stars/"},
		{"/site/private-stars/", 4, "https://www.private.com/site/private-stars/4/"},
	}
	for _, c := range cases {
		got := listingURL(resolved{base: canonicalBase, basePath: c.path}, c.page)
		if got != c.want {
			t.Errorf("listingURL(%q, %d) = %q, want %q", c.path, c.page, got, c.want)
		}
	}
}

func TestSeriesFromPath(t *testing.T) {
	cases := []struct {
		path, want string
	}{
		{"/site/private-stars/", "Private Stars"},
		{"/site/anal-introductions/", "Anal Introductions"},
		{"/site/i-confess-files/", "I Confess Files"},
		{"/scenes", ""},
		{"/movies", ""},
		{"/pornstar/8391-kore-fox/", ""},
	}
	for _, c := range cases {
		if got := seriesFromPath(c.path); got != c.want {
			t.Errorf("seriesFromPath(%q) = %q, want %q", c.path, got, c.want)
		}
	}
}

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url string
		ok  bool
	}{
		{"https://www.private.com/", true},
		{"https://private.com/scenes/2", true},
		{"https://www.private.com/site/private-stars/", true},
		{"https://analintroductions.com/", true},
		{"https://www.privatemilfs.com/", true},
		{"https://russianfakeagent.com/", true},
		// Out-of-network.
		{"https://example.com/", false},
		// privateblack.com NOT in matchRe (different CMS, intentional).
		{"https://privateblack.com/", false},
		// Substring trap.
		{"https://com-private.com/", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.ok {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.ok)
		}
	}
}

func TestListScenes_endToEnd(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		switch r.URL.Path {
		case "/scenes":
			_, _ = fmt.Fprint(w, listingHTML)
		case "/scenes/2":
			_, _ = fmt.Fprint(w, listing2HTML)
		default:
			_, _ = fmt.Fprint(w, emptyHTML)
		}
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client()}
	// We have to inject the httptest URL into resolveInput's expected base
	// somehow. Easiest: subclass-style — drop in a tiny override by
	// constructing the listingURL helper manually for this test.
	// Instead, use a custom run loop in this test mirroring the real one,
	// pointing at ts.URL. (parseListing + listingURL are pure functions —
	// covered by the smaller targeted tests above.)
	target := resolved{base: ts.URL, basePath: "/scenes"}
	out := make(chan scraper.SceneResult)
	go s.runFromResolved(context.Background(), target, "test", scraper.ListOpts{}, out)

	var scenes, total int
	for r := range out {
		switch r.Kind {
		case scraper.KindScene:
			scenes++
			if r.Scene.Studio != "Private" {
				t.Errorf("Studio = %q", r.Scene.Studio)
			}
		case scraper.KindTotal:
			total = r.Total
		case scraper.KindError:
			t.Errorf("unexpected error: %v", r.Err)
		}
	}
	if scenes != 3 {
		t.Errorf("got %d scenes, want 3 (2 + 1 across two pages)", scenes)
	}
	if total != 100 {
		t.Errorf("total = %d, want 100 (50 × 2 cards)", total)
	}
}

func TestListScenes_knownIDsStopsEarly(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		if r.URL.Path == "/scenes" {
			_, _ = fmt.Fprint(w, listingHTML)
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client()}
	target := resolved{base: ts.URL, basePath: "/scenes"}
	out := make(chan scraper.SceneResult)
	go s.runFromResolved(context.Background(), target, "test", scraper.ListOpts{
		KnownIDs: map[string]bool{"27139": true},
	}, out)

	var scenes int
	var stoppedEarly bool
	for r := range out {
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
		t.Errorf("got %d scenes, want 1 (stop before known)", scenes)
	}
	if !stoppedEarly {
		t.Error("expected StoppedEarly signal")
	}
}

func TestListScenes_rejectsDetailURL(t *testing.T) {
	s := New()
	ch, err := s.ListScenes(context.Background(),
		"https://www.private.com/scene/foo-bar/12345", scraper.ListOpts{})
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

// TestPatternsMatchRegistry — keep Patterns() display list and the matchRe
// host list in sync.
func TestPatternsMatchRegistry(t *testing.T) {
	s := New()
	for _, p := range s.Patterns() {
		dom := strings.SplitN(p, "/", 2)[0]
		u := "https://" + dom + "/"
		if !s.MatchesURL(u) {
			t.Errorf("Patterns() lists %q but MatchesURL(%q) = false", p, u)
		}
	}
}
