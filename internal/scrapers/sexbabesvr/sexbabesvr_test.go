package sexbabesvr

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

const sitemap1 = `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url>
    <loc>https://sexbabesvr.com/video/the-fitting-room-nata-ocean/</loc>
    <lastmod>2026-06-26</lastmod>
  </url>
  <url>
    <loc>https://sexbabesvr.com/video/double-trouble-vicky-love-diane/</loc>
  </url>
  <url>
    <loc>https://sexbabesvr.com/models/someone/</loc>
  </url>
</urlset>`

const detailHTML = `<html><head>
<meta property="og:title" content="The Fitting Room - VR PORN">
<meta property="og:image" content="https://sexbabesvr.com/contents/og.jpg">
<script type="application/ld+json">{
  "@context": "https://schema.org",
  "@type": "VideoObject",
  "name": "The Fitting Room",
  "description": "Nata Ocean was hired to dress you.",
  "uploadDate": "2026-06-24T07:53:39Z",
  "duration": "PT0H46M51S",
  "thumbnailUrl": "https://sexbabesvr.com/contents/videos_screenshots/0/764/preview.jpg",
  "actor": [{"@type": "Person", "name": "Nata Ocean", "url": "https://sexbabesvr.com/model/nata-ocean-vr/"}]
}</script>
</head><body></body></html>`

// fallbackHTML has no JSON-LD VideoObject, exercising the og: fallbacks.
const fallbackHTML = `<html><head>
<meta property="og:title" content="Double Trouble - VR PORN">
<meta property="og:image" content="https://sexbabesvr.com/contents/dt.jpg">
<meta property="og:description" content="Two babes one room.">
</head><body></body></html>`

func sitemapForBase(base string) (string, string) {
	sm1 := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>%s/video/the-fitting-room-nata-ocean/</loc></url>
  <url><loc>%s/video/double-trouble-vicky-love-diane/</loc></url>
  <url><loc>%s/models/someone/</loc></url>
</urlset>`, base, base, base)
	sm2 := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>%s/video/third-video/</loc></url>
</urlset>`, base)
	return sm1, sm2
}

func detailServer(t *testing.T) *httptest.Server {
	t.Helper()
	var base string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sm1, sm2 := sitemapForBase(base)
		switch {
		case strings.Contains(r.URL.RawQuery, "from_links_videos=1"):
			_, _ = fmt.Fprint(w, sm1)
		case strings.Contains(r.URL.RawQuery, "from_links_videos=2"):
			_, _ = fmt.Fprint(w, sm2)
		case r.URL.Path == "/video/the-fitting-room-nata-ocean/":
			_, _ = fmt.Fprint(w, detailHTML)
		default:
			_, _ = fmt.Fprint(w, fallbackHTML)
		}
	}))
	base = ts.URL
	return ts
}

func TestParseSitemap(t *testing.T) {
	items := parseSitemap([]byte(sitemap1))
	if len(items) != 2 { // /models/ skipped
		t.Fatalf("got %d items, want 2", len(items))
	}
	if items[0].id != "the-fitting-room-nata-ocean" {
		t.Errorf("id = %q", items[0].id)
	}
}

func TestFetchScene(t *testing.T) {
	ts := detailServer(t)
	defer ts.Close()

	s := New()
	s.Client = ts.Client()
	now := time.Date(2026, 6, 26, 0, 0, 0, 0, time.UTC)

	it := sitemapItem{id: "the-fitting-room-nata-ocean", url: ts.URL + "/video/the-fitting-room-nata-ocean/"}
	sc, err := s.fetchScene(context.Background(), ts.URL, it, now)
	if err != nil {
		t.Fatalf("fetchScene: %v", err)
	}
	if sc.ID != "the-fitting-room-nata-ocean" {
		t.Errorf("ID = %q", sc.ID)
	}
	if sc.Title != "The Fitting Room" {
		t.Errorf("Title = %q", sc.Title)
	}
	if sc.SiteID != siteID || sc.Studio != studioName {
		t.Errorf("SiteID/Studio = %q/%q", sc.SiteID, sc.Studio)
	}
	if sc.Duration != 2811 { // 46*60+51
		t.Errorf("Duration = %d, want 2811", sc.Duration)
	}
	if len(sc.Performers) != 1 || sc.Performers[0] != "Nata Ocean" {
		t.Errorf("Performers = %v", sc.Performers)
	}
	if sc.Thumbnail != "https://sexbabesvr.com/contents/videos_screenshots/0/764/preview.jpg" {
		t.Errorf("Thumbnail = %q", sc.Thumbnail)
	}
	wantDate := time.Date(2026, 6, 24, 7, 53, 39, 0, time.UTC)
	if !sc.Date.Equal(wantDate) {
		t.Errorf("Date = %v, want %v", sc.Date, wantDate)
	}
}

func TestFetchSceneOGFallback(t *testing.T) {
	ts := detailServer(t)
	defer ts.Close()

	s := New()
	s.Client = ts.Client()
	now := time.Date(2026, 6, 26, 0, 0, 0, 0, time.UTC)

	it := sitemapItem{id: "double-trouble-vicky-love-diane", url: ts.URL + "/video/double-trouble-vicky-love-diane/"}
	sc, err := s.fetchScene(context.Background(), ts.URL, it, now)
	if err != nil {
		t.Fatalf("fetchScene: %v", err)
	}
	if sc.Title != "Double Trouble" { // " - VR PORN" suffix stripped
		t.Errorf("Title = %q (want og fallback, suffix stripped)", sc.Title)
	}
	if sc.Thumbnail != "https://sexbabesvr.com/contents/dt.jpg" {
		t.Errorf("Thumbnail = %q", sc.Thumbnail)
	}
	if sc.Description != "Two babes one room." {
		t.Errorf("Description = %q", sc.Description)
	}
}

func TestListScenesEndToEnd(t *testing.T) {
	ts := detailServer(t)
	defer ts.Close()

	oldSM := sitemapURLs
	sitemapURLs = []string{
		ts.URL + "/sitemap/?type=videos&from_links_videos=1",
		ts.URL + "/sitemap/?type=videos&from_links_videos=2",
	}
	defer func() { sitemapURLs = oldSM }()

	s := New()
	s.Client = ts.Client()
	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{Workers: 2})
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
	// 2 from sitemap1 + 1 from sitemap2 = 3.
	if count != 3 {
		t.Errorf("got %d scenes, want 3", count)
	}
}

func TestMatchesURL(t *testing.T) {
	s := New()
	if !s.MatchesURL("https://sexbabesvr.com/video/foo/") {
		t.Error("expected sexbabesvr.com to match")
	}
	if s.MatchesURL("https://realjamvr.com/") {
		t.Error("should not match realjamvr.com")
	}
}

func TestParseDate(t *testing.T) {
	if got := parseDate("2026-06-24T07:53:39Z"); got.IsZero() {
		t.Error("parseDate returned zero for valid input")
	}
	if got := parseDate(""); !got.IsZero() {
		t.Errorf("parseDate(\"\") = %v, want zero", got)
	}
}
