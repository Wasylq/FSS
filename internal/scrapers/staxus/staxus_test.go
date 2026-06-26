package staxus

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wasylq/FSS/scraper"
)

// cardBody is the content of a single listing card AFTER the
// `<div class="update_details" data-setid="N"` split marker is stripped.
const cardBody = `>
  <div class="thumb" style="background-image: url(/trial/content/1234/thumb.jpg)"></div>
  <a class="title_bar_movie" href="/trial/gallery.php?id=1234&type=vids"><span itemprop="name">Twink Adventure &amp; Fun</span></a>
  <span>18 Jun 2026</span>
  <div class="update_models">
    <a href="/trial/models/johnny.html"><span itemprop="name">Johnny Doe</span></a>
    <a href="/trial/models/mike.html"><span itemprop="name">Mike Smith</span></a>
    <a href="/trial/models/mike.html"><span itemprop="name">Mike Smith</span></a>
  </div>
</div>`

// fullCard wraps cardBody with the split marker so it can be fed to fetchCards.
func fullCard(setID, galleryID, title, date string) string {
	return fmt.Sprintf(`<div class="update_details" data-setid="%s">
  <div class="thumb" style="background-image: url(/trial/content/%s/thumb.jpg)"></div>
  <a class="title_bar_movie" href="/trial/gallery.php?id=%s&type=vids"><span itemprop="name">%s</span></a>
  <span>%s</span>
  <div class="update_models">
    <a href="#"><span itemprop="name">Johnny Doe</span></a>
  </div>
</div>`, setID, galleryID, galleryID, title, date)
}

func listingHTML(cards ...string) string {
	body := `<html><body><div class="listing">`
	for _, c := range cards {
		body += c
	}
	return body + `</div></body></html>`
}

func TestToScene(t *testing.T) {
	now := time.Date(2026, 6, 26, 12, 0, 0, 0, time.UTC)
	scene, ok := toScene("https://staxus.com/trial/category.php?id=50", cardBody, now)
	if !ok {
		t.Fatal("toScene returned ok=false")
	}
	if scene.ID != "1234" {
		t.Errorf("ID = %q, want 1234", scene.ID)
	}
	if scene.SiteID != "staxus" {
		t.Errorf("SiteID = %q", scene.SiteID)
	}
	if scene.Title != "Twink Adventure & Fun" {
		t.Errorf("Title = %q", scene.Title)
	}
	if scene.URL != "https://staxus.com/trial/gallery.php?id=1234&type=vids" {
		t.Errorf("URL = %q", scene.URL)
	}
	if scene.Studio != "Staxus" {
		t.Errorf("Studio = %q", scene.Studio)
	}
	if scene.Thumbnail != "https://staxus.com/trial/content/1234/thumb.jpg" {
		t.Errorf("Thumbnail = %q", scene.Thumbnail)
	}
	want := time.Date(2026, 6, 18, 0, 0, 0, 0, time.UTC)
	if !scene.Date.Equal(want) {
		t.Errorf("Date = %v, want %v", scene.Date, want)
	}
	if len(scene.Performers) != 2 {
		t.Fatalf("Performers = %v, want 2 (deduped)", scene.Performers)
	}
	if scene.Performers[0] != "Johnny Doe" || scene.Performers[1] != "Mike Smith" {
		t.Errorf("Performers = %v", scene.Performers)
	}
	if !scene.ScrapedAt.Equal(now) {
		t.Errorf("ScrapedAt = %v, want %v", scene.ScrapedAt, now)
	}
}

func TestToSceneSkipsNonVideo(t *testing.T) {
	// A photo-set card (type=highres) has no title_bar_movie type=vids href.
	card := `>
  <a class="title_bar_movie" href="/trial/gallery.php?id=99&type=highres"><span itemprop="name">Photo Set</span></a>`
	if _, ok := toScene("studio", card, time.Now()); ok {
		t.Error("expected ok=false for non-video card")
	}
}

func TestFetchCards(t *testing.T) {
	html := listingHTML(
		fullCard("1234", "1234", "First", "18 Jun 2026"),
		fullCard("5678", "5678", "Second", "10 May 2026"),
	)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, html)
	}))
	defer ts.Close()

	s := New()
	s.client = ts.Client()
	cards, err := s.fetchCards(context.Background(), ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 2 {
		t.Fatalf("got %d cards, want 2", len(cards))
	}
}

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url   string
		match bool
	}{
		{"https://staxus.com/trial/category.php?id=50", true},
		{"http://www.staxus.com/", true},
		{"https://example.com/", false},
		{"", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.match {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.match)
		}
	}
}

func TestID(t *testing.T) {
	if New().ID() != "staxus" {
		t.Errorf("ID = %q", New().ID())
	}
}

func TestListScenesEndToEnd(t *testing.T) {
	orig := siteBase
	defer func() { siteBase = orig }()

	page1 := listingHTML(
		fullCard("1234", "1234", "First", "18 Jun 2026"),
		fullCard("5678", "5678", "Second", "10 May 2026"),
	)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/trial/category.php":
			if r.URL.Query().Get("page") == "1" {
				_, _ = fmt.Fprint(w, page1)
				return
			}
			_, _ = fmt.Fprint(w, listingHTML())
		default:
			_, _ = fmt.Fprint(w, listingHTML())
		}
	}))
	defer ts.Close()
	siteBase = ts.URL

	s := New()
	s.client = ts.Client()
	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}
	var scenes int
	for r := range ch {
		switch r.Kind {
		case scraper.KindScene:
			scenes++
		case scraper.KindError:
			t.Errorf("unexpected error: %v", r.Err)
		}
	}
	if scenes != 2 {
		t.Errorf("got %d scenes, want 2", scenes)
	}
}

func TestListScenesKnownIDsStopsEarly(t *testing.T) {
	orig := siteBase
	defer func() { siteBase = orig }()

	page1 := listingHTML(
		fullCard("1234", "1234", "First", "18 Jun 2026"),
		fullCard("5678", "5678", "Second", "10 May 2026"),
	)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, page1)
	}))
	defer ts.Close()
	siteBase = ts.URL

	s := New()
	s.client = ts.Client()
	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{
		KnownIDs: map[string]bool{"5678": true},
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
		t.Error("expected StoppedEarly signal")
	}
}
