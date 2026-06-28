package perfectgonzoutil

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/scraper"
)

// listingHTML mirrors the real Perfect Gonzo /movies card markup (two cards).
const listingHTML = `<html><body>
<div id="content-main" class="bloc l-bloc ">
<!-- start_link -->
<div class="itemm" data-id="3947" data-fileid="1974">
  <a class="bloc-link shuffle-me si-container " href="/movies/tina-kay/" title='Tina Kay'>
    <img alt="0%" src="https://media2.perfectgonzo.com/content/movies/tina-kay/perfectgonzo-new-tour/cover-hover-pic.jpg" class="active">
    <img alt="20%" src="https://media2.perfectgonzo.com/content/movies/tina-kay/perfectgonzo-new-tour/hover-2.jpg" class="">
  </a>
  <h5 class="mg-md clearfix">
    <span class="nm-date">06/19/2026</span>
    <span class="nm-name truncate" style="word-wrap: break-word;">Tina Kay</span>
    <span class="nm-opts"><a class="mn-rating">4.79</a></span>
  </h5>
  <ul class="dropdown-menu">
    <li><p>Length: 40:44, 70 pics</p></li>
  </ul>
</div>

<div class="itemm" data-id="3946" data-fileid="1973">
  <a class="bloc-link shuffle-me si-container " href="/movies/jane-doe/" title='Jane &amp; Friends'>
    <img alt="0%" src="https://media2.perfectgonzo.com/content/movies/jane-doe/perfectgonzo-new-tour/cover-hover-pic.jpg" class="active">
  </a>
  <h5 class="mg-md clearfix">
    <span class="nm-date">06/12/2026</span>
    <span class="nm-name truncate">Jane &amp; Friends</span>
  </h5>
  <ul class="dropdown-menu">
    <li><p>Length: 1:02:05, 80 pics</p></li>
  </ul>
</div>
</div>
</body></html>`

const tinaDetailHTML = `<html><head><title>Tina Kay - All Internal</title></head><body>
<p class="mg-md">British anal superstar Tina Kay is a favorite. &amp; more.</p>
<div class="model-block">
  <h4>Featured model(s):</h4>
  <p><a href='/models/tina-kay/'>Tina Kay</a></p>
</div>
<div id="video-info2">
  <h4>Tags:</h4>
  Art &amp; Addons
  <a href='/movies?tag[]=piercing'>piercing</a>
  Body Type
  <a href='/movies?tag[]=athletic'>athletic</a>
</div>
</body></html>`

const janeDetailHTML = `<html><head><title>Jane &amp; Friends - All Internal</title></head><body>
<p class="mg-md">A group scene.</p>
<div class="model-block">
  <h4>Featured model(s):</h4>
  <p><a href='/models/jane-doe/'>Jane Doe</a> <a href='/models/john-roe/'>John Roe</a></p>
</div>
<div id="video-info2">
  <h4>Tags:</h4>
  <a href='/movies?tag[]=group'>group</a>
</div>
</body></html>`

const emptyHTML = `<html><body><div id="content-main"></div></body></html>`

