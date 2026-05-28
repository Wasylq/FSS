package porngutter

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wasylq/FSS/scraper"
)

// Fixture derived from real porngutter.com /updates/ markup — two cards
// with all the fields the parser cares about, plus a `class="pagination"`
// block pointing at page 4 as the last page.
const listingHTML = `<html><body>
<div class="container">

<div class="video-item grid-item">
  <div class="item-wrapper">
    <a href="/update/2780/?nats=ABC&amp;step=2" class="item-thumb">
      <img id="thumb-2780" src="https://fast-media.example/cover-2780.jpg" class="img" alt="" loading="auto">
      <video id="thumbvideo-2780" class="preview-video" playsinline controls loop muted="" preload="none">
        <source src="https://fast-media.example/preview-2780.mp4" type="video/mp4">
      </video>
    </a>
    <div class="item-content">
      <div class="item-cblock">
        <p><a href="/models/2072/" class="item-talent female">Elektra Rose</a></p>
        <a href="/update/2780/?nats=ABC&amp;step=2">Black &amp; Big Destroys Young Slut Elektra Rose</a>
      </div>
      <div class="item-cblock">
        <p>
          May 28, 2026 <br>
          <a href="/bonus_site/smut-merchants/?nats=ABC" class="quick-tag">Smut Merchants</a>
        </p>
      </div>
    </div>
  </div>
</div>

<div class="video-item grid-item">
  <div class="item-wrapper">
    <a href="/update/2779/?nats=ABC&amp;step=2" class="item-thumb">
      <img id="thumb-2779" src="https://fast-media.example/cover-2779.jpg" class="img" alt="">
    </a>
    <div class="item-content">
      <div class="item-cblock">
        <p>
          <a href="/models/1812/" class="item-talent female">Sara Smith</a>
          <a href="/models/1900/" class="item-talent female">Mai Jones</a>
        </p>
        <a href="/update/2779/?nats=ABC&amp;step=2">Two Girls One Cock</a>
      </div>
      <div class="item-cblock">
        <p>
          January 12, 2026 <br>
          <a href="/bonus_site/3-way-fuck/?nats=ABC" class="quick-tag">3 Way Fuck</a>
        </p>
      </div>
    </div>
  </div>
</div>

<ul class="pagination">
  <li class="page-item active"><a class="page-link" href="?page=1&">1</a></li>
  <li class="page-item"><a class="page-link" href="?page=2&">2</a></li>
  <li class="page-item"><a class="page-link" href="?page=3&">3</a></li>
  <li class="page-item"><a class="page-link" href="?page=4&">4</a></li>
</ul>
</div>
</body></html>`

// Single-card page used to test pagination termination via empty next page.
const listing2HTML = `<html><body>
<div class="video-item grid-item">
  <div class="item-wrapper">
    <a href="/update/2700/?nats=ABC" class="item-thumb">
      <img id="thumb-2700" src="https://fast-media.example/cover-2700.jpg">
    </a>
    <div class="item-content">
      <div class="item-cblock">
        <a href="/update/2700/?nats=ABC">Last One</a>
      </div>
      <div class="item-cblock">
        <p>December 1, 2025</p>
      </div>
    </div>
  </div>
</div>
</body></html>`

const emptyHTML = `<html><body><div class="container">no scenes</div></body></html>`

func TestParseListing(t *testing.T) {
	items := parseListing([]byte(listingHTML))
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}

	first := items[0]
	if first.id != "2780" {
		t.Errorf("ID = %q, want 2780", first.id)
	}
	if first.url != "/update/2780/" {
		t.Errorf("URL = %q, want /update/2780/", first.url)
	}
	// HTML entities should be unescaped: &amp; → &
	if first.title != "Black & Big Destroys Young Slut Elektra Rose" {
		t.Errorf("Title = %q (entity unescape failed?)", first.title)
	}
	if len(first.performers) != 1 || first.performers[0] != "Elektra Rose" {
		t.Errorf("Performers = %v", first.performers)
	}
	if first.series != "Smut Merchants" {
		t.Errorf("Series = %q", first.series)
	}
	if first.thumb != "https://fast-media.example/cover-2780.jpg" {
		t.Errorf("Thumb = %q", first.thumb)
	}
	if first.preview != "https://fast-media.example/preview-2780.mp4" {
		t.Errorf("Preview = %q", first.preview)
	}
	if first.date.Year() != 2026 || first.date.Month() != 5 || first.date.Day() != 28 {
		t.Errorf("Date = %v, want 2026-05-28", first.date)
	}

	second := items[1]
	if second.id != "2779" {
		t.Errorf("Second ID = %q", second.id)
	}
	// Two performers: Sara Smith and Mai Jones.
	if len(second.performers) != 2 {
		t.Errorf("Second performers = %v (want 2)", second.performers)
	}
	if second.series != "3 Way Fuck" {
		t.Errorf("Second series = %q", second.series)
	}
	if second.date.IsZero() {
		t.Errorf("Second date is zero, want Jan 12 2026")
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
	// pagination block has links 1, 2, 3, 4 → max 4 × 2 cards = 8.
	got := estimateTotal([]byte(listingHTML), 2)
	if got != 8 {
		t.Errorf("estimateTotal = %d, want 8", got)
	}
	// No pagination → just perPage.
	if got := estimateTotal([]byte(listing2HTML), 1); got != 1 {
		t.Errorf("estimateTotal(no-pagination) = %d, want 1", got)
	}
}

