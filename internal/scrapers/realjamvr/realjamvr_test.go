package realjamvr

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wasylq/FSS/scraper"
)

// sitemap1 references realjamvr.com URLs so parseSitemap can be tested directly;
// the end-to-end test uses sitemaps templated with the httptest server base.
const sitemap1 = `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9" xmlns:video="http://www.google.com/schemas/sitemap-video/1.1">
  <url>
    <loc>https://realjamvr.com/scene/my-girlfriends-little-stepsister/</loc>
    <lastmod>2022-12-29</lastmod>
    <video:video>
      <video:thumbnail_loc>https://cdn.example.com/scenes/my-girlfriends-little-stepsister/poster.webp</video:thumbnail_loc>
      <video:title><![CDATA[ My Girlfriend&#x27;s Little Stepsister ]]></video:title>
    </video:video>
  </url>
  <url>
    <loc>https://realjamvr.com/scene/second-scene/</loc>
    <lastmod>2023-01-15</lastmod>
    <video:video>
      <video:thumbnail_loc>https://cdn.example.com/scenes/second-scene/poster.webp</video:thumbnail_loc>
    </video:video>
  </url>
</urlset>`

const sitemap2 = `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url>
    <loc>https://realjamvr.com/scene/third-scene/</loc>
    <lastmod>2023-02-20</lastmod>
  </url>
  <url>
    <loc>https://realjamvr.com/some-other-page/</loc>
  </url>
</urlset>`

func sitemapForBase(base string) (string, string) {
	sm1 := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>%s/scene/my-girlfriends-little-stepsister/</loc><lastmod>2022-12-29</lastmod></url>
  <url><loc>%s/scene/second-scene/</loc><lastmod>2023-01-15</lastmod></url>
</urlset>`, base, base)
	sm2 := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>%s/scene/third-scene/</loc></url>
  <url><loc>%s/some-other-page/</loc></url>
</urlset>`, base, base)
	return sm1, sm2
}

// detailHTML mirrors the real page structure for the first scene.
const detailHTML = `<html><head>
<title>My Girlfriend&#x27;s Little Stepsister | RealJamVR</title>
<meta name="description" content="Watch My Girlfriend&#x27;s Little Stepsister VR porn video featuring Dakota Tyler, Lexi Lore in resolution 7K.">
</head><body>
<div class="specs-icon">
  <i class="bi bi-clock-history me-2"></i>
  <span style="font-weight:500;">1:05:25</span>
</div>
<div class="actors">
  Starring:
  <a href="/actor/dakota-tyler/">Dakota Tyler</a><span>, </span><a href="/actor/lexi-lore/">Lexi Lore</a>
</div>
<div class="align-self-center fs-8 text-end">
  May 26, 2023
</div>
</body></html>`

func detailServer(t *testing.T) *httptest.Server {
	t.Helper()
	var base string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sm1, sm2 := sitemapForBase(base)
		switch r.URL.Path {
		case "/sitemap-scenes-1.xml":
			_, _ = fmt.Fprint(w, sm1)
		case "/sitemap-scenes-2.xml":
			_, _ = fmt.Fprint(w, sm2)
		case "/scene/my-girlfriends-little-stepsister/":
			_, _ = fmt.Fprint(w, detailHTML)
		default:
			// Minimal valid page for the other scenes so the worker pool succeeds.
			_, _ = fmt.Fprint(w, `<title>Other Scene | RealJamVR</title><div>Starring: <a href="/actor/x/">Some Model</a></div>`)
		}
	}))
	base = ts.URL
	return ts
}

func TestParseSitemap(t *testing.T) {
	items := parseSitemap([]byte(sitemap1))
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	if items[0].id != "my-girlfriends-little-stepsister" {
		t.Errorf("id = %q", items[0].id)
	}
	if items[0].thumb != "https://cdn.example.com/scenes/my-girlfriends-little-stepsister/poster.webp" {
		t.Errorf("thumb = %q", items[0].thumb)
	}
	wantLM := time.Date(2022, 12, 29, 0, 0, 0, 0, time.UTC)
	if !items[0].lastmod.Equal(wantLM) {
		t.Errorf("lastmod = %v, want %v", items[0].lastmod, wantLM)
	}

	// sitemap2 has a non-scene URL that must be skipped.
	items2 := parseSitemap([]byte(sitemap2))
	if len(items2) != 1 || items2[0].id != "third-scene" {
		t.Errorf("sitemap2 items = %+v", items2)
	}
}

func TestFetchScene(t *testing.T) {
	ts := detailServer(t)
	defer ts.Close()

	s := New()
	s.Client = ts.Client()
	now := time.Date(2026, 6, 25, 0, 0, 0, 0, time.UTC)
	it := sitemapItem{
		id:      "my-girlfriends-little-stepsister",
		url:     ts.URL + "/scene/my-girlfriends-little-stepsister/",
		thumb:   "https://cdn.example.com/poster.webp",
		lastmod: time.Date(2022, 12, 29, 0, 0, 0, 0, time.UTC),
	}
	sc, err := s.fetchScene(context.Background(), ts.URL, it, now)
	if err != nil {
		t.Fatalf("fetchScene: %v", err)
	}
	if sc.ID != "my-girlfriends-little-stepsister" {
		t.Errorf("ID = %q", sc.ID)
	}
	if sc.Title != "My Girlfriend's Little Stepsister" {
		t.Errorf("Title = %q", sc.Title)
	}
	if sc.SiteID != siteID || sc.Studio != studioName {
		t.Errorf("SiteID/Studio = %q/%q", sc.SiteID, sc.Studio)
	}
	if sc.Duration != 3925 { // 1:05:25
		t.Errorf("Duration = %d, want 3925", sc.Duration)
	}
	if len(sc.Performers) != 2 || sc.Performers[0] != "Dakota Tyler" || sc.Performers[1] != "Lexi Lore" {
		t.Errorf("Performers = %v", sc.Performers)
	}
	if sc.Thumbnail != "https://cdn.example.com/poster.webp" {
		t.Errorf("Thumbnail = %q", sc.Thumbnail)
	}
	// On-page date overrides sitemap lastmod.
	wantDate := time.Date(2023, 5, 26, 0, 0, 0, 0, time.UTC)
	if !sc.Date.Equal(wantDate) {
		t.Errorf("Date = %v, want %v (on-page override)", sc.Date, wantDate)
	}
}

func TestListScenesEndToEnd(t *testing.T) {
	ts := detailServer(t)
	defer ts.Close()

	oldSM := sitemapURLs
	sitemapURLs = []string{ts.URL + "/sitemap-scenes-1.xml", ts.URL + "/sitemap-scenes-2.xml"}
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
	if !s.MatchesURL("https://realjamvr.com/scene/foo/") {
		t.Error("expected realjamvr.com to match")
	}
	if s.MatchesURL("https://sexbabesvr.com/") {
		t.Error("should not match sexbabesvr.com")
	}
}
