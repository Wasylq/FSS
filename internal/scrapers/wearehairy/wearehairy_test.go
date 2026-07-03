package wearehairy

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

func card(id, model, slug, thumb string) string {
	return fmt.Sprintf(`<div class="_results_item _results_posts_item col">
<div class="position-relative" data-post-id="%s">
<a class="d-block" href="/models/%s/%s" title="%s">
<img class="img-fluid" src="/media/no_video_cover.png" alt="%s">
<img class="_image h-100 fit-cover" src="%s" alt="%s" />
</a></div></div>`, id, slug, slug, model, model, thumb, model)
}

func listingHTML(cards ...string) string {
	return `<html><body>
<div class="_results_item _results_tags_item ">ignored tag</div>
<div class="row">` + strings.Join(cards, "\n") + `</div></body></html>`
}

// ---- TestMatchesURL ----

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url   string
		match bool
	}{
		{"http://wearehairy.com/categories/Photos?page=1", true},
		{"https://www.wearehairy.com/", true},
		{"https://example.com/x", false},
		{"", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.match {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.match)
		}
	}
}

// ---- TestToScene ----

func TestToScene(t *testing.T) {
	orig := siteBase
	defer func() { siteBase = orig }()
	siteBase = "https://wearehairy.com"

	c := card("22238", "Neysi L", "neysi-l-1", "https://cdn.example/cover.jpeg")
	now := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	sc, ok := toScene("studioURL", c, now)
	if !ok {
		t.Fatal("toScene returned ok=false")
	}
	if sc.ID != "22238" || sc.SiteID != "wearehairy" {
		t.Errorf("identity = %q/%q", sc.ID, sc.SiteID)
	}
	if sc.Title != "Neysi L" {
		t.Errorf("Title = %q", sc.Title)
	}
	if len(sc.Performers) != 1 || sc.Performers[0] != "Neysi L" {
		t.Errorf("Performers = %v", sc.Performers)
	}
	if sc.URL != "https://wearehairy.com/post/details/22238" {
		t.Errorf("URL = %q", sc.URL)
	}
	if sc.Thumbnail != "https://cdn.example/cover.jpeg" {
		t.Errorf("Thumbnail = %q", sc.Thumbnail)
	}
	if sc.Studio != "We Are Hairy" {
		t.Errorf("Studio = %q", sc.Studio)
	}
}

// ---- TestFetchListing ----

func TestFetchListing(t *testing.T) {
	orig := siteBase
	defer func() { siteBase = orig }()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, listingHTML(
			card("1", "Alice", "alice-1", "https://cdn/a.jpeg"),
			card("2", "Bob", "bob-2", "https://cdn/b.jpeg"),
		))
	}))
	defer ts.Close()
	siteBase = ts.URL

	s := &Scraper{Client: ts.Client()}
	cards, err := s.fetchListing(context.Background(), 1)
	if err != nil {
		t.Fatalf("fetchListing error: %v", err)
	}
	if len(cards) != 2 {
		t.Fatalf("got %d cards, want 2", len(cards))
	}
}

// ---- TestListScenes (end-to-end + pagination stop on empty page) ----

func TestListScenes(t *testing.T) {
	orig := siteBase
	defer func() { siteBase = orig }()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("page") {
		case "1":
			_, _ = fmt.Fprint(w, listingHTML(
				card("10", "Alice", "alice-1", "https://cdn/a.jpeg"),
				card("11", "Bob", "bob-2", "https://cdn/b.jpeg"),
			))
		default:
			// page 2: no cards -> Paginate stops.
			_, _ = fmt.Fprint(w, `<html><body>no items</body></html>`)
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
		if r.Kind == scraper.KindScene {
			got[r.Scene.ID] = r.Scene.Title
		}
	}
	if len(got) != 2 {
		t.Fatalf("got %d scenes, want 2: %v", len(got), got)
	}
	if got["10"] != "Alice" || got["11"] != "Bob" {
		t.Errorf("scenes = %v", got)
	}
}
