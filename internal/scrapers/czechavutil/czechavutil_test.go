package czechavutil

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
	"github.com/Wasylq/FSS/scraper"
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
		t.Errorf("duration = %d, want 1120", u.Video.Duration)
	}
	if u.Video.PubDate != "2015-04-03" {
		t.Errorf("pubdate = %q", u.Video.PubDate)
	}
	if u.Video.Thumbnail != "https://cdn77.hqmediago.com/files/czechcasting.com/e1281/orig/poster-1.jpg" {
		t.Errorf("thumb = %q", u.Video.Thumbnail)
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
		Cfg:    SiteConfig{SiteID: "test", Domain: domain, Studio: "Test"},
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
		Cfg:    SiteConfig{SiteID: "test", Domain: domain, Studio: "Test"},
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
	var _ scraper.StudioScraper = NewScraper(SiteConfig{SiteID: "test", Domain: "test.com", Studio: "Test"})
}

func TestMatchesURL(t *testing.T) {
	s := NewScraper(SiteConfig{SiteID: "czechcasting", Domain: "czechcasting.com", Studio: "Czech Casting"})
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
		Cfg:    SiteConfig{SiteID: "test", Domain: domain, Studio: "Test"},
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
