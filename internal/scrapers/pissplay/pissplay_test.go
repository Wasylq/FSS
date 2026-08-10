package pissplay

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
	"github.com/Anastylosis/FSS/scraper"
)

const sitemapFixture = `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
	<url><loc>https://pissplay.com/</loc></url>
	<url><loc>https://pissplay.com/blog</loc></url>
	<url><loc>https://pissplay.com/models</loc></url>
	<url><loc>https://pissplay.com/videos/non-stop-piss-drinking</loc></url>
	<url><loc>https://pissplay.com/videos/folsom-fair-dp-afterparty</loc></url>
	<url><loc>https://pissplay.com/blog/top-10-videos-of-2025</loc></url>
	<url><loc>https://pissplay.com/videos/non-stop-piss-drinking</loc></url>
</urlset>`

const detailFixture = `<!DOCTYPE html><html><head>
<meta property="og:type" content="article" />
<meta property="og:site_name" content="PissPlay" />
<meta property="og:title" content="Non-stop Piss Drinking - Bruce and Morgan" />
<meta property="og:description" content="I need to step up my swallowing speed, and I mean now. I&#8217;ve been training." />
<meta property="og:url" content="https://pissplay.com/videos/non-stop-piss-drinking" />
<meta property="og:image" content="https://pissplay.com/wp-content/uploads/464-Thumb-Non-Stop-Piss-Drinking.jpg" />
<script type="application/ld+json">{"@context":"https://schema.org","@graph":[{"@type":"WebPage","datePublished":"2026-06-26","dateModified":"2026-06-26"}]}</script>
</head><body>
<div class="video_date"><svg class="icon_calendar" viewBox="0 0 24 24"><path d="M0 0h24v24H0z"/></svg> 26 Jun 2026</div>
</body></html>`

// detailNoJSONLD exercises the video_date fallback path.
const detailNoJSONLD = `<html><head>
<meta property="og:title" content="Fallback Title" />
<meta property="og:url" content="https://pissplay.com/videos/fallback" />
</head><body>
<div class="video_date"><svg viewBox="0 0 24 24"><path d="x"/></svg> 6 Sep 2024</div>
</body></html>`

func TestParseSitemap(t *testing.T) {
	slugs := parseSitemap([]byte(sitemapFixture))
	if len(slugs) != 2 {
		t.Fatalf("expected 2 unique scene slugs, got %d: %v", len(slugs), slugs)
	}
	if slugs[0] != "non-stop-piss-drinking" {
		t.Errorf("slug[0] = %q, want non-stop-piss-drinking", slugs[0])
	}
	if slugs[1] != "folsom-fair-dp-afterparty" {
		t.Errorf("slug[1] = %q, want folsom-fair-dp-afterparty", slugs[1])
	}
}

func TestParseDetail(t *testing.T) {
	d := parseDetail([]byte(detailFixture))
	if d.title != "Non-stop Piss Drinking - Bruce and Morgan" {
		t.Errorf("title = %q", d.title)
	}
	if d.description != "I need to step up my swallowing speed, and I mean now. I’ve been training." {
		t.Errorf("description = %q", d.description)
	}
	if d.thumbnail != "https://pissplay.com/wp-content/uploads/464-Thumb-Non-Stop-Piss-Drinking.jpg" {
		t.Errorf("thumbnail = %q", d.thumbnail)
	}
	if d.url != "https://pissplay.com/videos/non-stop-piss-drinking" {
		t.Errorf("url = %q", d.url)
	}
	if d.date.Format("2006-01-02") != "2026-06-26" {
		t.Errorf("date = %v, want 2026-06-26", d.date)
	}
}

func TestParseDetailDateFallback(t *testing.T) {
	d := parseDetail([]byte(detailNoJSONLD))
	if d.title != "Fallback Title" {
		t.Errorf("title = %q", d.title)
	}
	if d.date.Format("2006-01-02") != "2024-09-06" {
		t.Errorf("fallback date = %v, want 2024-09-06", d.date)
	}
}

func TestSlugToTitle(t *testing.T) {
	if got := slugToTitle("non-stop-piss-drinking"); got != "non stop piss drinking" {
		t.Errorf("slugToTitle = %q", got)
	}
}

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := map[string]bool{
		"https://pissplay.com/":                      true,
		"https://www.pissplay.com/videos/piss-angel": true,
		"http://pissplay.com/videos/non-stop":        true,
		"https://example.com/pissplay":               false,
		"https://notpissplay.com/":                   false,
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
// fetchPage and the sitemap walk at 0% and the package at 30.9%. The base is now
// a field, so the whole sitemap -> detail walk runs offline.
func TestListScenesEndToEnd(t *testing.T) {
	// Handlers run on separate goroutines (Workers > 1), so these counters
	// must be atomic — a plain int++ here is a real data race.
	var detailHits atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/sitemap.xml":
			// Served verbatim: sitemapLocRe only matches <loc> entries on the real
			// pissplay.com host, and it yields *slugs* only. Detail URLs are then
			// built from s.base, so they still land on this server. Rewriting the
			// host here would make the regex match nothing.
			_, _ = fmt.Fprint(w, sitemapFixture)
		case strings.HasPrefix(r.URL.Path, "/videos/"):
			detailHits.Add(1)
			// Substitute the requested slug so each scene gets its own og:url and
			// title. Serving the fixture verbatim would give every scene the same
			// URL — the shared-fixture trap that hid duplicate data elsewhere.
			slug := strings.TrimPrefix(r.URL.Path, "/videos/")
			_, _ = fmt.Fprint(w, strings.ReplaceAll(detailFixture, "non-stop-piss-drinking", slug))
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
	// The sitemap lists three /videos/ entries, one of them a duplicate.
	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2 (duplicate <loc> deduped)", len(scenes))
	}
	if detailHits.Load() != 2 {
		t.Errorf("detail fetches = %d, want 2", detailHits.Load())
	}
	// Scene.URL comes from the detail page's og:url, so it is fixture data and
	// legitimately names the live host — it is not the URL that was fetched.
	seenURL := map[string]bool{}
	for _, sc := range scenes {
		if sc.Title == "" {
			t.Errorf("scene %q has empty Title", sc.ID)
		}
		if sc.Date.IsZero() {
			t.Errorf("scene %q has zero Date", sc.ID)
		}
		if seenURL[sc.URL] {
			t.Errorf("scene %q repeats URL %q — each detail page must yield its own", sc.ID, sc.URL)
		}
		seenURL[sc.URL] = true
	}
}
