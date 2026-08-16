package porndudecasting

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
	"github.com/Anastylosis/FSS/models"
	"github.com/Anastylosis/FSS/scraper"
)

// entry reproduces one sitemap record. The description is escaped HTML inside
// CDATA, with inline chapter markers rendered as their own divs — stripping the
// tags leaves the timestamps loose in the prose unless they are removed too.
func entry(id int, slug, title, date string, tags []string) string {
	var tagXML strings.Builder
	for _, t := range tags {
		fmt.Fprintf(&tagXML, "<video:tag><![CDATA[%s]]></video:tag>", t)
	}
	return fmt.Sprintf(`<url>
  <loc>https://porndudecasting.com/models/%s/</loc>
  <lastmod>%s</lastmod>
  <video:video>
    <video:thumbnail_loc>https://pictures.example/contents/videos_screenshots/0/%d/preview.jpg</video:thumbnail_loc>
    <video:title><![CDATA[%s]]></video:title>
    <video:description><![CDATA[&lt;div&gt;A synopsis with &amp;ldquo;quotes&amp;rdquo;.&lt;br /&gt;&lt;/div&gt;
&lt;div&gt;&lt;div class=&#34;time blowjob&#34;&gt;06:45&lt;/div&gt;And it continues here.&lt;/div&gt;]]></video:description>
    <video:duration>2244</video:duration>
    <video:content_loc>https://porndudecasting.com/get_file/3/abc/0/%d/%d.mp4/</video:content_loc>
    <video:rating>4.5</video:rating>
    <video:view_count>81</video:view_count>
    <video:publication_date>%s</video:publication_date>
    <video:category><![CDATA[Professional Model]]></video:category>
    %s
  </video:video>
</url>`, slug, date, id, title, id, id, date, tagXML.String())
}

