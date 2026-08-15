package czechavutil

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
	"github.com/Anastylosis/FSS/scraper"
)

func TestExtractSlug(t *testing.T) {
	cases := []struct {
		loc    string
		domain string
		want   string
	}{
		{"https://czechcasting.com/sitemap.xml/video/czech-casting-tana-2081/", "czechcasting.com", "czech-casting-tana-2081"},
		{"https://horrorporn.com/sitemap.xml/video/horror-porn-1-demonic-beauty/", "horrorporn.com", "horror-porn-1-demonic-beauty"},
		{"https://czechcasting.com/video/czech-casting-tana-2081/", "czechcasting.com", "czech-casting-tana-2081"},
		{"https://other.com/sitemap.xml/video/slug/", "czechcasting.com", ""},
		{"https://czechcasting.com/pages/tags/", "czechcasting.com", ""},
	}
	for _, c := range cases {
		if got := ExtractSlug(c.loc, c.domain); got != c.want {
			t.Errorf("ExtractSlug(%q, %q) = %q, want %q", c.loc, c.domain, got, c.want)
		}
	}
}

func TestParseSitemap(t *testing.T) {
	body := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"
        xmlns:video="http://www.google.com/schemas/sitemap-video/1.1">
<url>
<loc>https://czechcasting.com/sitemap.xml/video/czech-casting-tana-2081/</loc>
<lastmod>2015-04-03</lastmod>
<video:video>
<video:thumbnail_loc>https://cdn77.hqmediago.com/files/czechcasting.com/e1281/orig/poster-1.jpg</video:thumbnail_loc>
<video:title><![CDATA[Tana (18)]]></video:title>
<video:description><![CDATA[Beautiful Tana shows up.]]></video:description>
<video:duration>1120</video:duration>
<video:publication_date>2015-04-03</video:publication_date>
</video:video>
</url>
<url>
<loc>https://czechcasting.com/sitemap.xml/video/czech-casting-petra-2082/</loc>
<lastmod>2015-04-10</lastmod>
<video:video>
<video:thumbnail_loc>https://cdn77.hqmediago.com/files/czechcasting.com/e1282/orig/poster-1.jpg</video:thumbnail_loc>
<video:title><![CDATA[Petra (21)]]></video:title>
<video:description><![CDATA[Petra arrives.]]></video:description>
<video:duration>930</video:duration>
<video:publication_date>2015-04-10</video:publication_date>
</video:video>
</url>
</urlset>`)

	urls := ParseSitemap(body)
	if len(urls) != 2 {
		t.Fatalf("got %d URLs, want 2", len(urls))
	}

	u := urls[0]
	if u.Video.Title != "Tana (18)" {
		t.Errorf("title = %q", u.Video.Title)
	}
	if u.Video.Description != "Beautiful Tana shows up." {
		t.Errorf("desc = %q", u.Video.Description)
	}
	if u.Video.Duration != 1120 {
		t.Errorf("duration = %v, want 1120", u.Video.Duration)
	}
	if u.Video.PubDate != "2015-04-03" {
		t.Errorf("pubdate = %q", u.Video.PubDate)
	}
	if u.Video.Thumbnail != "https://cdn77.hqmediago.com/files/czechcasting.com/e1281/orig/poster-1.jpg" {
		t.Errorf("thumb = %q", u.Video.Thumbnail)
	}
}

func TestParseSitemapFloatDuration(t *testing.T) {
	body := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"
        xmlns:video="http://www.google.com/schemas/sitemap-video/1.1">
<url>
<loc>https://czechcasting.com/video/tana-2081/</loc>
<lastmod>2025-12-01</lastmod>
<video:video>
<video:thumbnail_loc>https://cdn77.hqmediago.com/thumb.jpg</video:thumbnail_loc>
<video:title><![CDATA[Tana]]></video:title>
<video:description><![CDATA[Desc.]]></video:description>
<video:duration>1787.68</video:duration>
<video:publication_date>2025-12-01</video:publication_date>
</video:video>
</url>
</urlset>`)

	urls := ParseSitemap(body)
	if len(urls) != 1 {
		t.Fatalf("got %d URLs, want 1", len(urls))
	}
	if urls[0].Video.Duration != 1787.68 {
		t.Errorf("duration = %v, want 1787.68", urls[0].Video.Duration)
	}
}

func TestParseSitemapEmpty(t *testing.T) {
	urls := ParseSitemap([]byte(`<html>not xml</html>`))
	if len(urls) != 0 {
		t.Errorf("got %d URLs, want 0", len(urls))
	}
}

