package cearalynch

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
	"github.com/Wasylq/FSS/scraper"
)

const sitemapFixture = `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
<url><loc>http://www.cearalynch.com/</loc><changefreq>daily</changefreq><priority>1.0</priority></url>
<url><loc>http://www.cearalynch.com/content/loser-mark</loc><lastmod>2014-01-01T12:44Z</lastmod><changefreq>never</changefreq></url>
<url><loc>http://www.cearalynch.com/video/booty-basement</loc><lastmod>2017-07-10T14:01Z</lastmod><changefreq>never</changefreq></url>
<url><loc>http://www.cearalynch.com/video/light-snack</loc><lastmod>2017-07-10T14:01Z</lastmod><changefreq>never</changefreq></url>
<url><loc>http://www.cearalynch.com/gallery/some-gallery</loc><lastmod>2017-07-10T14:01Z</lastmod></url>
<url><loc>http://www.cearalynch.com/links/foo</loc></url>
<url><loc>http://www.cearalynch.com/video/booty-basement</loc><lastmod>2017-07-10T14:01Z</lastmod></url>
</urlset>`

const detailFixture = `<!DOCTYPE html>
<html><head>
<meta name="generator" content="Drupal 7 (http://drupal.org)" />
<title>Booty Basement  | Ceara Lynch</title>
<meta name="description" content="Youve waited all day &amp; night in my dark basement." />
</head><body>
<img typeof="foaf:Image" class="img-responsive" src="https://www.cearalynch.com/sites/default/files/video_image/bootybasement.gif" width="350" height="197" alt="" />
</body></html>`

func TestParseSitemap(t *testing.T) {
	entries := parseSitemap(siteBase, []byte(sitemapFixture))
	// Only the two distinct /video/ entries should survive (dedup drops the
	// repeated booty-basement; home/content/gallery/links are filtered out).
	if len(entries) != 2 {
		t.Fatalf("expected 2 video entries, got %d: %+v", len(entries), entries)
	}
	if entries[0].slug != "booty-basement" {
		t.Errorf("slug = %q, want booty-basement", entries[0].slug)
	}
	if entries[0].url != "https://www.cearalynch.com/video/booty-basement" {
		t.Errorf("url = %q", entries[0].url)
	}
	if entries[0].lastmod.Format("2006-01-02") != "2017-07-10" {
		t.Errorf("lastmod = %v, want 2017-07-10", entries[0].lastmod)
	}
	if entries[1].slug != "light-snack" {
		t.Errorf("entries[1].slug = %q, want light-snack", entries[1].slug)
	}
}

func TestParseDetail(t *testing.T) {
	d := parseDetail([]byte(detailFixture))
	if d.title != "Booty Basement" {
		t.Errorf("title = %q, want Booty Basement", d.title)
	}
	if d.description != "Youve waited all day & night in my dark basement." {
		t.Errorf("description = %q", d.description)
	}
	if d.thumbnail != "https://www.cearalynch.com/sites/default/files/video_image/bootybasement.gif" {
		t.Errorf("thumbnail = %q", d.thumbnail)
	}
}

func TestParseDetailMissingTitle(t *testing.T) {
	d := parseDetail([]byte(`<html><head></head><body></body></html>`))
	if d.title != "" {
		t.Errorf("title = %q, want empty", d.title)
	}
}

func TestSlugToTitle(t *testing.T) {
	if got := slugToTitle("booty-basement"); got != "booty basement" {
		t.Errorf("slugToTitle = %q", got)
	}
}

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := map[string]bool{
		"https://www.cearalynch.com/":                 true,
		"https://cearalynch.com/video/booty-basement": true,
		"http://www.cearalynch.com/sitemap.xml":       true,
		"https://example.com/cearalynch":              false,
		"https://notcearalynch.com/":                  false,
	}
	for u, want := range cases {
		if got := s.MatchesURL(u); got != want {
			t.Errorf("MatchesURL(%q) = %v, want %v", u, got, want)
		}
	}
}

// --- end to end ---------------------------------------------------------------
//
// The tests above call the parsers directly, which left ListScenes, run,
// fetchPage and the sitemap walk at 0% and the package at 35.6%. The base is now
// a field and parseSitemap takes it, so the sitemap -> detail walk runs offline.
func TestListScenesEndToEnd(t *testing.T) {
	// Handlers run on separate goroutines (Workers > 1), so this must be atomic.
	var detailHits atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/sitemap.xml":
			// Served verbatim: parseSitemap only takes the *slug* from each <loc>
			// and rebuilds the URL from the base, so detail fetches still land here.
			_, _ = fmt.Fprint(w, sitemapFixture)
		case strings.HasPrefix(r.URL.Path, "/video/"):
			detailHits.Add(1)
			_, _ = fmt.Fprint(w, detailFixture)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	s := New()
	s.client = ts.Client()
	s.base = ts.URL

	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{Workers: 2})
	if err != nil {
		t.Fatal(err)
	}
	scenes := testutil.CollectScenes(t, ch)
	// The sitemap holds two distinct /video/ slugs plus a duplicate, and several
	// non-video paths (/content/, /gallery/, /links/) that must be ignored.
	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2 (non-video paths skipped, duplicate deduped)", len(scenes))
	}
	if got := detailHits.Load(); got != 2 {
		t.Errorf("detail fetches = %d, want 2", got)
	}
	for _, sc := range scenes {
		if !strings.HasPrefix(sc.URL, ts.URL) {
			t.Errorf("scene %q URL %q escaped the test server", sc.ID, sc.URL)
		}
		if sc.Title == "" {
			t.Errorf("scene %q has empty Title", sc.ID)
		}
	}
}
