package jizzonteens

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wasylq/FSS/scraper"
)

const page1HTML = `<html><body>
<article>
  <a href="/content/teen-pool-party/thumbnails/1.jpg">x</a>
  <h2>Teen Pool Party</h2>
  <video poster="/content/teen-pool-party/poster.jpg"></video>
  <textarea>A sunny &amp; wet afternoon.</textarea>
</article>
<article>
  <a href="/content/late-night-study/thumbnails/1.jpg">x</a>
  <h2>Late Night Study</h2>
  <video poster="http://cdn.example.com/late.jpg"></video>
  <textarea>Cramming for finals.</textarea>
</article>
</body></html>`

// page2 repeats page1's last article (site behaviour past the end) plus one new.
const page2HTML = `<html><body>
<article>
  <a href="/content/beach-day/thumbnails/1.jpg">x</a>
  <h2>Beach Day</h2>
  <video poster="/content/beach-day/poster.jpg"></video>
  <textarea>Sand everywhere.</textarea>
</article>
</body></html>`

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			_, _ = w.Write([]byte(page1HTML))
		case "/page/2":
			_, _ = w.Write([]byte(page2HTML))
		default:
			// Pages past the end repeat the last page.
			_, _ = w.Write([]byte(page2HTML))
		}
	}))
}

func TestToScene(t *testing.T) {
	prev := siteBase
	siteBase = "http://jizzonteens.test"
	defer func() { siteBase = prev }()

	article := `
  <a href="/content/teen-pool-party/thumbnails/1.jpg">x</a>
  <h2>Teen &amp; Pool Party</h2>
  <video poster="/content/teen-pool-party/poster.jpg"></video>
  <textarea>A sunny &amp; wet afternoon.</textarea>`

	sc, ok := toScene("http://jizzonteens.test", article, time.Now().UTC())
	if !ok {
		t.Fatal("expected scene, got ok=false")
	}
	if sc.ID != "teen-pool-party" {
		t.Errorf("ID = %q, want teen-pool-party", sc.ID)
	}
	if sc.Title != "Teen & Pool Party" {
		t.Errorf("Title = %q, want unescaped", sc.Title)
	}
	if sc.URL != "http://jizzonteens.test/content/teen-pool-party/" {
		t.Errorf("URL = %q", sc.URL)
	}
	if sc.Thumbnail != "http://jizzonteens.test/content/teen-pool-party/poster.jpg" {
		t.Errorf("Thumbnail = %q", sc.Thumbnail)
	}
	if sc.Description != "A sunny & wet afternoon." {
		t.Errorf("Description = %q", sc.Description)
	}
	if sc.SiteID != "jizzonteens" || sc.Studio != "Jizz on Teens" {
		t.Errorf("SiteID/Studio = %q/%q", sc.SiteID, sc.Studio)
	}
}

func TestToSceneNoSlug(t *testing.T) {
	if _, ok := toScene("u", `<h2>No slug here</h2>`, time.Now().UTC()); ok {
		t.Error("expected ok=false for article without a content slug")
	}
}

func TestAbsURL(t *testing.T) {
	prev := siteBase
	siteBase = "http://jizzonteens.test"
	defer func() { siteBase = prev }()

	if got := absURL("http://cdn/x.jpg"); got != "http://cdn/x.jpg" {
		t.Errorf("absolute passthrough = %q", got)
	}
	if got := absURL("/a/b.jpg"); got != "http://jizzonteens.test/a/b.jpg" {
		t.Errorf("rooted = %q", got)
	}
	if got := absURL("a/b.jpg"); got != "http://jizzonteens.test/a/b.jpg" {
		t.Errorf("bare = %q", got)
	}
}

func TestListScenesPagination(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()
	prev := siteBase
	siteBase = ts.URL
	defer func() { siteBase = prev }()

	s := New()
	s.Client = ts.Client()

	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}

	ids := map[string]bool{}
	for r := range ch {
		if r.Err != nil {
			t.Errorf("unexpected error: %v", r.Err)
			continue
		}
		if r.Kind == scraper.KindScene {
			ids[r.Scene.ID] = true
		}
	}
	for _, want := range []string{"teen-pool-party", "late-night-study", "beach-day"} {
		if !ids[want] {
			t.Errorf("missing scene %q (got %v)", want, ids)
		}
	}
	if len(ids) != 3 {
		t.Errorf("expected 3 unique scenes, got %d: %v", len(ids), ids)
	}
}

func TestMatchesURL(t *testing.T) {
	s := New()
	for _, u := range []string{"https://jizzonteens.com", "http://www.jizzonteens.com/page/2"} {
		if !s.MatchesURL(u) {
			t.Errorf("should match %q", u)
		}
	}
	if s.MatchesURL("https://example.com") {
		t.Error("should not match example.com")
	}
}