func TestParseSitemapControlChars(t *testing.T) {
	body := []byte("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n" +
		"<urlset xmlns=\"http://www.sitemaps.org/schemas/sitemap/0.9\"\n" +
		"        xmlns:video=\"http://www.google.com/schemas/sitemap-video/1.1\">\n" +
		"<url>\n" +
		"<loc>https://example.com/sitemap.xml/video/scene-1/</loc>\n" +
		"<video:video>\n" +
		"<video:title><![CDATA[Scene with control\x19char]]></video:title>\n" +
		"<video:description><![CDATA[She\x19s ready]]></video:description>\n" +
		"<video:duration>600</video:duration>\n" +
		"<video:publication_date>2026-01-15</video:publication_date>\n" +
		"</video:video>\n" +
		"</url>\n" +
		"</urlset>")

	urls := ParseSitemap(body)
	if len(urls) != 1 {
		t.Fatalf("got %d URLs, want 1", len(urls))
	}
	if urls[0].Video.Title != "Scene with controlchar" {
		t.Errorf("title = %q", urls[0].Video.Title)
	}
}

func TestParseDetailPage(t *testing.T) {
	body := []byte(`<html><head>
<script type="application/ld+json">
{
  "@type": "VideoObject",
  "name": "Horror Porn 1",
  "actor": ["vinna reed", "thomas lee"],
  "keywords": "blowjob, rough, blonde"
}
</script>
</head><body></body></html>`)

	d := ParseDetailPage(body)
	if len(d.Performers) != 2 || d.Performers[0] != "vinna reed" || d.Performers[1] != "thomas lee" {
		t.Errorf("performers = %v", d.Performers)
	}
	if len(d.Tags) != 3 || d.Tags[0] != "blowjob" || d.Tags[1] != "rough" || d.Tags[2] != "blonde" {
		t.Errorf("tags = %v", d.Tags)
	}
}

func TestParseDetailPageActorObjects(t *testing.T) {
	body := []byte(`<script type="application/ld+json">
{"@type": "VideoObject", "actor": [{"name": "Alice"}, {"name": "Bob"}], "keywords": ""}
</script>`)

	d := ParseDetailPage(body)
	if len(d.Performers) != 2 || d.Performers[0] != "Alice" || d.Performers[1] != "Bob" {
		t.Errorf("performers = %v", d.Performers)
	}
}

func TestParseDetailPageHTMLPerformers(t *testing.T) {
	body := []byte(`<script type="application/ld+json">
{"@type": "VideoObject", "keywords": "tag1"}
</script>
<a href="/pages/search/?q=jane+doe&adult-performer&key=420" class="inline text-link--secondary text--capitalize">jane doe</a>
<a href="/pages/search/?q=john+smith&adult-performer&key=421" class="inline text-link--secondary text--capitalize">john smith</a>`)

	d := ParseDetailPage(body)
	if len(d.Performers) != 2 || d.Performers[0] != "jane doe" || d.Performers[1] != "john smith" {
		t.Errorf("performers = %v", d.Performers)
	}
}

func TestParseDetailPageNoJSONLD(t *testing.T) {
	d := ParseDetailPage([]byte(`<html><body>no json-ld here</body></html>`))
	if len(d.Performers) != 0 || len(d.Tags) != 0 {
		t.Errorf("expected empty detail, got %+v", d)
	}
}

const sitemapTpl = `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"
        xmlns:video="http://www.google.com/schemas/sitemap-video/1.1">
%s
</urlset>`

const sitemapEntryTpl = `<url>
<loc>http://%s/sitemap.xml/video/scene-%d/</loc>
<lastmod>2026-01-15</lastmod>
<video:video>
<video:thumbnail_loc>https://cdn.test/thumb-%d.jpg</video:thumbnail_loc>
<video:title><![CDATA[Scene %d]]></video:title>
<video:description><![CDATA[Description %d.]]></video:description>
<video:duration>600</video:duration>
<video:publication_date>2026-01-15</video:publication_date>
</video:video>
</url>`

const detailTpl = `<html><head>
<script type="application/ld+json">
{"@type": "VideoObject", "actor": ["Performer %d"], "keywords": "tag1, tag2"}
</script>
</head><body></body></html>`

func buildSitemap(domain string, ids []int) string {
	var entries string
	for _, id := range ids {
		entries += fmt.Sprintf(sitemapEntryTpl, domain, id, id, id, id)
	}
	return fmt.Sprintf(sitemapTpl, entries)
}