func testConfig(base string) SiteConfig {
	return SiteConfig{
		ID:       "allinternal",
		SiteBase: base,
		Studio:   "All Internal",
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?allinternal\.com`),
	}
}

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		switch r.URL.Path {
		case "/movies":
			_, _ = fmt.Fprint(w, listingHTML)
		case "/movies/tina-kay/":
			_, _ = fmt.Fprint(w, tinaDetailHTML)
		case "/movies/jane-doe/":
			_, _ = fmt.Fprint(w, janeDetailHTML)
		default:
			_, _ = fmt.Fprint(w, emptyHTML)
		}
	}))
}

func TestParseListing(t *testing.T) {
	cards := parseListing([]byte(listingHTML))
	if len(cards) != 2 {
		t.Fatalf("got %d cards, want 2", len(cards))
	}
	first := cards[0]
	if first.id != "3947" {
		t.Errorf("id = %q, want 3947", first.id)
	}
	if first.slug != "tina-kay" {
		t.Errorf("slug = %q, want tina-kay", first.slug)
	}
	if first.title != "Tina Kay" {
		t.Errorf("title = %q", first.title)
	}
	if first.date != "06/19/2026" {
		t.Errorf("date = %q", first.date)
	}
	if first.length != "40:44" {
		t.Errorf("length = %q, want 40:44", first.length)
	}
	if first.thumb != "https://media2.perfectgonzo.com/content/movies/tina-kay/perfectgonzo-new-tour/cover-hover-pic.jpg" {
		t.Errorf("thumb = %q", first.thumb)
	}
	if cards[1].title != "Jane & Friends" {
		t.Errorf("second title = %q, want decoded ampersand", cards[1].title)
	}
}

func TestApplyDetail(t *testing.T) {
	s := New(testConfig("http://www.allinternal.com"))
	scene := models.Scene{ID: "tina-kay", Title: "Tina Kay"}
	s.applyDetail(&scene, tinaDetailHTML)
	if scene.Description != "British anal superstar Tina Kay is a favorite. & more." {
		t.Errorf("desc = %q", scene.Description)
	}
	if len(scene.Performers) != 1 || scene.Performers[0] != "Tina Kay" {
		t.Errorf("performers = %v", scene.Performers)
	}
	wantTags := []string{"piercing", "athletic"}
	if len(scene.Tags) != len(wantTags) {
		t.Fatalf("tags = %v, want %v", scene.Tags, wantTags)
	}
	for i, tg := range wantTags {
		if scene.Tags[i] != tg {
			t.Errorf("tags[%d] = %q, want %q", i, scene.Tags[i], tg)
		}
	}
}

func TestListScenes_endToEnd(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	s := New(testConfig(ts.URL))
	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}

	byID := map[string]scraper.SceneResult{}
	for r := range ch {
		switch r.Kind {
		case scraper.KindScene:
			byID[r.Scene.ID] = r
		case scraper.KindError:
			t.Errorf("unexpected error: %v", r.Err)
		}
	}
	if len(byID) != 2 {
		t.Fatalf("got %d scenes, want 2", len(byID))
	}

	tina, ok := byID["tina-kay"]
	if !ok {
		t.Fatal("missing scene tina-kay")
	}
	sc := tina.Scene
	if sc.SiteID != "allinternal" {
		t.Errorf("SiteID = %q", sc.SiteID)
	}
	if sc.Title != "Tina Kay" {
		t.Errorf("Title = %q", sc.Title)
	}
	if sc.URL != ts.URL+"/movies/tina-kay/" {
		t.Errorf("URL = %q", sc.URL)
	}
	if sc.Date.Year() != 2026 || sc.Date.Month() != 6 || sc.Date.Day() != 19 {
		t.Errorf("Date = %v, want 2026-06-19", sc.Date)
	}
	if sc.Duration != 2444 {
		t.Errorf("Duration = %d, want 2444", sc.Duration)
	}
	if sc.Thumbnail != "https://media2.perfectgonzo.com/content/movies/tina-kay/perfectgonzo-new-tour/cover-hover-pic.jpg" {
		t.Errorf("Thumbnail = %q", sc.Thumbnail)
	}
	if sc.Studio != "All Internal" {
		t.Errorf("Studio = %q", sc.Studio)
	}
	if len(sc.Performers) != 1 || sc.Performers[0] != "Tina Kay" {
		t.Errorf("Performers = %v", sc.Performers)
	}
	if len(sc.Tags) != 2 {
		t.Errorf("Tags = %v, want 2", sc.Tags)
	}

	jane := byID["jane-doe"].Scene
	if jane.Duration != 3725 {
		t.Errorf("jane Duration = %d, want 3725", jane.Duration)
	}
	if len(jane.Performers) != 2 {
		t.Errorf("jane Performers = %v, want 2", jane.Performers)
	}
}

func TestListScenes_knownIDsStopsEarly(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	s := New(testConfig(ts.URL))
	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{
		KnownIDs: map[string]bool{"jane-doe": true},
	})
	if err != nil {
		t.Fatal(err)
	}
	var (
		scenes       int
		stoppedEarly bool
	)
	for r := range ch {
		switch r.Kind {
		case scraper.KindScene:
			scenes++
		case scraper.KindStoppedEarly:
			stoppedEarly = true
		}
	}
	if scenes != 1 {
		t.Errorf("got %d scenes, want 1", scenes)
	}
	if !stoppedEarly {
		t.Error("expected StoppedEarly")
	}
}

func TestMatchesURL(t *testing.T) {
	s := New(testConfig("http://www.allinternal.com"))
	cases := []struct {
		url   string
		match bool
	}{
		{"http://www.allinternal.com/movies", true},
		{"https://allinternal.com/movies/tina-kay/", true},
		{"https://example.com/", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.match {
			t.Errorf("MatchesURL(%q) = %v", c.url, got)
		}
	}
}

func TestListingURL(t *testing.T) {
	s := New(testConfig("http://www.allinternal.com"))
	if got := s.listingURL(1); got != "http://www.allinternal.com/movies" {
		t.Errorf("page 1 = %q", got)
	}
	if got := s.listingURL(3); got != "http://www.allinternal.com/movies/page-3/" {
		t.Errorf("page 3 = %q", got)
	}
}

func TestCleanText(t *testing.T) {
	cases := []struct{ in, want string }{
		{"<b>Hi</b> &amp; <i>there</i>  friend", "Hi & there friend"},
		{"  spaced   out  ", "spaced out"},
	}
	for _, c := range cases {
		if got := cleanText(c.in); got != c.want {
			t.Errorf("cleanText(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
