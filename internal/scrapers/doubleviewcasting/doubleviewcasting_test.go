package doubleviewcasting

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
<ul class="main-thumbs title-two-lines">
  <li>
    <a class="thumb" href="/scene/id/353" title="Carol Vega is slammed hard">
      <span class="title">Carol Vega is slammed hard <span class="date">Added: February 28, 2014</span></span>
      <img src="/contents/scenes/353/thumbnails/295x241.jpg" width="295" height="241" alt="Carol Vega is slammed hard" />
    </a>
  </li>
  <li>
    <a class="thumb" href="/scene/id/325" title="Foxy Di fulfills anal wish">
      <span class="title">Foxy Di fulfills anal wish <span class="date">Added: January 15, 2014</span></span>
      <img src="/contents/scenes/325/thumbnails/295x241.jpg" alt="Foxy Di" />
    </a>
  </li>
</ul>
</body></html>`
}

func detailHTML() string {
	return `<html><body>
<div class="info-description"><p>Today we introduce you a tall and slender babe.</p></div>
<ul class="scene-info-bottom">
  <li class="duration"><span>Duration:</span> 00:35:38</li>
  <li class="duration"><span>Times Viewed:</span> 125773</li>
  <li class="models"><span>Girls: </span> <a href="/model/id/181">Carol Vega</a> </li>
  <li class="tags"><span>Tags: </span> <a href="/scenes/tag/4">Hardcore</a>, <a href="/scenes/tag/5">Anal</a> </li>
</ul>
</body></html>`
}

// ---- TestMatchesURL ----

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url   string
		match bool
	}{
		{"http://doubleviewcasting.com/scenes", true},
		{"https://www.doubleviewcasting.com/scene/id/353", true},
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
	items, err := s.fetchListing(context.Background(), ts.URL+"/scenes/page/1")
	if err != nil {
		t.Fatalf("fetchListing error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2: %+v", len(items), items)
	}
	if items[0].id != "353" || items[0].title != "Carol Vega is slammed hard" {
		t.Errorf("item0 = %+v", items[0])
	}
	if items[0].url != siteBase+"/scene/id/353" {
		t.Errorf("item0.url = %q", items[0].url)
	}
	if items[0].date != "February 28, 2014" {
		t.Errorf("item0.date = %q", items[0].date)
	}
	if !strings.Contains(items[0].thumb, "/contents/scenes/353/") {
		t.Errorf("item0.thumb = %q", items[0].thumb)
	}
}

// ---- TestToScene (detail parse) ----

func TestToScene(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, detailHTML())
	}))
	defer ts.Close()

	s := &Scraper{Client: ts.Client()}
	it := listItem{id: "353", url: ts.URL + "/scene/id/353", title: "Carol Vega is slammed hard", date: "February 28, 2014", thumb: "http://x/t.jpg"}
	now := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	sc := s.toScene(context.Background(), "studioURL", it, now)

	if sc.ID != "353" || sc.SiteID != siteID || sc.Studio != studioName {
		t.Errorf("identity wrong: %+v", sc)
	}
	if sc.Title != "Carol Vega is slammed hard" {
		t.Errorf("Title = %q", sc.Title)
	}
	wantDate := time.Date(2014, 2, 28, 0, 0, 0, 0, time.UTC)
	if !sc.Date.Equal(wantDate) {
		t.Errorf("Date = %v, want %v", sc.Date, wantDate)
	}
	if sc.Duration != 35*60+38 {
		t.Errorf("Duration = %d, want 2138", sc.Duration)
	}
	if !strings.HasPrefix(sc.Description, "Today we introduce") {
		t.Errorf("Description = %q", sc.Description)
	}
	if strings.Join(sc.Performers, ",") != "Carol Vega" {
		t.Errorf("Performers = %v", sc.Performers)
	}
	if strings.Join(sc.Tags, ",") != "Hardcore,Anal" {
		t.Errorf("Tags = %v", sc.Tags)
	}
}

// ---- TestListScenes (end-to-end) ----

func TestListScenes(t *testing.T) {
	orig := siteBase
	defer func() { siteBase = orig }()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/scenes/page/1":
			_, _ = fmt.Fprint(w, listingHTML())
		case strings.HasPrefix(r.URL.Path, "/scene/id/"):
			_, _ = fmt.Fprint(w, detailHTML())
		default:
			// page 2+ are empty -> Paginate stops.
			_, _ = fmt.Fprint(w, "<html><body></body></html>")
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
	if got["353"] != "Carol Vega is slammed hard" || got["325"] != "Foxy Di fulfills anal wish" {
		t.Errorf("scenes = %v", got)
	}
}
