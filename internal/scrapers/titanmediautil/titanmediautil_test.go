package titanmediautil

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/Wasylq/FSS/scraper"
)

// listingHTML is built at runtime so trailer hrefs point at the test server.
func listingHTML(base string) string {
	return fmt.Sprintf(`<html><body>
<div class="item-video" data-videoid="bVid1551" data-videoposter="/tour/content/thumbs/12859-1x.jpg">
  <a href="%s/trailers/Brand-Jun-19-2026.html" title="Jun. 19, 2026" class="item-thumb-link"></a>
</div>
<div class="item-video" data-videoid="bVid1552" data-videoposter="/tour/content/thumbs/12860-1x.jpg">
  <a href="%s/trailers/Brand-Jun-12-2026.html" title="Jun. 12, 2026" class="item-thumb-link"></a>
</div>
</body></html>`, base, base)
}

const detailHTML = `<html><body>
<div class="video-meta">Runtime: <span> 27:43 </span></div>
<div class="content"><p>Hot scene with @JaneDoe and @JohnRoe enjoying themselves.</p></div>
<ul class="tags">
  <li><a href="categories/Oral_1_d.html">Oral</a></li>
  <li><a href="categories/Bareback_1_d.html">Bareback</a></li>
</ul>
</body></html>`

func testConfig(base string) SiteConfig {
	return SiteConfig{
		ID:       "gloryholeswallow",
		Studio:   "Gloryhole Swallow",
		SiteBase: base,
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?gloryholeswallow\.com`),
	}
}

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		switch r.URL.Path {
		case "/categories/Movies_1_d.html", "/categories/Movies_2_d.html":
			// Page 2 repeats page 1's cards to exercise the dup-page stop.
			_, _ = fmt.Fprint(w, listingHTML(srv.URL))
		case "/trailers/Brand-Jun-19-2026.html", "/trailers/Brand-Jun-12-2026.html":
			_, _ = fmt.Fprint(w, detailHTML)
		default:
			http.NotFound(w, r)
		}
	}))
	return srv
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
	if len(byID) != 2 {
		t.Fatalf("got %d scenes, want 2 (dup page should stop)", len(byID))
	}

	first, ok := byID["1551"]
	if !ok {
		t.Fatal("missing scene id 1551")
	}
	sc := first.Scene
	if sc.Title != "Jun. 19, 2026" {
		t.Errorf("Title = %q, want the date", sc.Title)
	}
	if sc.Date.Year() != 2026 || sc.Date.Month() != 6 || sc.Date.Day() != 19 {
		t.Errorf("Date = %v, want 2026-06-19", sc.Date)
	}
	if sc.Duration != 1663 {
		t.Errorf("Duration = %d, want 1663", sc.Duration)
	}
	if sc.Description != "Hot scene with @JaneDoe and @JohnRoe enjoying themselves." {
		t.Errorf("Description = %q", sc.Description)
	}
	wantPerf := []string{"JaneDoe", "JohnRoe"}
	if len(sc.Performers) != len(wantPerf) {
		t.Fatalf("Performers = %v, want %v", sc.Performers, wantPerf)
	}
	for i, p := range wantPerf {
		if sc.Performers[i] != p {
			t.Errorf("Performers[%d] = %q, want %q", i, sc.Performers[i], p)
		}
	}
	wantTags := map[string]bool{"Oral": true, "Bareback": true}
	if len(sc.Tags) != 2 {
		t.Errorf("Tags = %v, want Oral+Bareback", sc.Tags)
	}
	for _, tg := range sc.Tags {
		if !wantTags[tg] {
			t.Errorf("unexpected tag %q", tg)
		}
	}
	if sc.Thumbnail != ts.URL+"/tour/content/thumbs/12859-1x.jpg" {
		t.Errorf("Thumbnail = %q", sc.Thumbnail)
	}
	if sc.Studio != "Gloryhole Swallow" {
		t.Errorf("Studio = %q", sc.Studio)
	}
}

func TestMatchesURL(t *testing.T) {
	s := New(testConfig("https://gloryholeswallow.com"))
	cases := []struct {
		url   string
		match bool
	}{
		{"https://gloryholeswallow.com/", true},
		{"http://www.gloryholeswallow.com/categories/Movies_1_d.html", true},
		{"https://example.com/", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.match {
			t.Errorf("MatchesURL(%q) = %v", c.url, got)
		}
	}
}

func TestCleanText(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"<b>Hi</b> &amp; <i>there</i>  friend", "Hi & there friend"},
		{"  spaced   out  ", "spaced out"},
		{"<p>plain</p>", "plain"},
	}
	for _, c := range cases {
		if got := cleanText(c.in); got != c.want {
			t.Errorf("cleanText(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSiteRoot(t *testing.T) {
	cases := []struct {
		base, want string
	}{
		{"https://gloryholeswallow.com/tour", "https://gloryholeswallow.com"},
		{"https://cumpsters.com", "https://cumpsters.com"},
	}
	for _, c := range cases {
		s := New(SiteConfig{SiteBase: c.base, MatchRe: regexp.MustCompile(`.*`)})
		if got := s.siteRoot(); got != c.want {
			t.Errorf("siteRoot(%q) = %q, want %q", c.base, got, c.want)
		}
	}
}
