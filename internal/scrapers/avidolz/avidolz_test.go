package avidolz

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
	"github.com/Wasylq/FSS/scraper"
)

func readFixture(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/detail.html")
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	return b
}

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url   string
		match bool
	}{
		{"https://avidolz.com", true},
		{"https://avidolz.com/japan-porn/", true},
		{"https://www.avidolz.com/seductive-masseuse-mirai-haneda-likes-her-job/", true},
		{"http://avidolz.com/jav-models/mirai-haneda/", true},
		{"https://avidolz.net/", false},
		{"", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.match {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.match)
		}
	}
}

func TestID(t *testing.T) {
	if got := New().ID(); got != siteID {
		t.Errorf("ID() = %q, want %q", got, siteID)
	}
}

func TestParsePage(t *testing.T) {
	const pageURL = "https://avidolz.com/seductive-masseuse-mirai-haneda-likes-her-job/"
	now := time.Now().UTC()
	date := time.Date(2020, time.May, 3, 14, 3, 35, 0, time.UTC)

	scene, skip, err := parsePage("https://avidolz.com", pageURL, readFixture(t), now, date)
	if err != nil {
		t.Fatalf("parsePage: %v", err)
	}
	if skip {
		t.Fatal("scene page was skipped")
	}

	// There is no numeric id anywhere on the site, so the slug is the key.
	if scene.ID != "seductive-masseuse-mirai-haneda-likes-her-job" {
		t.Errorf("ID = %q", scene.ID)
	}
	if scene.SiteID != siteID || scene.Studio != studioName {
		t.Errorf("SiteID = %q, Studio = %q", scene.SiteID, scene.Studio)
	}
	// Taken from the info panel, so it has no " | AvidolZ" suffix.
	if scene.Title != "Seductive masseuse, Mirai Haneda likes her job" {
		t.Errorf("Title = %q", scene.Title)
	}
	if !slices.Equal(scene.Performers, []string{"Mirai Haneda"}) {
		t.Errorf("Performers = %v", scene.Performers)
	}
	wantCats := []string{"Big Tits", "Blowjob", "Hand Job", "HD"}
	if !slices.Equal(scene.Categories, wantCats) {
		t.Errorf("Categories = %v, want %v", scene.Categories, wantCats)
	}
	// "31Min 19sec"
	if scene.Duration != 1879 {
		t.Errorf("Duration = %d, want 1879", scene.Duration)
	}
	if scene.Width != 1920 || scene.Height != 1080 || scene.Resolution != "1080p" {
		t.Errorf("resolution = %dx%d %q", scene.Width, scene.Height, scene.Resolution)
	}
	if scene.Series != "Idol Premium Collection" {
		t.Errorf("Series = %q", scene.Series)
	}
	if !strings.HasPrefix(scene.Description, "Big titted masseuse") {
		t.Errorf("Description = %q", scene.Description)
	}
	// The description must not be doubled up by a greedy match.
	if strings.Count(scene.Description, "Big titted masseuse") != 1 {
		t.Errorf("Description repeats itself: %q", scene.Description)
	}
	// Protocol-relative CDN URLs get upgraded to https.
	if !strings.HasPrefix(scene.Thumbnail, "https://") {
		t.Errorf("Thumbnail = %q", scene.Thumbnail)
	}
	// The date can only come from the sitemap; the page has none.
	if !scene.Date.Equal(date) {
		t.Errorf("Date = %v, want %v", scene.Date, date)
	}
}

