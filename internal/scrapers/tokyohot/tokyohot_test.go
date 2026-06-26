package tokyohot

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wasylq/FSS/scraper"
)

// listingHTML is a /product/ listing page with two product links (plus a
// duplicate to exercise dedupe).
const listingHTML = `<html><body>
<div class="contents">
  <a href="/product/n1234/"><img src="x"></a>
  <a href="/product/k0567/"><img src="x"></a>
  <a href="/product/n1234/">dup link</a>
</div>
</body></html>`

// detailHTML is a product detail page with a full <dl class="info"> table.
const detailHTML = `<html><head>
<title>Tokyo Hot n1234 Some Scene Title | Tokyo-Hot Official</title>
<meta property="og:title" content="Tokyo Hot n1234 Some Scene Title">
</head><body>
<video poster="https://my.cdn.tokyo-hot.com/media/n1234/poster.jpg"></video>
<dl class="info">
  <dt>Model</dt><dd><a href="/cast/x/">Aoi Mizuki</a><a href="/cast/y/">Unknown</a></dd>
  <dt>Play</dt><dd><a href="/play/a/">Creampie</a><a href="/play/b/">Toy</a></dd>
  <dt>Theme</dt><dd><a href="/theme/c/">Amateur</a></dd>
  <dt>Label</dt><dd><a href="/label/d/">Tokyo Hot</a></dd>
  <dt>Release Date</dt><dd>2006/01/02</dd>
  <dt>Duration</dt><dd>01:23:45</dd>
  <dt>Resolution</dt><dd>1920x1080</dd>
</dl>
</body></html>`

func TestFetchListing_extractsCodes(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, listingHTML)
	}))
	defer ts.Close()

	s := New()
	codes, err := s.fetchListing(context.Background(), ts.URL+"/product/?page=1")
	if err != nil {
		t.Fatalf("fetchListing: %v", err)
	}
	// productLinkRe matches all anchors including the duplicate.
	if len(codes) != 3 {
		t.Fatalf("got %d codes, want 3: %v", len(codes), codes)
	}
	if codes[0] != "n1234" || codes[1] != "k0567" {
		t.Errorf("codes = %v", codes)
	}
}

func TestToScene_parsesDetail(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, detailHTML)
	}))
	defer ts.Close()

	orig := siteBase
	defer func() { siteBase = orig }()
	siteBase = ts.URL

	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	scene := New().toScene(context.Background(), "https://www.tokyo-hot.com", "n1234", now)

	if scene.ID != "n1234" {
		t.Errorf("ID = %q", scene.ID)
	}
	if scene.SiteID != "tokyohot" {
		t.Errorf("SiteID = %q", scene.SiteID)
	}
	if scene.Title != "Tokyo Hot n1234 Some Scene Title" {
		t.Errorf("Title = %q", scene.Title)
	}
	if scene.Thumbnail != "https://my.cdn.tokyo-hot.com/media/n1234/poster.jpg" {
		t.Errorf("Thumbnail = %q", scene.Thumbnail)
	}
	// "Unknown" model must be skipped.
	if len(scene.Performers) != 1 || scene.Performers[0] != "Aoi Mizuki" {
		t.Errorf("Performers = %v, want [Aoi Mizuki]", scene.Performers)
	}
	wantTags := map[string]bool{"Creampie": true, "Toy": true, "Amateur": true}
	if len(scene.Tags) != 3 {
		t.Errorf("Tags = %v, want 3", scene.Tags)
	}
	for _, tg := range scene.Tags {
		if !wantTags[tg] {
			t.Errorf("unexpected tag %q", tg)
		}
	}
	if scene.Series != "Tokyo Hot" {
		t.Errorf("Series (from Label) = %q", scene.Series)
	}
	if scene.Resolution != "1920x1080" {
		t.Errorf("Resolution = %q", scene.Resolution)
	}
	wantDate := time.Date(2006, 1, 2, 0, 0, 0, 0, time.UTC)
	if !scene.Date.Equal(wantDate) {
		t.Errorf("Date = %v, want %v", scene.Date, wantDate)
	}
	if scene.Duration != 5025 { // 01:23:45
		t.Errorf("Duration = %d, want 5025", scene.Duration)
	}
}

func TestParseTitle_fallbackToOG(t *testing.T) {
	detail := `<title> | Tokyo-Hot</title>` +
		`<meta property="og:title" content="OG Fallback Title">`
	if got := parseTitle(detail); got != "OG Fallback Title" {
		t.Errorf("parseTitle = %q, want OG Fallback Title", got)
	}
}

func TestParseThumbnail_listImageFallback(t *testing.T) {
	detail := `<img src="https://my.cdn.tokyo-hot.com/media/k0567/list_image/foo820x462bar.jpg">` +
		`<img src="https://my.cdn.tokyo-hot.com/media/other/list_image/x820x462y.jpg">`
	got := parseThumbnail(detail, "k0567")
	want := "https://my.cdn.tokyo-hot.com/media/k0567/list_image/foo820x462bar.jpg"
	if got != want {
		t.Errorf("parseThumbnail = %q, want %q", got, want)
	}
}

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url   string
		match bool
	}{
		{"https://www.tokyo-hot.com/product/n1234/", true},
		{"http://tokyo-hot.com/product/", true},
		{"https://example.com/", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.match {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.match)
		}
	}
}

func TestListScenes_endToEnd(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/product/":
			if r.URL.Query().Get("page") == "1" {
				_, _ = fmt.Fprint(w, listingHTML)
				return
			}
			_, _ = fmt.Fprint(w, `<html><body>no products</body></html>`)
		case "/product/n1234/", "/product/k0567/":
			_, _ = fmt.Fprint(w, detailHTML)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	orig := siteBase
	defer func() { siteBase = orig }()
	siteBase = ts.URL

	s := New()
	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{})
	if err != nil {
		t.Fatalf("ListScenes: %v", err)
	}

	var count int
	for r := range ch {
		switch r.Kind {
		case scraper.KindScene:
			count++
		case scraper.KindError:
			t.Errorf("unexpected error: %v", r.Err)
		}
	}
	// Two unique codes (n1234 deduped).
	if count != 2 {
		t.Errorf("got %d scenes, want 2", count)
	}
}