func newTestServer(ids []int) (*httptest.Server, string) {
	var ts *httptest.Server
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		domain := ts.Listener.Addr().String()
		switch r.URL.Path {
		case "/sitemap.xml":
			w.Header().Set("Content-Type", "application/xml")
			_, _ = fmt.Fprint(w, buildSitemap(domain, ids))
		default:
			w.Header().Set("Content-Type", "text/html")
			var id int
			_, _ = fmt.Sscanf(r.URL.Path, "/video/scene-%d/", &id)
			if id > 0 {
				_, _ = fmt.Fprintf(w, detailTpl, id)
			} else {
				w.WriteHeader(404)
			}
		}
	}))
	return ts, ts.Listener.Addr().String()
}

func TestListScenes(t *testing.T) {
	ts, domain := newTestServer([]int{3, 2, 1})
	defer ts.Close()

	s := &Scraper{
		cfg:    SiteConfig{SiteID: "test", Domain: domain, Studio: "Test"},
		Client: ts.Client(),
		Base:   ts.URL,
	}

	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}

	results := testutil.CollectScenes(t, ch)
	if len(results) != 3 {
		t.Fatalf("got %d scenes, want 3", len(results))
	}
	for _, sc := range results {
		if sc.SiteID != "test" {
			t.Errorf("siteID = %q", sc.SiteID)
		}
		if sc.Studio != "Test" {
			t.Errorf("studio = %q", sc.Studio)
		}
		if sc.Duration != 600 {
			t.Errorf("duration = %d, want 600", sc.Duration)
		}
		if len(sc.Performers) != 1 {
			t.Errorf("performers = %v", sc.Performers)
		}
		if len(sc.Tags) != 2 {
			t.Errorf("tags = %v", sc.Tags)
		}
	}
}

func TestKnownIDsStopsEarly(t *testing.T) {
	ts, domain := newTestServer([]int{3, 2, 1})
	defer ts.Close()

	s := &Scraper{
		cfg:    SiteConfig{SiteID: "test", Domain: domain, Studio: "Test"},
		Client: ts.Client(),
		Base:   ts.URL,
	}

	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{
		KnownIDs: map[string]bool{"scene-2": true},
	})
	if err != nil {
		t.Fatal(err)
	}

	results, stoppedEarly := testutil.CollectScenesWithStop(t, ch)
	if !stoppedEarly {
		t.Error("expected StoppedEarly signal")
	}
	if len(results) != 1 {
		t.Fatalf("got %d scenes, want 1", len(results))
	}
}

func TestScraperInterface(t *testing.T) {
	var _ scraper.StudioScraper = New(SiteConfig{SiteID: "test", Domain: "test.com", Studio: "Test"})
}

func TestMatchesURL(t *testing.T) {
	s := New(SiteConfig{SiteID: "czechcasting", Domain: "czechcasting.com", Studio: "Czech Casting"})
	cases := []struct {
		url  string
		want bool
	}{
		{"https://czechcasting.com/", true},
		{"https://www.czechcasting.com/video/scene-1/", true},
		{"https://horrorporn.com/", false},
		{"", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.want {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.want)
		}
	}
}

func TestSceneDate(t *testing.T) {
	ts, domain := newTestServer([]int{1})
	defer ts.Close()

	s := &Scraper{
		cfg:    SiteConfig{SiteID: "test", Domain: domain, Studio: "Test"},
		Client: ts.Client(),
		Base:   ts.URL,
	}

	ch, _ := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{})
	results := testutil.CollectScenes(t, ch)
	if len(results) != 1 {
		t.Fatal("expected 1 scene")
	}
	wantDate := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	if !results[0].Date.Equal(wantDate) {
		t.Errorf("date = %v, want %v", results[0].Date, wantDate)
	}
}

const searchDetailTpl = `<html><head>
<script type="application/ld+json">
{"@type":"VideoObject","name":"Scene %d","description":"Desc %d","thumbnailUrl":"https://cdn.test/thumb-%d.jpg","duration":"PT10M","uploadDate":"2026-03-01","actor":["Tereza"],"keywords":"casting"}
</script>
</head><body></body></html>`

