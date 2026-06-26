package timtales

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wasylq/FSS/scraper"
)

// ---- fixtures ----

func listingHTML() string {
	return `<html><body>
<a href="/videos/category/">Categories</a>
<div class="video-item video">
  <h2>Hot Scene &amp; More</h2>
  <div class="player" style="background-image: url(&#39;https://cdn.timtales.com/splash1.jpg&#39;)"></div>
  <a href="/videos/hot-scene/">watch</a>
</div>
<div class="video-item video">
  <h2>Second One</h2>
  <div class="player" style="background-image: url(&#039;https://cdn.timtales.com/splash2.jpg&#039;)"></div>
  <a href="/videos/second-one/">watch</a>
</div>
</body></html>`
}

func detailHTML(title, date, runtime, desc string) string {
	return fmt.Sprintf(`<html><body>
<h1>%s</h1>
<p class="date"> %s &#8211; Runtime: %s </p>
<p class="bodytext">%s</p>
</body></html>`, title, date, runtime, desc)
}

// ---- TestMatchesURL ----

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url   string
		match bool
	}{
		{"https://www.timtales.com/videos/latest/", true},
		{"https://timtales.com/videos/foo/", true},
		{"https://example.com/x", false},
		{"", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.match {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.match)
		}
	}
}

// ---- TestCleanText ----

func TestCleanText(t *testing.T) {
	if got := cleanText(`  <b>A&amp;B</b>  c  `); got != "A&B c" {
		t.Errorf("cleanText = %q, want %q", got, "A&B c")
	}
}

// ---- TestFetchListing ----

func TestFetchListing(t *testing.T) {
	orig := baseURL
	defer func() { baseURL = orig }()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, listingHTML())
	}))
	defer ts.Close()
	baseURL = ts.URL

	s := &Scraper{client: ts.Client()}
	items, err := s.fetchListing(context.Background(), ts.URL+"/videos/latest/")
	if err != nil {
		t.Fatalf("fetchListing error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2: %+v", len(items), items)
	}
	if items[0].id != "hot-scene" || items[0].title != "Hot Scene & More" {
		t.Errorf("item0 = %+v", items[0])
	}
	if items[0].thumbnail != "https://cdn.timtales.com/splash1.jpg" {
		t.Errorf("item0.thumbnail = %q", items[0].thumbnail)
	}
	if items[0].url != ts.URL+"/videos/hot-scene/" {
		t.Errorf("item0.url = %q", items[0].url)
	}
	if items[1].id != "second-one" || items[1].thumbnail != "https://cdn.timtales.com/splash2.jpg" {
		t.Errorf("item1 = %+v", items[1])
	}
}

// ---- TestToScene (detail parse) ----

func TestToScene(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, detailHTML("Hot Scene Full Title", "June 1, 2024", "24:13", "A great description here."))
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client()}
	it := listItem{id: "hot-scene", url: ts.URL + "/videos/hot-scene/", title: "Hot Scene", thumbnail: "thumb.jpg"}
	now := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	sc := s.toScene(context.Background(), "studioURL", it, now)

	if sc.ID != "hot-scene" {
		t.Errorf("ID = %q", sc.ID)
	}
	if sc.SiteID != siteID {
		t.Errorf("SiteID = %q", sc.SiteID)
	}
	if sc.Title != "Hot Scene Full Title" {
		t.Errorf("Title = %q (h1 should override listing title)", sc.Title)
	}
	if sc.URL != ts.URL+"/videos/hot-scene/" {
		t.Errorf("URL = %q", sc.URL)
	}
	wantDate := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	if !sc.Date.Equal(wantDate) {
		t.Errorf("Date = %v, want %v", sc.Date, wantDate)
	}
	if sc.Duration != 1453 {
		t.Errorf("Duration = %d, want 1453 (24:13)", sc.Duration)
	}
	if sc.Description != "A great description here." {
		t.Errorf("Description = %q", sc.Description)
	}
}

// ---- TestListScenes (end-to-end) ----

func TestListScenes(t *testing.T) {
	orig := baseURL
	defer func() { baseURL = orig }()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/videos/latest/":
			_, _ = fmt.Fprint(w, listingHTML())
		case "/videos/hot-scene/":
			_, _ = fmt.Fprint(w, detailHTML("Hot Scene Full Title", "June 1, 2024", "24:13", "Desc one."))
		case "/videos/second-one/":
			_, _ = fmt.Fprint(w, detailHTML("Second One Full", "July 2, 2024", "18:00", "Desc two."))
		default:
			// page-2 etc. repeats the listing -> dedup empties -> Done.
			_, _ = fmt.Fprint(w, listingHTML())
		}
	}))
	defer ts.Close()
	baseURL = ts.URL

	s := &Scraper{client: ts.Client()}
	// Empty studioURL -> defaults to baseURL + "/videos/latest".
	ch, err := s.ListScenes(context.Background(), "", scraper.ListOpts{})
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
	if got["hot-scene"] != "Hot Scene Full Title" || got["second-one"] != "Second One Full" {
		t.Errorf("scenes = %v", got)
	}
}
