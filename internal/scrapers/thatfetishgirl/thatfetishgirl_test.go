package thatfetishgirl

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

func cardHTML(setID, slug, title, thumbID, date, dur, model string) string {
	return fmt.Sprintf(`<div class="latestUpdateB" data-setid="%s">
	<div class="videoPic">
	<a href="https://thatfetishgirl.com/scenes/%s_vids.html">
	<video poster_1x="/content//contentthumbs/%s/%s/%s-1x.jpg" src="/content//contentthumbs/%s/%s/%s.mp4"></video></a>
	<div class="update_hover">
	<h4 class="link_bright">
		<a href="https://thatfetishgirl.com/scenes/%s_vids.html">%s</a>
	</h4>
	<p class="link_light">
		<a class="link_bright infolink" href="https://thatfetishgirl.com/models/%s.html">%s</a>
	</p>
	<ul class="videoInfo">
		<li class="text_med"><!-- Date --> %s</li>
		<li class="text_med"><i class="fas fa-video"></i>%s min</li>
	</ul>
	</div></div>
</div>`, setID, slug, thumbID[:2], thumbID[2:], thumbID, thumbID[:2], thumbID[2:], thumbID, slug, title, model, model, date, dur)
}

func listingHTML() string {
	return "<html><body><div class=\"iLatestUArea\">" +
		cardHTML("1671", "The-Silver-Rack", "The Silver Rack", "2892", "06/25/2026", "24", "Mia Hope") +
		cardHTML("1670", "Caged-Heat", "Caged &amp; Heat", "2891", "06/18/2026", "31", "Tina Lee Comet") +
		"</div></body></html>"
}

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := map[string]bool{
		"https://thatfetishgirl.com/updates/page_1.html": true,
		"https://www.thatfetishgirl.com/":                true,
		"https://example.com/x":                          false,
		"":                                               false,
	}
	for u, want := range cases {
		if got := s.MatchesURL(u); got != want {
			t.Errorf("MatchesURL(%q) = %v, want %v", u, got, want)
		}
	}
}

func TestParseListing(t *testing.T) {
	orig := siteBase
	defer func() { siteBase = orig }()
	siteBase = "https://thatfetishgirl.com"

	scenes := parseListing([]byte(listingHTML()), "studioURL", time.Now().UTC())
	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2", len(scenes))
	}
	sc := scenes[0]
	if sc.ID != "2892" { // numeric content id from thumbnail
		t.Errorf("ID = %q, want 2892", sc.ID)
	}
	if sc.Title != "The Silver Rack" {
		t.Errorf("Title = %q", sc.Title)
	}
	if sc.URL != "https://thatfetishgirl.com/scenes/The-Silver-Rack_vids.html" {
		t.Errorf("URL = %q", sc.URL)
	}
	if sc.Duration != 24*60 {
		t.Errorf("Duration = %d, want 1440", sc.Duration)
	}
	if len(sc.Performers) != 1 || sc.Performers[0] != "Mia Hope" {
		t.Errorf("Performers = %v", sc.Performers)
	}
	if sc.Date.Year() != 2026 || sc.Date.Month() != 6 || sc.Date.Day() != 25 {
		t.Errorf("Date = %v", sc.Date)
	}
	if !strings.Contains(sc.Thumbnail, "contentthumbs/28/92/2892-1x.jpg") {
		t.Errorf("Thumbnail = %q", sc.Thumbnail)
	}
	if scenes[1].Title != "Caged & Heat" {
		t.Errorf("title[1] = %q", scenes[1].Title)
	}
}

func TestListScenes(t *testing.T) {
	orig := siteBase
	defer func() { siteBase = orig }()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "page_1.html") {
			_, _ = fmt.Fprint(w, listingHTML())
			return
		}
		_, _ = fmt.Fprint(w, "<html><body>empty</body></html>")
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
	if got["2892"] != "The Silver Rack" {
		t.Errorf("scenes = %v", got)
	}
}
