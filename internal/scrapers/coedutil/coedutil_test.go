package coedutil

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/Wasylq/FSS/scraper"
)

// listingHTML mixes the trailer theme (h5 + availdate) and the gallery theme
// (h4 + bare date span + CDN-absolute thumbnail), plus a boilerplate card with
// no content thumbnail that must be skipped.
func listingHTML(base string) string {
	return fmt.Sprintf(`<html><body>
<div class="updateItem">
  <div class="updateThumb">
    <a  href="%s/trailers/Kitty-Kate-Strapon-Fun.html" onclick="tload('/trailers/x.mp4'); return false;">
      <img alt="062626_kitty_kate" class="stdimage " src="content/062626_kitty_kate/1.jpg" src0_1x="content/062626_kitty_kate/1.jpg" />
    </a>
  </div>
  <div class="updateInfo">
    <h5><a  href="%s/trailers/Kitty-Kate-Strapon-Fun.html">Kitty Kate &amp; Dora Strapon Fun</a></h5>
    <p>
      <span class="tour_update_models">
        <a href="%s/models/honey-dory.html">Abbie Storm aka Dora</a> , <a href="%s/models/KittiKate.html">Kitti Kate</a>
      </span>
      <span class="availdate">06/26/2026</span>
    </p>
  </div>
</div>
<div class="updateItem">
  <a href="content/101525_sexy_striptease/0-large.jpg" class="fancybox">
    <img alt="101525_sexy_striptease" class="stdimage " src0_1x="https://c74178f28e.mjedge.net/content/101525_sexy_striptease/1.jpg" />
  </a>
  <div class="updateDetails">
    <h4><a href="content/101525_sexy_striptease/0-large.jpg" class="fancybox">Sexy Striptease with Paulina</a></h4>
    <p>
      <span class="tour_update_models">
        <a href="%s/models/Paulina.html">Paulina</a>
      </span>
      <span>10/15/2025</span>
    </p>
  </div>
</div>
<div class="updateItem">
  <div class="updateThumb">No content here, just boilerplate.</div>
</div>
</body></html>`, base, base, base, base, base)
}

func testConfig(base string) SiteConfig {
	return SiteConfig{
		ID:       "nebraskacoeds",
		Studio:   "Nebraska Coeds",
		SiteBase: base,
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.|tour\.)?nebraskacoeds\.com`),
	}
}

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		switch r.URL.Path {
		case "/categories/updates_1_d.html", "/categories/updates_2_d.html":
			// Page 2 repeats page 1's cards to exercise the dedup stop.
			_, _ = fmt.Fprint(w, listingHTML(srv.URL))
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
	// Two valid cards; the boilerplate card is skipped and the dup page stops.
	if len(byID) != 2 {
		t.Fatalf("got %d scenes, want 2", len(byID))
	}

	trailer, ok := byID["062626_kitty_kate"]
	if !ok {
		t.Fatal("missing trailer scene id")
	}
	sc := trailer.Scene
	if sc.Title != "Kitty Kate & Dora Strapon Fun" {
		t.Errorf("Title = %q", sc.Title)
	}
	if sc.URL != ts.URL+"/trailers/Kitty-Kate-Strapon-Fun.html" {
		t.Errorf("URL = %q", sc.URL)
	}
	if sc.Thumbnail != ts.URL+"/content/062626_kitty_kate/1.jpg" {
		t.Errorf("Thumbnail = %q", sc.Thumbnail)
	}
	wantPerf := []string{"Abbie Storm aka Dora", "Kitti Kate"}
	if len(sc.Performers) != len(wantPerf) {
		t.Fatalf("Performers = %v, want %v", sc.Performers, wantPerf)
	}
	for i, p := range wantPerf {
		if sc.Performers[i] != p {
			t.Errorf("Performers[%d] = %q, want %q", i, sc.Performers[i], p)
		}
	}
	if sc.Date.Year() != 2026 || sc.Date.Month() != 6 || sc.Date.Day() != 26 {
		t.Errorf("Date = %v, want 2026-06-26", sc.Date)
	}
	if sc.Studio != "Nebraska Coeds" {
		t.Errorf("Studio = %q", sc.Studio)
	}

	// Gallery theme: h4 title, bare date span, CDN-absolute thumbnail, fancybox URL.
	gallery, ok := byID["101525_sexy_striptease"]
	if !ok {
		t.Fatal("missing gallery scene id")
	}
	g := gallery.Scene
	if g.Title != "Sexy Striptease with Paulina" {
		t.Errorf("gallery Title = %q", g.Title)
	}
	if g.Thumbnail != "https://c74178f28e.mjedge.net/content/101525_sexy_striptease/1.jpg" {
		t.Errorf("gallery Thumbnail = %q", g.Thumbnail)
	}
	if g.URL != ts.URL+"/content/101525_sexy_striptease/0-large.jpg" {
		t.Errorf("gallery URL = %q", g.URL)
	}
	if g.Date.Year() != 2025 || g.Date.Month() != 10 || g.Date.Day() != 15 {
		t.Errorf("gallery Date = %v, want 2025-10-15", g.Date)
	}
	if len(g.Performers) != 1 || g.Performers[0] != "Paulina" {
		t.Errorf("gallery Performers = %v, want [Paulina]", g.Performers)
	}
}

func TestMatchesURL(t *testing.T) {
	s := New(testConfig("https://tour.nebraskacoeds.com"))
	cases := []struct {
		url   string
		match bool
	}{
		{"https://tour.nebraskacoeds.com/", true},
		{"https://nebraskacoeds.com/categories/updates_1_d.html", true},
		{"https://example.com/", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.match {
			t.Errorf("MatchesURL(%q) = %v", c.url, got)
		}
	}
}

func TestParseDate(t *testing.T) {
	d := parseDate(`<span class="availdate">06/26/2026</span>`)
	if d.Year() != 2026 || d.Month() != 6 || d.Day() != 26 {
		t.Errorf("availdate parse = %v", d)
	}
	d = parseDate(`<span>10/15/2025</span>`)
	if d.Year() != 2025 || d.Month() != 10 || d.Day() != 15 {
		t.Errorf("bare span parse = %v", d)
	}
	if got := parseDate(`no date here`); !got.IsZero() {
		t.Errorf("missing date = %v, want zero", got)
	}
}

func TestCleanText(t *testing.T) {
	if got := cleanText("<b>Hi</b> &amp; bye"); got != "Hi & bye" {
		t.Errorf("cleanText = %q", got)
	}
}
