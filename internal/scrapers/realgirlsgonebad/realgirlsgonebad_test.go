package realgirlsgonebad

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wasylq/FSS/scraper"
)

func listingHTML() string {
	// Trailer links appear twice per card (thumb + title). Root-relative so the
	// end-to-end test can resolve them against the httptest base.
	return `<html><body>
<div class="item">
  <a href="/tour/trailers/party-frolics-105.html"><img></a>
  <a href="/tour/trailers/party-frolics-105.html">Party Frolics 105</a>
</div>
<div class="item">
  <a href="/tour/trailers/public-nudity-2-part-4.html"><img></a>
  <a href="/tour/trailers/public-nudity-2-part-4.html">Public Nudity</a>
</div>
</body></html>`
}

func detailHTML(id, title, added string) string {
	return fmt.Sprintf(`<html><head>
<meta property="og:title" content="%s | Real Girls Gone Bad">
<meta property="og:image" content="//www.realgirlsgonebad.com/tour/content/contentthumbs/%s.jpg">
</head><body>
<div class="views">129,472 views</div>
<p>Here&#8217;s another filthy party full of naughty games and hot girls.</p>
<div class="eDtls">
  <span><strong>Runtime:</strong> 06:47</span>
  <span><strong>Added:</strong> %s</span>
</div>
</body></html>`, title, id, added)
}

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url   string
		match bool
	}{
		{"https://www.realgirlsgonebad.com/tour/categories/videos_1_d.html", true},
		{"https://realgirlsgonebad.com/tour/trailers/foo.html", true},
		{"https://example.com/x", false},
		{"", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.match {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.match)
		}
	}
}

func TestParseListing(t *testing.T) {
	orig := baseURL
	defer func() { baseURL = orig }()
	baseURL = "https://www.realgirlsgonebad.com"

	urls := parseListing([]byte(listingHTML()), map[string]bool{})
	if len(urls) != 2 {
		t.Fatalf("got %d urls, want 2 (dups dropped): %v", len(urls), urls)
	}
	if urls[0] != "https://www.realgirlsgonebad.com/tour/trailers/party-frolics-105.html" {
		t.Errorf("url0 = %q", urls[0])
	}
}

func TestNormalizeURL(t *testing.T) {
	orig := baseURL
	defer func() { baseURL = orig }()
	baseURL = "https://www.realgirlsgonebad.com"

	cases := map[string]string{
		"//www.realgirlsgonebad.com/tour/trailers/x.html": "https://www.realgirlsgonebad.com/tour/trailers/x.html",
		"/tour/trailers/y.html":                           "https://www.realgirlsgonebad.com/tour/trailers/y.html",
		"https://other.com/z.html":                        "https://other.com/z.html",
	}
	for in, want := range cases {
		if got := normalizeURL(in); got != want {
			t.Errorf("normalizeURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseDetail(t *testing.T) {
	orig := baseURL
	defer func() { baseURL = orig }()
	baseURL = "https://www.realgirlsgonebad.com"

	sc := parseDetail([]byte(detailHTML("8492", "Party Frolics 105", "4 April, 2026")),
		"studioURL", "https://www.realgirlsgonebad.com/tour/trailers/party-frolics-105.html", time.Now().UTC())

	if sc.ID != "8492" {
		t.Errorf("ID = %q, want 8492", sc.ID)
	}
	if sc.Title != "Party Frolics 105" {
		t.Errorf("Title = %q", sc.Title)
	}
	if sc.SiteID != siteID || sc.Studio != studio {
		t.Errorf("SiteID/Studio = %q/%q", sc.SiteID, sc.Studio)
	}
	if sc.Duration != 6*60+47 {
		t.Errorf("Duration = %d, want 407", sc.Duration)
	}
	if y, m, d := sc.Date.Date(); y != 2026 || m != 4 || d != 4 {
		t.Errorf("Date = %v, want 2026-04-04", sc.Date)
	}
	if sc.Thumbnail != "https://www.realgirlsgonebad.com/tour/content/contentthumbs/8492.jpg" {
		t.Errorf("Thumbnail = %q", sc.Thumbnail)
	}
	if !strings.HasPrefix(sc.Description, "Here") || !strings.Contains(sc.Description, "hot girls") {
		t.Errorf("Description = %q", sc.Description)
	}
}

func TestListScenes(t *testing.T) {
	orig := baseURL
	defer func() { baseURL = orig }()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "videos_1_d.html"):
			_, _ = fmt.Fprint(w, listingHTML())
		case strings.Contains(r.URL.Path, "party-frolics-105"):
			_, _ = fmt.Fprint(w, detailHTML("8492", "Party Frolics 105", "4 April, 2026"))
		case strings.Contains(r.URL.Path, "public-nudity-2-part-4"):
			_, _ = fmt.Fprint(w, detailHTML("8500", "Public Nudity 2 Part 4", "1 May, 2026"))
		default:
			_, _ = fmt.Fprint(w, "<html></html>")
		}
	}))
	defer ts.Close()
	baseURL = ts.URL

	s := &Scraper{Client: ts.Client()}
	ch, err := s.ListScenes(context.Background(), "studioURL", scraper.ListOpts{})
	if err != nil {
		t.Fatalf("ListScenes error: %v", err)
	}
	got := map[string]string{}
	for r := range ch {
		if r.Err != nil {
			t.Errorf("unexpected error: %v", r.Err)
			continue
		}
		if r.Kind == scraper.KindScene {
			got[r.Scene.ID] = r.Scene.Title
		}
	}
	if len(got) != 2 {
		t.Fatalf("got %d scenes, want 2: %v", len(got), got)
	}
	if got["8492"] != "Party Frolics 105" || got["8500"] != "Public Nudity 2 Part 4" {
		t.Errorf("scenes = %v", got)
	}
}
