package firstanalquest

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

func card(slug, id, title, models, date string) string {
	return fmt.Sprintf(`<li class="thumb">
	<div class="thumb-content">
		<a href="http://www.firstanalquest.com/videos/%s-%s/" class="thumb-img">
			<img src="http://www.firstanalquest.com/contents/videos_screenshots/0/%s/295x241/1.jpg" alt="%s">
			<div class="thumb-header"><span class="thumb-title">%s</span></div>
			<span class="thumb-quality">HD</span>
			<span class="thumb-duration">34:30</span>
		</a>
		<div class="thumb-footer">
			<span class="thumb-models">%s</span>
			<span class="thumb-rating">2.50</span>
			<span class="thumb-added">%s</span>
		</div>
	</div>
</li>`, slug, id, id, title, title, models, date)
}

func listingHTML() string {
	one := card("vera-star-s-ass", "754", "Vera Star&#039;s ass",
		`<a href="http://www.firstanalquest.com/models/vera-star/">Vera Star</a>`, "Jun 18, 2026")
	two := card("double-trouble", "742", "Double Trouble",
		`<a href="http://www.firstanalquest.com/models/sida/">Sida</a> <a href="http://www.firstanalquest.com/models/yuri/">Yuri</a>`,
		"Oct 30, 2017")
	return "<html><body><ul>" + one + two + "</ul></body></html>"
}

// ---- TestMatchesURL ----

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url   string
		match bool
	}{
		{"http://www.firstanalquest.com/latest-updates/", true},
		{"https://firstanalquest.com/videos/foo-1/", true},
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
	cards := cardSplitRe.Split(listingHTML(), -1)[1:]
	if len(cards) != 2 {
		t.Fatalf("got %d cards, want 2", len(cards))
	}
	now := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)

	sc, ok := toScene("studioURL", cards[0], now)
	if !ok {
		t.Fatal("toScene returned !ok for card 0")
	}
	if sc.ID != "754" {
		t.Errorf("ID = %q, want 754", sc.ID)
	}
	if sc.SiteID != siteID || sc.Studio != studioName {
		t.Errorf("SiteID=%q Studio=%q", sc.SiteID, sc.Studio)
	}
	if sc.Title != "Vera Star's ass" {
		t.Errorf("Title = %q", sc.Title)
	}
	if sc.URL != "http://www.firstanalquest.com/videos/vera-star-s-ass-754/" {
		t.Errorf("URL = %q", sc.URL)
	}
	if sc.Duration != 34*60+30 {
		t.Errorf("Duration = %d, want 2070", sc.Duration)
	}
	if sc.Resolution != "HD" {
		t.Errorf("Resolution = %q", sc.Resolution)
	}
	if sc.Thumbnail == "" || !strings.Contains(sc.Thumbnail, "/754/") {
		t.Errorf("Thumbnail = %q", sc.Thumbnail)
	}
	wantDate := time.Date(2026, 6, 18, 0, 0, 0, 0, time.UTC)
	if !sc.Date.Equal(wantDate) {
		t.Errorf("Date = %v, want %v", sc.Date, wantDate)
	}
	if strings.Join(sc.Performers, ",") != "Vera Star" {
		t.Errorf("Performers = %v", sc.Performers)
	}

	sc2, _ := toScene("studioURL", cards[1], now)
	if strings.Join(sc2.Performers, ",") != "Sida,Yuri" {
		t.Errorf("multi-model Performers = %v", sc2.Performers)
	}
}

// ---- TestListScenes (end-to-end) ----

func TestListScenes(t *testing.T) {
	orig := siteBase
	defer func() { siteBase = orig }()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Page 1 is /latest-updates/; later pages /latest-updates/{N}/ are empty.
		if r.URL.Path == "/latest-updates/" {
			_, _ = fmt.Fprint(w, listingHTML())
			return
		}
		_, _ = fmt.Fprint(w, "<html><body><ul></ul></body></html>")
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
	if got["754"] != "Vera Star's ass" || got["742"] != "Double Trouble" {
		t.Errorf("scenes = %v", got)
	}
}