func TestResolveListingPath(t *testing.T) {
	cases := []struct {
		in, want string
		wantErr  bool
	}{
		{"", "/updates/", false},
		{"/", "/updates/", false},
		{"/updates", "/updates/", false},
		{"/updates/", "/updates/", false},
		{"/bonus_site/smut-merchants/", "/bonus_site/smut-merchants/", false},
		{"/bonus_site/3-way-fuck", "/bonus_site/3-way-fuck/", false}, // trailing slash added
		// Detail URL → reject.
		{"/update/2780/", "", true},
		// Unknown path → reject (user mistake, don't silently scrape root).
		{"/models/2072/", "", true},
	}
	for _, c := range cases {
		got, err := resolveListingPath(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("resolveListingPath(%q) → no error, want one", c.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("resolveListingPath(%q) → unexpected error: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("resolveListingPath(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url string
		ok  bool
	}{
		{"https://porngutter.com/", true},
		{"https://www.porngutter.com/updates/", true},
		{"https://home2.smutpuppet.com/", true},
		{"https://smutpuppet.com/bonus_site/smut-merchants/", true},
		{"https://blackandbig.com/", true},
		{"https://teenerotica.xxx/", true},
		{"http://porn-uk.com/", true},
		// Sister-domain test for every entry in Patterns().
		{"https://3wayfuck.com/", true},
		{"https://hcjav.com/", true},
		{"https://maturefucksteen.com/", true},
		// Out-of-network domain — must NOT match.
		{"https://example.com/", false},
		{"https://creampiethais.com/", false},
		// Substring trap: "gutterporn.com" is not "porngutter.com".
		{"https://gutterporn.com/", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.ok {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.ok)
		}
	}
}

func TestListingURL_preservesHostAndQuery(t *testing.T) {
	s := New()
	got, err := s.listingURL("https://smutpuppet.com/updates/?nats=ABC", "/updates/", 3)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, "https://smutpuppet.com/updates/?") {
		t.Errorf("URL = %q (expected smutpuppet.com host preserved)", got)
	}
	if !strings.Contains(got, "page=3") {
		t.Errorf("URL = %q (missing page=3)", got)
	}
	if !strings.Contains(got, "nats=ABC") {
		t.Errorf("URL = %q (lost nats= affiliate code)", got)
	}
}

func TestListScenes_endToEnd(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		page := r.URL.Query().Get("page")
		switch {
		case r.URL.Path == "/updates/" && (page == "" || page == "1"):
			_, _ = fmt.Fprint(w, listingHTML)
		case r.URL.Path == "/updates/" && page == "2":
			_, _ = fmt.Fprint(w, listing2HTML)
		case r.URL.Path == "/updates/":
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

	var scenes, total int
	for r := range ch {
		switch r.Kind {
		case scraper.KindScene:
			scenes++
			if r.Scene.Studio != "Porn Gutter" {
				t.Errorf("Studio = %q", r.Scene.Studio)
			}
			if !strings.HasPrefix(r.Scene.URL, ts.URL+"/update/") {
				t.Errorf("URL = %q (expected scheme+host/update/...)", r.Scene.URL)
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
	if total != 8 {
		t.Errorf("total = %d, want 8", total)
	}
}

func TestListScenes_knownIDsStopsEarly(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		if r.URL.Path == "/updates/" {
			_, _ = fmt.Fprint(w, listingHTML)
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	s := New()
	ch, err := s.ListScenes(context.Background(), ts.URL+"/", scraper.ListOpts{
		// Second card's ID — first should pass through, then stop.
		KnownIDs: map[string]bool{"2779": true},
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
		t.Errorf("got %d scenes, want 1 (stop before known)", scenes)
	}
	if !stoppedEarly {
		t.Error("expected StoppedEarly signal")
	}
}

// TestListScenes_bonusSitePath confirms that /bonus_site/{slug}/ inputs are
// passed through to the server as-is (not rewritten to /updates/).
func TestListScenes_bonusSitePath(t *testing.T) {
	var fetched string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fetched = r.URL.Path
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, emptyHTML)
	}))
	defer ts.Close()

	s := New()
	ch, err := s.ListScenes(context.Background(), ts.URL+"/bonus_site/smut-merchants/", scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}
	for range ch {
	}
	if fetched != "/bonus_site/smut-merchants/" {
		t.Errorf("server saw %q, want /bonus_site/smut-merchants/", fetched)
	}
}

func TestListScenes_rejectsDetailURL(t *testing.T) {
	s := New()
	ch, err := s.ListScenes(context.Background(),
		"https://porngutter.com/update/2780/?nats=ABC", scraper.ListOpts{})
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

// TestPatternsMatchRegistry — the human-readable Patterns() list must stay
// in sync with the matchRe domain list. If you add a sister site to one,
// add it to the other.
func TestPatternsMatchRegistry(t *testing.T) {
	s := New()
	for _, p := range s.Patterns() {
		// Reconstruct a URL from the pattern's first path segment and test it.
		dom := strings.SplitN(p, "/", 2)[0]
		u := "https://" + dom + "/"
		if !s.MatchesURL(u) {
			t.Errorf("Patterns() lists %q but MatchesURL(%q) = false", p, u)
		}
	}
}
