package gangbangmedia

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

func time0() time.Time { return time.Date(2026, 6, 26, 0, 0, 0, 0, time.UTC) }

// ---- fixtures ----

// card renders one listing card. The page is split on the `<a class="h-100"
// href="/video/` anchor, so each card chunk begins with the slug.
func card(slug, videoID, title, thumb, dur string) string {
	return fmt.Sprintf(`<a class="h-100" href="/video/%s" data-controller="thumb" data-thumb-videoid-value="%s">
  <img class="card-img-top" src="%s" />
  <span><i class="fa fa-clock-o"></i> %s</span>
  <div class="title"><strong>%s</strong></div>
</a>`, slug, videoID, thumb, dur, title)
}

func listingHTML(cards ...string) string {
	var sb strings.Builder
	sb.WriteString(`<html><body><div class="grid">`)
	for _, c := range cards {
		sb.WriteString(c)
	}
	sb.WriteString(`</div></body></html>`)
	return sb.String()
}

// ---- MatchesURL ----

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url   string
		match bool
	}{
		{"https://p-p-p.tv/videos/list", true},
		{"http://www.p-p-p.tv/video/foo", true},
		{"https://example.com/", false},
		{"", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.match {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.match)
		}
	}
}

// ---- card parser ----

func TestToScene(t *testing.T) {
	c := card("alicia-dark-und-diana-love", "12345",
		"Heisser Gangbang &amp; Mehr", "/thumbs/12345.jpg", "25:30")
	// Drop the split anchor prefix so the chunk starts at the slug, like the
	// real split produces.
	chunk := strings.TrimPrefix(c, `<a class="h-100" href="/video/`)

	sc, ok := toScene("https://p-p-p.tv/videos/list", chunk, time0())
	if !ok {
		t.Fatal("toScene returned ok=false")
	}
	if sc.ID != "12345" {
		t.Errorf("ID = %q", sc.ID)
	}
	if sc.SiteID != siteID {
		t.Errorf("SiteID = %q", sc.SiteID)
	}
	if sc.URL != "https://p-p-p.tv/video/alicia-dark-und-diana-love" {
		t.Errorf("URL = %q", sc.URL)
	}
	if sc.Title != "Heisser Gangbang & Mehr" {
		t.Errorf("Title = %q", sc.Title)
	}
	if sc.Thumbnail != "https://p-p-p.tv/thumbs/12345.jpg" {
		t.Errorf("Thumbnail = %q", sc.Thumbnail)
	}
	if sc.Duration != 25*60+30 {
		t.Errorf("Duration = %d, want %d", sc.Duration, 25*60+30)
	}
	want := []string{"Alicia Dark", "Diana Love"}
	if len(sc.Performers) != 2 || sc.Performers[0] != want[0] || sc.Performers[1] != want[1] {
		t.Errorf("Performers = %v, want %v", sc.Performers, want)
	}
	if !sc.Date.IsZero() {
		t.Errorf("Date = %v, want zero", sc.Date)
	}
}

func TestToSceneTitleFallbackFromSlug(t *testing.T) {
	// No <strong> title -> title derived from slug (with "und" joiner dropped).
	chunk := `nina-elle-und-sara-luvv" data-thumb-videoid-value="777">
  <img class="card-img-top" src="/t/777.jpg" />
  <i class="fa fa-clock-o"></i> 12:00`
	sc, ok := toScene("https://p-p-p.tv", chunk, time0())
	if !ok {
		t.Fatal("toScene returned ok=false")
	}
	if sc.Title != "Nina Elle Sara Luvv" {
		t.Errorf("Title = %q, want slug-derived", sc.Title)
	}
	if len(sc.Performers) != 2 {
		t.Errorf("Performers = %v", sc.Performers)
	}
}

func TestPerformersFromSlugDropsTeil(t *testing.T) {
	got := performersFromSlug("alicia-dark-und-diana-love-teil-2")
	want := []string{"Alicia Dark", "Diana Love"}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("performersFromSlug = %v, want %v", got, want)
	}
}

func TestToSceneRejectsNonCard(t *testing.T) {
	if _, ok := toScene("https://p-p-p.tv", `no-video-id-here" foo`, time0()); ok {
		t.Error("expected ok=false when video id is missing")
	}
}

// ---- end-to-end ----

func TestListScenesEndToEnd(t *testing.T) {
	prev := siteBase
	t.Cleanup(func() { siteBase = prev })

	page1 := listingHTML(
		card("alicia-dark-und-diana-love", "111", "Scene One", "/t/111.jpg", "20:00"),
		card("kira-noir", "222", "Scene Two", "/t/222.jpg", "15:45"),
	)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		switch r.URL.Path {
		case "/videos/list":
			if r.URL.Query().Get("page") == "1" {
				_, _ = fmt.Fprint(w, page1)
				return
			}
			http.NotFound(w, r) // past the last page -> clean stop
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()
	siteBase = ts.URL

	s := &Scraper{client: ts.Client()}
	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{})
	if err != nil {
		t.Fatalf("ListScenes error: %v", err)
	}

	got := map[string]string{}
	for r := range ch {
		switch r.Kind {
		case scraper.KindScene:
			got[r.Scene.ID] = r.Scene.Title
		case scraper.KindError:
			t.Errorf("unexpected error: %v", r.Err)
		}
	}
	if len(got) != 2 {
		t.Fatalf("got %d scenes, want 2: %v", len(got), got)
	}
	if got["111"] != "Scene One" || got["222"] != "Scene Two" {
		t.Errorf("unexpected scenes: %v", got)
	}
}