func TestPerformerSearch(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/pages/search/":
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprint(w, `<html><body>
				<a href="/video/scene-1/">Scene 1</a>
				<a href="/video/scene-2/">Scene 2</a>
			</body></html>`)
		case strings.HasPrefix(r.URL.Path, "/video/scene-"):
			w.Header().Set("Content-Type", "text/html")
			var id int
			_, _ = fmt.Sscanf(r.URL.Path, "/video/scene-%d/", &id)
			_, _ = fmt.Fprintf(w, searchDetailTpl, id, id, id)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	domain := ts.Listener.Addr().String()
	s := &Scraper{
		cfg:    SiteConfig{SiteID: "test", Domain: domain, Studio: "Test"},
		Client: ts.Client(),
		Base:   ts.URL,
	}

	ch, err := s.ListScenes(context.Background(), ts.URL+"/pages/search/?q=Tereza&adult-performer", scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}

	results := testutil.CollectScenes(t, ch)
	if len(results) != 2 {
		t.Fatalf("got %d scenes, want 2", len(results))
	}
	if results[0].Performers[0] != "Tereza" {
		t.Errorf("performers = %v", results[0].Performers)
	}
	if results[0].Duration != 600 {
		t.Errorf("duration = %d, want 600", results[0].Duration)
	}
}

// The live sitemap is in no order at all — czechcasting's runs 2011-11-22,
// 2011-11-05, 2012-01-09 … 2026-04-28, 2026-08-17, 2024-10-08 — so entries are
// sorted on the sitemap's own publication_date before the KnownIDs stop is
// applied. Without that the stop aborts at whatever position the first stored
// scene occupies and permanently misses everything after it.
func TestSortNewestFirst(t *testing.T) {
	entry := func(slug, date string) sceneEntry {
		var e sceneEntry
		e.slug = slug
		e.url.Video.PubDate = date
		return e
	}
	entries := []sceneEntry{
		entry("old", "2011-11-22"),
		entry("newest", "2026-08-17"),
		entry("undated", ""),
		entry("middle", "2024-10-08"),
		entry("unparseable", "not-a-date"),
	}
	sortNewestFirst(entries)

	got := make([]string, len(entries))
	for i, e := range entries {
		got[i] = e.slug
	}
	// Undated entries lead, in their original relative order, so a stop can
	// never skip past one; the dated ones follow newest-first.
	want := []string{"undated", "unparseable", "newest", "middle", "old"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order = %v, want %v", got, want)
		}
	}
}

// A performer page is a single already-fetched page whose scenes are in no
// particular order, so a stop there saves nothing and would drop everything
// listed after the first stored slug. Known slugs are skipped instead, and the
// run says so.
func TestPerformerPageSkipsKnownSlugsInsteadOfStopping(t *testing.T) {
	slugs := []string{"alpha", "beta", "gamma"}

	var sb strings.Builder
	sb.WriteString(`<html><body>`)
	for _, s := range slugs {
		fmt.Fprintf(&sb, `<a href="/video/%s/">%s</a>`, s, s)
	}
	sb.WriteString(`</body></html>`)
	performerPage := sb.String()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/video/") {
			slug := strings.Trim(strings.TrimPrefix(r.URL.Path, "/video/"), "/")
			_, _ = fmt.Fprintf(w, `<html><head>
<meta property="og:title" content="Scene %s" />
<meta property="og:description" content="desc" />
<meta property="og:image" content="https://cdn.example/%s.jpg" />
</head><body></body></html>`, slug, slug)
			return
		}
		_, _ = fmt.Fprint(w, performerPage)
	}))
	defer ts.Close()

	s := New(SiteConfig{SiteID: "czechcasting", Domain: "czechcasting.com", Studio: "Czech Casting"})
	s.Client = ts.Client()
	s.Base = ts.URL

	ch, err := s.ListScenes(context.Background(), ts.URL+"/pages/search/?q=Somebody&adult-performer", scraper.ListOpts{
		Workers: 1,
		// The FIRST of three. A truncating stop would yield nothing at all.
		KnownIDs: map[string]bool{"alpha": true},
	})
	if err != nil {
		t.Fatalf("ListScenes: %v", err)
	}

	var ids []string
	stopped := false
	for r := range ch {
		switch r.Kind {
		case scraper.KindScene:
			ids = append(ids, r.Scene.ID)
		case scraper.KindStoppedEarly:
			stopped = true
		}
	}
	if len(ids) != 2 {
		t.Fatalf("got %v, want the two unknown slugs", ids)
	}
	if !stopped {
		t.Error("skipping stored scenes did not report StoppedEarly")
	}
	for _, id := range ids {
		if id == "alpha" {
			t.Error("a stored scene was re-fetched")
		}
	}
}
