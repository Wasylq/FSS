package bronetworkutil

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

	"github.com/Wasylq/FSS/scraper"
)

func timeFixed() time.Time { return time.Date(2026, 6, 26, 0, 0, 0, 0, time.UTC) }

// Fixtures modelled on the Pinstripe / "Bro Network" PHP CMS listing markup.

const card1 = `<div class="updateDetails">
  <a href="https://menatplay.com/updates/SuitedSeduction.html">
    <video poster="https://menatplay.com/content/contentthumbs/01/23/123-1x.jpg"></video>
  </a>
  <h4>Suited Seduction</h4>
  <span class="tour_update_models"><a href="models/john-doe.html">John Doe</a> <a href="models/jack-roe.html">Jack Roe</a></span>
  <span class="availdate">Jan 23, 2026</span>
  <span class="availdate">25:30 min</span>
</div>`

const card2 = `<div class="updateDetails">
  <a href="https://menatplay.com/updates/LockerRoom.html">
    <img class="lazy" src="https://menatplay.com/content/contentthumbs/04/56/456-1x.jpg" />
  </a>
  <h4>Locker Room &amp; Shower</h4>
  <span class="tour_update_models"><a href="models/mark-smith.html">Mark Smith</a></span>
  <span class="availdate">Dec 15, 2025</span>
  <span class="availdate">18:45 min</span>
</div>`

func listingHTML() string {
	return `<html><body><div class="content">` + card1 + card2 + `</div></body></html>`
}

const emptyHTML = `<html><body><div class="content"></div></body></html>`

