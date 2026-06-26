package femjoy

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

// card builds one "post_item video" card body (the text that follows the
// cardSplitRe marker), so a listing page is the marker + this for each scene.
func card(id, title, poster, date, dur, model, photog string) string {
	return fmt.Sprintf(`<div class="post_item video" data-post-id="%s" data-media-poster="%s">
  <span class="posted_on">%s</span>
  <span><i class="fa fa-video"></i> %s</span>
  <h1><a href="/join" title="%s">%s</a></h1>
  <h2><a href="/models/x" title="%s">%s</a> &amp; <a href="/photographers/y" title="%s">%s</a></h2>
</div>`, id, poster, date, dur, title, title, model, model, photog, photog)
}

func listingHTML(cards ...string) string {
	var sb strings.Builder
	sb.WriteString("<html><body><div class=\"listing\">")
	for _, c := range cards {
		sb.WriteString(c)
	}
	sb.WriteString("</div></body></html>")
	return sb.String()
}

func card1() string {
	return card("48852", "Morning Light", "https://cdn.femjoy.com/48852/poster.jpg",
		"Jan 2, 2024", "12:34", "Jane Doe", "John Smith")
}

func card2() string {
	return card("48853", "Sea &amp; Sun", "https://cdn.femjoy.com/48853/poster.jpg",
		"Feb 15, 2024", "08:05", "Mary Major", "John Smith")
}

// ---- TestMatchesURL ----

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url   string
		match bool
	}{
		{"https://www.femjoy.com/videos", true},
		{"https://femjoy.com/gallery/123", true},
		{"http://femjoy.com", true},
		{"https://www.manyvids.com/x", false},
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
	now := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	sc, ok := toScene("https://www.femjoy.com/videos", card1(), now)
	if !ok {
		t.Fatal("toScene returned ok=false")
	}
	if sc.ID != "48852" {
		t.Errorf("ID = %q, want 48852", sc.ID)
	}
	if sc.SiteID != "femjoy" {
		t.Errorf("SiteID = %q", sc.SiteID)
	}
	if sc.Title != "Morning Light" {
		t.Errorf("Title = %q", sc.Title)
	}
	if sc.URL != "https://www.femjoy.com/gallery/48852" {
		t.Errorf("URL = %q", sc.URL)
	}
	if sc.Thumbnail != "https://cdn.femjoy.com/48852/poster.jpg" {
		t.Errorf("Thumbnail = %q", sc.Thumbnail)
	}
	wantDate := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
	if !sc.Date.Equal(wantDate) {
		t.Errorf("Date = %v, want %v", sc.Date, wantDate)
	}
	if sc.Duration != 754 {
		t.Errorf("Duration = %d, want 754 (12:34)", sc.Duration)
	}
	if len(sc.Performers) != 1 || sc.Performers[0] != "Jane Doe" {
		t.Errorf("Performers = %v, want [Jane Doe]", sc.Performers)
	}
	if sc.Director != "John Smith" {
		t.Errorf("Director = %q, want John Smith", sc.Director)
	}
}

func TestToSceneNoID(t *testing.T) {
	now := time.Now().UTC()
	if _, ok := toScene("u", `<div class="post_item video">no id here</div>`, now); ok {
		t.Error("toScene should return ok=false when no data-post-id")
	}
}

func TestToSceneHTMLEntity(t *testing.T) {
	now := time.Now().UTC()
	sc, ok := toScene("u", card2(), now)
	if !ok {
		t.Fatal("ok=false")
	}
	if sc.Title != "Sea & Sun" {
		t.Errorf("Title = %q, want 'Sea & Sun'", sc.Title)
	}
}

// ---- TestListScenes (end-to-end via httptest) ----

func TestListScenes(t *testing.T) {
	orig := siteBase
	defer func() { siteBase = orig }()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.RawQuery, "page=1") {
			_, _ = fmt.Fprint(w, listingHTML(card1(), card2()))
			return
		}
		_, _ = fmt.Fprint(w, listingHTML())
	}))
	defer ts.Close()
	siteBase = ts.URL

	s := &Scraper{client: ts.Client()}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/videos", scraper.ListOpts{})
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
	if got["48852"] != "Morning Light" || got["48853"] != "Sea & Sun" {
		t.Errorf("scenes = %v", got)
	}
}
