package strokiesutil

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/Wasylq/FSS/scraper"
)

// listingHTML serves three v-thumb cards. The second page repeats them to
// exercise the dedup stop. The third card uses the newer "/tour/" asset path
// (no "_pics" suffix) that strokies.com switched to.
const listingHTML = `<html><body>
  <a class="v-thumb" style="position: relative;" href="/video/first-scene/">
    <img src="//cdn2.strokies.com/assets/tour_pics/3769-eva_nyx/1.jpg" alt="First Scene">
  </a>
  <div class="video-card">
    <a href="/video/second-scene/"><img src="//cdn.strokies.com/assets/tour_pics/3770-jane_doe/1.jpg" alt="Jane Doe"></a>
  </div>
  <a class="v-thumb" style="position: relative;" href="/video/third-scene/">
    <img src="//cdn.strokies.com/tour/3771-summer_kline/1.jpg" alt="Third Scene">
  </a>
</body></html>`

const firstDetailHTML = `<html><body>
<h1 class="video-title">First Scene With Eva Nyx &amp; Friends</h1>
<div class="video-description" style="color: white;"><p>A <strong>wild</strong> handjob scene featuring Eva Nyx.</p></div>
<div class="model-tags">
  <span>More Info On </span>
  <a href="/model/eva-nyx/">Eva Nyx</a>
</div>
<div class="model-tags">
  <span>Tags: <a href="/search/handjob/tag">handjob</a>, <a href="/search/blonde/tag">blonde</a>, <a href="/search/handjob/tag">handjob</a></span>
</div>
</body></html>`

const secondDetailHTML = `<html><body>
<h1 class="video-title">Second Scene</h1>
<div class="video-description"><p>Another scene.</p></div>
<div class="model-tags"><a href="/model/jane-doe/">Jane Doe</a></div>
<div class="model-tags"><span>Tags: <a href="/search/pov/tag">pov</a></span></div>
</body></html>`

const thirdDetailHTML = `<html><body>
<h1 class="video-title">Third Scene</h1>
<div class="video-description"><p>Yet another scene.</p></div>
<div class="model-tags"><a href="/model/summer-kline/">Summer Kline</a></div>
<div class="model-tags"><span>Tags: <a href="/search/pov/tag">pov</a></span></div>
</body></html>`

func testConfig(base string) SiteConfig {
	return SiteConfig{
		ID:       "strokies",
		Studio:   "Strokies",
		SiteBase: base,
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?strokies\.com`),
	}
}

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		switch r.URL.Path {
		case "/", "/page2/":
			_, _ = fmt.Fprint(w, listingHTML)
		case "/video/first-scene/":
			_, _ = fmt.Fprint(w, firstDetailHTML)
		case "/video/second-scene/":
			_, _ = fmt.Fprint(w, secondDetailHTML)
		case "/video/third-scene/":
			_, _ = fmt.Fprint(w, thirdDetailHTML)
		default:
			http.NotFound(w, r)
		}
	}))
}

func TestListScenes_endToEnd(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	s := New(testConfig(ts.URL))
	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}

	byID := map[string]scraper.SceneResult{}
	for r := range ch {
		switch r.Kind {
		case scraper.KindScene:
			byID[r.Scene.ID] = r
		case scraper.KindError:
			t.Errorf("unexpected error: %v", r.Err)
		}
	}
	if len(byID) != 3 {
		t.Fatalf("got %d scenes, want 3 (dup page should stop)", len(byID))
	}

	first, ok := byID["3769"]
	if !ok {
		t.Fatal("missing scene id 3769")
	}
	sc := first.Scene
	if sc.Title != "First Scene With Eva Nyx & Friends" {
		t.Errorf("Title = %q", sc.Title)
	}
	if sc.URL != ts.URL+"/video/first-scene/" {
		t.Errorf("URL = %q", sc.URL)
	}
	if sc.Thumbnail != "https://cdn2.strokies.com/assets/tour_pics/3769-eva_nyx/1.jpg" {
		t.Errorf("Thumbnail = %q", sc.Thumbnail)
	}
	if sc.Description != "A wild handjob scene featuring Eva Nyx." {
		t.Errorf("Description = %q", sc.Description)
	}
	if len(sc.Performers) != 1 || sc.Performers[0] != "Eva Nyx" {
		t.Errorf("Performers = %v, want [Eva Nyx]", sc.Performers)
	}
	// Duplicate "handjob" tag must be de-duplicated.
	wantTags := []string{"handjob", "blonde"}
	if len(sc.Tags) != len(wantTags) {
		t.Fatalf("Tags = %v, want %v", sc.Tags, wantTags)
	}
	for i, tg := range wantTags {
		if sc.Tags[i] != tg {
			t.Errorf("Tags[%d] = %q, want %q", i, sc.Tags[i], tg)
		}
	}
	if sc.Studio != "Strokies" {
		t.Errorf("Studio = %q", sc.Studio)
	}
	if !sc.Date.IsZero() {
		t.Errorf("Date should be zero, got %v", sc.Date)
	}

	second := byID["3770"].Scene
	if second.Title != "Second Scene" {
		t.Errorf("second Title = %q", second.Title)
	}
	if second.Thumbnail != "https://cdn.strokies.com/assets/tour_pics/3770-jane_doe/1.jpg" {
		t.Errorf("second Thumbnail = %q", second.Thumbnail)
	}

	third, ok := byID["3771"]
	if !ok {
		t.Fatal("missing scene id 3771 (newer /tour/ asset path without _pics suffix)")
	}
	if third.Scene.Thumbnail != "https://cdn.strokies.com/tour/3771-summer_kline/1.jpg" {
		t.Errorf("third Thumbnail = %q", third.Scene.Thumbnail)
	}
}

func TestMatchesURL(t *testing.T) {
	s := New(testConfig("https://strokies.com"))
	cases := []struct {
		url   string
		match bool
	}{
		{"https://strokies.com/", true},
		{"http://www.strokies.com/page2/", true},
		{"https://example.com/", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.match {
			t.Errorf("MatchesURL(%q) = %v", c.url, got)
		}
	}
}

func TestAbsURL(t *testing.T) {
	s := New(testConfig("https://strokies.com"))
	cases := []struct{ in, want string }{
		{"//cdn.strokies.com/x.jpg", "https://cdn.strokies.com/x.jpg"},
		{"/video/abc/", "https://strokies.com/video/abc/"},
		{"https://strokies.com/video/abc/", "https://strokies.com/video/abc/"},
	}
	for _, c := range cases {
		if got := s.absURL(c.in); got != c.want {
			t.Errorf("absURL(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestCleanText(t *testing.T) {
	cases := []struct{ in, want string }{
		{"<b>Hi</b> &amp; <i>there</i>  friend", "Hi & there friend"},
		{"  spaced   out  ", "spaced out"},
	}
	for _, c := range cases {
		if got := cleanText(c.in); got != c.want {
			t.Errorf("cleanText(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