func testConfig(base string) SiteConfig {
	return SiteConfig{
		ID:       "menatplay",
		Studio:   "MENatPLAY",
		SiteBase: base,
		Slug:     "movies",
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?menatplay\.com`),
	}
}

func TestToScene_videoPoster(t *testing.T) {
	s := New(testConfig("https://menatplay.com"))
	sc, ok := s.toScene("https://menatplay.com", card1, timeFixed())
	if !ok {
		t.Fatal("toScene returned ok=false")
	}
	if sc.ID != "123" {
		t.Errorf("ID = %q, want 123", sc.ID)
	}
	if sc.Title != "Suited Seduction" {
		t.Errorf("Title = %q", sc.Title)
	}
	if sc.URL != "https://menatplay.com/updates/SuitedSeduction.html" {
		t.Errorf("URL = %q", sc.URL)
	}
	if sc.Thumbnail != "https://menatplay.com/content/contentthumbs/01/23/123-1x.jpg" {
		t.Errorf("Thumbnail = %q", sc.Thumbnail)
	}
	if len(sc.Performers) != 2 || sc.Performers[0] != "John Doe" || sc.Performers[1] != "Jack Roe" {
		t.Errorf("Performers = %v", sc.Performers)
	}
	if sc.Date.Year() != 2026 || sc.Date.Month() != 1 || sc.Date.Day() != 23 {
		t.Errorf("Date = %v, want 2026-01-23", sc.Date)
	}
	if sc.Duration != 25*60+30 {
		t.Errorf("Duration = %d, want %d", sc.Duration, 25*60+30)
	}
	if sc.SiteID != "menatplay" || sc.Studio != "MENatPLAY" {
		t.Errorf("SiteID/Studio = %q/%q", sc.SiteID, sc.Studio)
	}
}

func TestToScene_imgThumbAndUnescape(t *testing.T) {
	s := New(testConfig("https://menatplay.com"))
	sc, ok := s.toScene("https://menatplay.com", card2, timeFixed())
	if !ok {
		t.Fatal("toScene returned ok=false")
	}
	if sc.ID != "456" {
		t.Errorf("ID = %q, want 456", sc.ID)
	}
	if sc.Title != "Locker Room & Shower" {
		t.Errorf("Title = %q (want unescaped)", sc.Title)
	}
	if sc.Thumbnail != "https://menatplay.com/content/contentthumbs/04/56/456-1x.jpg" {
		t.Errorf("Thumbnail = %q", sc.Thumbnail)
	}
	if len(sc.Performers) != 1 || sc.Performers[0] != "Mark Smith" {
		t.Errorf("Performers = %v", sc.Performers)
	}
	if sc.Duration != 18*60+45 {
		t.Errorf("Duration = %d", sc.Duration)
	}
}

func TestToScene_relativeURLAndSlugFallback(t *testing.T) {
	s := New(testConfig("https://menatplay.com"))
	card := `<div class="updateDetails"><a href="/updates/HotScene.html"></a></div>`
	sc, ok := s.toScene("https://menatplay.com", card, timeFixed())
	if !ok {
		t.Fatal("ok=false")
	}
	if sc.URL != "https://menatplay.com/updates/HotScene.html" {
		t.Errorf("URL = %q", sc.URL)
	}
	// No thumbnail → ID falls back to the URL slug.
	if sc.ID != "HotScene" {
		t.Errorf("ID = %q, want slug fallback HotScene", sc.ID)
	}
	// No <h4> → Title derived from slug.
	if sc.Title != "HotScene" {
		t.Errorf("Title = %q", sc.Title)
	}
}

func TestToScene_rejectsNonUpdateCard(t *testing.T) {
	s := New(testConfig("https://menatplay.com"))
	if _, ok := s.toScene("https://menatplay.com", `<div class="updateDetails"><a href="/models/foo.html"></a></div>`, timeFixed()); ok {
		t.Error("expected ok=false for a non-/updates/ link")
	}
	if _, ok := s.toScene("https://menatplay.com", `<div class="updateDetails">no link</div>`, timeFixed()); ok {
		t.Error("expected ok=false when no href present")
	}
}

func TestListingURLRe_categoryDetection(t *testing.T) {
	cases := []struct {
		url  string
		want string
		ok   bool
	}{
		{"https://menatplay.com/categories/masqulin_2_d.html", "masqulin", true},
		{"https://menatplay.com/categories/movies_1_d.html", "movies", true},
		{"https://menatplay.com/updates/Foo.html", "", false},
	}
	for _, c := range cases {
		m := listingURLRe.FindStringSubmatch(c.url)
		if c.ok {
			if m == nil {
				t.Errorf("%q: no match, want %q", c.url, c.want)
				continue
			}
			if m[1] != c.want {
				t.Errorf("%q: slug = %q, want %q", c.url, m[1], c.want)
			}
		} else if m != nil {
			t.Errorf("%q: unexpected match %v", c.url, m)
		}
	}
}

func TestMatchesURL(t *testing.T) {
	s := New(testConfig("https://menatplay.com"))
	cases := []struct {
		url   string
		match bool
	}{
		{"https://menatplay.com/", true},
		{"http://www.menatplay.com/categories/movies_2_d.html", true},
		{"https://example.com/", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.match {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.match)
		}
	}
}

func TestListScenes_endToEnd(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		switch r.URL.Path {
		case "/categories/movies_1_d.html":
			_, _ = fmt.Fprint(w, listingHTML())
		default:
			_, _ = fmt.Fprint(w, emptyHTML)
		}
	}))
	defer ts.Close()

	s := New(SiteConfig{
		ID:       "menatplay",
		Studio:   "MENatPLAY",
		SiteBase: ts.URL,
		Slug:     "movies",
		MatchRe:  regexp.MustCompile(`.*`),
	})

	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}

	var ids []string
	for r := range ch {
		switch r.Kind {
		case scraper.KindScene:
			ids = append(ids, r.Scene.ID)
			if r.Scene.Studio != "MENatPLAY" {
				t.Errorf("Studio = %q", r.Scene.Studio)
			}
			if r.Scene.Date.IsZero() {
				t.Errorf("Date zero for %q", r.Scene.Title)
			}
		case scraper.KindError:
			t.Errorf("unexpected error: %v", r.Err)
		}
	}
	if len(ids) != 2 {
		t.Fatalf("got %d scenes %v, want 2", len(ids), ids)
	}
	if ids[0] != "123" || ids[1] != "456" {
		t.Errorf("ids = %v, want [123 456]", ids)
	}
}

func TestListScenes_categoryURLOverridesSlug(t *testing.T) {
	var requested string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		switch r.URL.Path {
		case "/categories/masqulin_1_d.html":
			requested = r.URL.Path
			_, _ = fmt.Fprint(w, listingHTML())
		default:
			_, _ = fmt.Fprint(w, emptyHTML)
		}
	}))
	defer ts.Close()

	s := New(SiteConfig{
		ID:       "menatplay",
		Studio:   "MENatPLAY",
		SiteBase: ts.URL,
		Slug:     "movies", // overridden by the category URL below
		MatchRe:  regexp.MustCompile(`.*`),
	})

	studioURL := ts.URL + "/categories/masqulin_1_d.html"
	ch, err := s.ListScenes(context.Background(), studioURL, scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}
	var scenes int
	for r := range ch {
		if r.Kind == scraper.KindScene {
			scenes++
		}
	}
	if requested != "/categories/masqulin_1_d.html" {
		t.Errorf("category slug from URL not used; requested %q", requested)
	}
	if scenes != 2 {
		t.Errorf("got %d scenes, want 2", scenes)
	}
}

func TestSlugFromURL(t *testing.T) {
	if got := slugFromURL("https://menatplay.com/updates/Foo-Bar.html"); got != "Foo-Bar" {
		t.Errorf("slugFromURL = %q", got)
	}
	if got := slugFromURL("nopath"); got != "nopath" {
		t.Errorf("slugFromURL = %q", got)
	}
}