// A page with no title is a category page — those share the root namespace
// with scenes, so they can reach the parser and must be skipped.
func TestParsePageSkipsNonScene(t *testing.T) {
	body := []byte(`<html><head></head><body><h1>Big Tits</h1></body></html>`)
	_, skip, err := parsePage("https://avidolz.com", "https://avidolz.com/big-tits/", body, time.Now(), time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if !skip {
		t.Error("a page with no title should be skipped")
	}
}

func TestParsePageTitleFallsBackToTitleTag(t *testing.T) {
	body := []byte(`<html><head><title>Some Scene | AvidolZ</title></head><body></body></html>`)
	scene, skip, err := parsePage("https://avidolz.com", "https://avidolz.com/some-scene/", body, time.Now(), time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if skip {
		t.Fatal("should not skip: the title tag carries a title")
	}
	if scene.Title != "Some Scene" {
		t.Errorf("Title = %q, want %q (suffix stripped)", scene.Title, "Some Scene")
	}
}

// Cast must come from the schema.org actor markup. Bare /jav-models/ links
// also appear in related-models blocks and are not this scene's cast.
func TestPerformersScopedToActorMarkup(t *testing.T) {
	body := []byte(`
<p><strong>JAV Model: </strong><span itemprop="actor" itemscope itemtype="http://schema.org/Person"><a itemprop="url" href="/jav-models/mirai-haneda/" rel="tag"><span itemprop="name">Mirai Haneda</span></a></span></p>
<div class="related-models">
  <a href="/jav-models/sara-yurikawa/">Sara Yurikawa</a>
  <a href="/jav-models/natsu-ando/">Natsu Ando</a>
</div>`)

	got := dedupe(actorRe, string(body))
	if !slices.Equal(got, []string{"Mirai Haneda"}) {
		t.Errorf("performers = %v, want only the credited actor", got)
	}
}

func TestParseDuration(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{`<strong>Duration:</strong> 31Min 19sec`, 1879},
		{`<strong>Duration:</strong> 5Min 0sec`, 300},
		{`<strong>Duration:</strong> 45Min`, 2700},
		{`<strong>Duration:</strong> 30sec`, 30},
		{`no duration here`, 0},
	}
	for _, c := range cases {
		if got := parseDuration(c.in); got != c.want {
			t.Errorf("parseDuration(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestNormalizeURL(t *testing.T) {
	cases := map[string]string{
		"//static.avidolz.com/a.jpg":       "https://static.avidolz.com/a.jpg",
		"https://static.avidolz.com/b.jpg": "https://static.avidolz.com/b.jpg",
		"http://x/c.jpg":                   "http://x/c.jpg",
	}
	for in, want := range cases {
		if got := normalizeURL(in); got != want {
			t.Errorf("normalizeURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDedupe(t *testing.T) {
	body := `<a itemprop="genre" href="/a/">Big Tits</a><a itemprop="genre" href="/b/">Big Tits</a><a itemprop="genre" href="/c/">HD</a>`
	if got := dedupe(genreRe, body); !slices.Equal(got, []string{"Big Tits", "HD"}) {
		t.Errorf("dedupe = %v", got)
	}
}

// ---- end-to-end ----

// The sitemap <lastmod> must reach the scene: it is the only date the site has,
// so losing it would leave every scene undated.
func TestListScenesUsesSitemapLastMod(t *testing.T) {
	detail := readFixture(t)

	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "wp-sitemap-posts-vms_videos") {
			w.Header().Set("Content-Type", "application/xml")
			_, _ = fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>%s/scene-one/</loc><lastmod>2020-05-03T14:03:35+00:00</lastmod></url>
  <url><loc>%s/scene-two/</loc><lastmod>2020-07-06T06:23:30+00:00</lastmod></url>
</urlset>`, srv.URL, srv.URL)
			return
		}
		_, _ = w.Write(detail)
	}))
	defer srv.Close()

	orig := siteBase
	siteBase = srv.URL
	defer func() { siteBase = orig }()

	s := New()
	s.client = srv.Client()

	ch, err := s.ListScenes(context.Background(), srv.URL, scraper.ListOpts{Workers: 1})
	if err != nil {
		t.Fatal(err)
	}
	scenes := testutil.CollectScenes(t, ch)
	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2", len(scenes))
	}

	byURL := map[string]time.Time{}
	for _, sc := range scenes {
		if sc.Date.IsZero() {
			t.Errorf("scene %s has no date; the sitemap lastmod was lost", sc.ID)
		}
		byURL[sc.URL] = sc.Date
	}
	want := time.Date(2020, time.May, 3, 14, 3, 35, 0, time.UTC)
	if got := byURL[srv.URL+"/scene-one/"]; !got.Equal(want) {
		t.Errorf("scene-one date = %v, want %v", got, want)
	}
}