func sitemapXML(entries ...string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"
        xmlns:video="http://www.google.com/schemas/sitemap-video/1.1"
        xmlns:image="http://www.google.com/schemas/sitemap-image/1.1">` +
		strings.Join(entries, "\n") + `</urlset>`
}

func newServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sitemap/" || r.URL.Query().Get("type") != "videos" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/xml")
		_, _ = fmt.Fprint(w, body)
	}))
}

func newTestScraper(srv *httptest.Server) *Scraper {
	s := New()
	s.Client = srv.Client()
	s.baseOverride = srv.URL
	return s
}

func collect(t *testing.T, s *Scraper, opts scraper.ListOpts) ([]models.Scene, []error, int, bool) {
	t.Helper()
	ch, err := s.ListScenes(context.Background(), "https://porndudecasting.com/", opts)
	if err != nil {
		t.Fatalf("ListScenes: %v", err)
	}
	var scenes []models.Scene
	var errs []error
	total := 0
	stopped := false
	for res := range ch {
		switch res.Kind {
		case scraper.KindScene:
			scenes = append(scenes, res.Scene)
		case scraper.KindError:
			errs = append(errs, res.Err)
		case scraper.KindTotal:
			total = res.Total
		case scraper.KindStoppedEarly:
			stopped = true
		}
	}
	return scenes, errs, total, stopped
}

func TestSitemapIsTheWholeRecord(t *testing.T) {
	srv := newServer(t, sitemapXML(
		entry(846, "gracey-snow", "Gracey Snow Cures Her Cravings", "2026-08-14", []string{"blonde", "shaved"}),
	))
	defer srv.Close()

	scenes, errs, total, _ := collect(t, newTestScraper(srv), scraper.ListOpts{})
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(scenes) != 1 || total != 1 {
		t.Fatalf("got %d scenes (total %d), want 1", len(scenes), total)
	}
	got := scenes[0]

	// The id is the site's own video number, taken from the media path — the
	// model slug would collapse a model's scenes if one ever gained a second.
	if got.ID != "846" {
		t.Errorf("ID = %q, want the media-path number", got.ID)
	}
	if got.Title != "Gracey Snow Cures Her Cravings" {
		t.Errorf("Title = %q", got.Title)
	}
	if got.Duration != 2244 {
		t.Errorf("Duration = %d", got.Duration)
	}
	if got.Views != 81 {
		t.Errorf("Views = %d", got.Views)
	}
	if got.Date.Format("2006-01-02") != "2026-08-14" {
		t.Errorf("Date = %v", got.Date)
	}
	if strings.Join(got.Tags, ",") != "blonde,shaved" {
		t.Errorf("Tags = %v", got.Tags)
	}
	if strings.Join(got.Categories, ",") != "Professional Model" {
		t.Errorf("Categories = %v", got.Categories)
	}
	// The feed names no performer; the page URL's slug does.
	if len(got.Performers) != 1 || got.Performers[0] != "Gracey Snow" {
		t.Errorf("Performers = %v", got.Performers)
	}
	if got.Studio != studioName || got.SiteID != siteID {
		t.Errorf("Studio/SiteID = %q/%q", got.Studio, got.SiteID)
	}
}

// The description is doubly-escaped HTML with chapter timestamps rendered as
// their own divs. One unescape pass leaves markup; stripping tags without
// removing the timestamps leaves "06:45" loose in the prose.
func TestDescriptionIsFlattenedAndTimestampsRemoved(t *testing.T) {
	srv := newServer(t, sitemapXML(entry(846, "gracey-snow", "T", "2026-08-14", nil)))
	defer srv.Close()

	scenes, _, _, _ := collect(t, newTestScraper(srv), scraper.ListOpts{})
	got := scenes[0].Description

	for _, bad := range []string{"<div", "&lt;", "&amp;", "06:45"} {
		if strings.Contains(got, bad) {
			t.Errorf("description still contains %q: %q", bad, got)
		}
	}
	if !strings.Contains(got, "A synopsis with “quotes”.") {
		t.Errorf("description lost its prose: %q", got)
	}
	if !strings.Contains(got, "And it continues here.") {
		t.Errorf("description lost the text after the chapter marker: %q", got)
	}
}

// The feed is newest-first in practice, but sorting is what makes the KnownIDs
// stop mean "everything older is stored" rather than depend on that holding.
func TestScenesAreSortedNewestFirstAndKnownIDsStop(t *testing.T) {
	srv := newServer(t, sitemapXML(
		entry(800, "older-model", "Older", "2024-01-01", nil),
		entry(846, "newest-model", "Newest", "2026-08-14", nil),
		entry(820, "middle-model", "Middle", "2025-06-01", nil),
	))
	defer srv.Close()

	scenes, _, _, _ := collect(t, newTestScraper(srv), scraper.ListOpts{})
	if len(scenes) != 3 {
		t.Fatalf("got %d scenes, want 3", len(scenes))
	}
	if scenes[0].ID != "846" || scenes[2].ID != "800" {
		t.Errorf("order = %s,%s,%s", scenes[0].ID, scenes[1].ID, scenes[2].ID)
	}

	stoppedScenes, _, _, stopped := collect(t, newTestScraper(srv),
		scraper.ListOpts{KnownIDs: map[string]bool{"820": true}})
	if !stopped {
		t.Error("StoppedEarly was not reported")
	}
	if len(stoppedScenes) != 1 || stoppedScenes[0].ID != "846" {
		t.Errorf("got %v, want just the newest", stoppedScenes)
	}
}

// A sitemap that fetched and decoded but named no scenes is a feed change, not
// an empty catalogue, and must not read as one to an authoritative --full save.
func TestEmptySitemapIsAParseError(t *testing.T) {
	srv := newServer(t, sitemapXML())
	defer srv.Close()

	scenes, errs, _, _ := collect(t, newTestScraper(srv), scraper.ListOpts{})
	if len(scenes) != 0 {
		t.Fatalf("got %d scenes", len(scenes))
	}
	if len(errs) == 0 {
		t.Fatal("an empty sitemap reported no error")
	}
	if k := scraper.Classify(errs[0]); k != scraper.FailureParse {
		t.Errorf("classified as %v, want FailureParse", k)
	}
}

// An entry with no media path has no id to store it under, and one with no
// title has nothing to match on; both are dropped rather than stored broken.
func TestEntriesWithoutAnIDOrTitleAreDropped(t *testing.T) {
	srv := newServer(t, sitemapXML(
		`<url><loc>https://porndudecasting.com/models/x/</loc><video:video>
		   <video:title><![CDATA[No media path]]></video:title></video:video></url>`,
		entry(846, "gracey-snow", "", "2026-08-14", nil),
		entry(847, "real-model", "A Real Scene", "2026-08-15", nil),
	))
	defer srv.Close()

	scenes, _, _, _ := collect(t, newTestScraper(srv), scraper.ListOpts{})
	if len(scenes) != 1 || scenes[0].ID != "847" {
		t.Fatalf("got %v, want only the complete entry", scenes)
	}
}

func TestMatchesURL(t *testing.T) {
	s := New()
	for _, u := range []string{
		"https://porndudecasting.com",
		"https://www.porndudecasting.com/",
		"http://porndudecasting.com/models/gracey-snow/",
	} {
		if !s.MatchesURL(u) {
			t.Errorf("MatchesURL(%q) = false, want true", u)
		}
	}
	for _, u := range []string{"https://porndude.com/", "https://example.com/porndudecasting.com"} {
		if s.MatchesURL(u) {
			t.Errorf("MatchesURL(%q) = true, want false", u)
		}
	}
}

func TestScenesValidate(t *testing.T) {
	srv := newServer(t, sitemapXML(
		entry(846, "gracey-snow", "One", "2026-08-14", []string{"blonde"}),
		entry(844, "another-model", "Two", "2026-08-10", nil),
	))
	defer srv.Close()

	scenes, _, _, _ := collect(t, newTestScraper(srv), scraper.ListOpts{})
	for _, sc := range scenes {
		testutil.ValidateScene(t, sc)
	}
}

func TestContextCancellationStopsTheWalk(t *testing.T) {
	srv := newServer(t, sitemapXML(entry(846, "gracey-snow", "One", "2026-08-14", nil)))
	defer srv.Close()

	s := newTestScraper(srv)
	ctx, cancel := context.WithCancel(context.Background())
	ch, err := s.ListScenes(ctx, "https://porndudecasting.com/", scraper.ListOpts{Delay: 10 * time.Millisecond})
	if err != nil {
		t.Fatalf("ListScenes: %v", err)
	}
	cancel()
	for range ch { //nolint:revive // drain so the goroutine can finish its sends
	}
}
