package nudolls

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

// ---- fixtures ----

func listingHTML() string {
	return `<html><body>
<div class="grid">
  <div class="item"><a href="/kira-chaotically-video-1265.html" class="cover"><img src="/pub/content/1265/medium.jpg" alt="Chaotically"></a></div>
  <div class="item"><a href="/-apple-video-1135.html" class="cover"><img src="/pub/content/1135/medium.jpg" alt="Apple"></a></div>
  <div class="item"><a href="/kira-chaotically-video-1265.html" class="cover">dup</a></div>
</div>
</body></html>`
}

func detailHTML(title, modelSlug, modelID, modelName, coverPath string) string {
	return fmt.Sprintf(`<html><head><title>ND — Video — %s</title></head><body>
<ol class="breadcrumb"><li><a href="./">Home</a></li><li><a href="./videos.html">Videos</a></li><li><a href="/%s-model-%s.html" title="NuDolls %s">%s</a></li><li>%s</li></ol>
<article class="view view_cover"><div class="header"><h1>%s</h1></div>
<div class="cover"><a href="/join.html"><img src="%s" alt="%s"></a></div></article>
<div class="grid grid2"><div class="item"><a href="/anya-other-video-1277.html" class="cover"><img src="/pub/content/1277/medium.jpg" alt="Other"></a></div></div>
</body></html>`, title, modelSlug, modelID, modelName, modelName, title, title, coverPath, modelName)
}

// ---- TestMatchesURL ----

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url   string
		match bool
	}{
		{"https://nudolls.com/videos.html", true},
		{"https://www.nudolls.com/kira-chaotically-video-1265.html", true},
		{"https://example.com/x", false},
		{"", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.match {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.match)
		}
	}
}

// ---- TestFetchListing ----

func TestFetchListing(t *testing.T) {
	orig := siteBase
	defer func() { siteBase = orig }()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, listingHTML())
	}))
	defer ts.Close()
	siteBase = ts.URL

	s := &Scraper{Client: ts.Client()}
	items, err := s.fetchListing(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("fetchListing error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2 (dup dropped): %+v", len(items), items)
	}
	if items[0].id != "1265" {
		t.Errorf("item0.id = %q, want 1265", items[0].id)
	}
	if items[0].url != siteBase+"/kira-chaotically-video-1265.html" {
		t.Errorf("item0.url = %q", items[0].url)
	}
	if items[1].id != "1135" {
		t.Errorf("item1.id = %q, want 1135", items[1].id)
	}
}

// ---- TestToScene (detail parse) ----

func TestToScene(t *testing.T) {
	orig := siteBase
	defer func() { siteBase = orig }()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, detailHTML("Chaotically", "kira", "104", "Kira", "/pub/content/1265/chaotically.jpg"))
	}))
	defer ts.Close()
	siteBase = ts.URL

	s := &Scraper{Client: ts.Client()}
	it := listItem{id: "1265", url: ts.URL + "/kira-chaotically-video-1265.html"}
	now := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	sc := s.toScene(context.Background(), "studioURL", it, now)

	if sc.ID != "1265" || sc.SiteID != siteID {
		t.Errorf("identity wrong: %+v", sc)
	}
	if sc.Title != "Chaotically" {
		t.Errorf("Title = %q, want Chaotically", sc.Title)
	}
	if len(sc.Performers) != 1 || sc.Performers[0] != "Kira" {
		t.Errorf("Performers = %v, want [Kira]", sc.Performers)
	}
	if sc.Thumbnail != siteBase+"/pub/content/1265/chaotically.jpg" {
		t.Errorf("Thumbnail = %q", sc.Thumbnail)
	}
	if sc.Studio != studioName {
		t.Errorf("Studio = %q", sc.Studio)
	}
}

// ---- TestToSceneTitleFallback (no <h1>, title tag used) ----

func TestToSceneTitleFallback(t *testing.T) {
	orig := siteBase
	defer func() { siteBase = orig }()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `<html><head><title>ND — Video — Honeyed Dreams</title></head><body>
<a href="/tania-model-99.html" title="NuDolls Tania">Tania</a>
</body></html>`)
	}))
	defer ts.Close()
	siteBase = ts.URL

	s := &Scraper{Client: ts.Client()}
	it := listItem{id: "777", url: ts.URL + "/tania-honeyed-dreams-video-777.html"}
	sc := s.toScene(context.Background(), "studioURL", it, time.Now().UTC())
	if sc.Title != "Honeyed Dreams" {
		t.Errorf("Title = %q, want Honeyed Dreams", sc.Title)
	}
}

// ---- TestListScenes (end-to-end) ----

func TestListScenes(t *testing.T) {
	orig := siteBase
	defer func() { siteBase = orig }()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/videos.html"):
			if r.URL.Query().Get("page") == "1" {
				_, _ = fmt.Fprint(w, listingHTML())
				return
			}
			_, _ = fmt.Fprint(w, `<html><body>no videos here</body></html>`)
		case strings.Contains(r.URL.Path, "video-1265"):
			_, _ = fmt.Fprint(w, detailHTML("Chaotically", "kira", "104", "Kira", "/pub/content/1265/c.jpg"))
		case strings.Contains(r.URL.Path, "video-1135"):
			_, _ = fmt.Fprint(w, detailHTML("Apple", "anya", "105", "Anya", "/pub/content/1135/a.jpg"))
		default:
			_, _ = fmt.Fprint(w, `<html><body>nothing</body></html>`)
		}
	}))
	defer ts.Close()
	siteBase = ts.URL

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
	if got["1265"] != "Chaotically" || got["1135"] != "Apple" {
		t.Errorf("scenes = %v", got)
	}
}
