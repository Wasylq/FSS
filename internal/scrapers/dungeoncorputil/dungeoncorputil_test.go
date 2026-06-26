package dungeoncorputil

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/Wasylq/FSS/scraper"
)

func timeFixed() time.Time { return time.Date(2026, 6, 26, 0, 0, 0, 0, time.UTC) }

// Fixtures modelled on the www.dungeoncorp.com ?page=updates listing markup.
// Each card is a single <a ... class="updatebox"> element carrying full metadata.

const card1 = `<a href="https://www.societysm.com/updates/guests/2026/SSM_x_FirstScene/" title="First Bound Scene" class="updatebox" data-thumbnails='[{"src":"a.jpg"}]' data-update-id="2013">
  <img class="thumb" src="https://www.societysm.com/content/SSM_x_FirstScene/thumb.jpg">
  <div class="models">
    <a href="/?page=models&model=Jane-Doe">Jane Doe</a>
    <a href="/?page=models&model=John-Roe">John Roe</a>
  </div>
  <span><i class="fas fa-clock"></i> 04/13/2026</span>
  <span><i class="fas fa-video"></i> 43 min</span>
</a>`

const card2 = `<a href="https://www.societysm.com/updates/guests/2026/SSM_x_SecondScene/" title="Second &amp; Final" class="updatebox" data-thumbnails='[]' data-update-id="2014">
  <img src="https://www.societysm.com/content/SSM_x_SecondScene/thumb.jpg">
  <div class="models">
    <a href="/?page=models&model=Mark-Smith">Mark Smith</a>
  </div>
  <span><i class="fas fa-clock"></i> 03/01/2026</span>
  <span><i class="fas fa-video"></i> 30 min</span>
</a>`

func listingHTML() string {
	return `<html><body><div class="updates">` + card1 + "\n" + card2 + `</div></body></html>`
}

func testConfig() SiteConfig {
	return SiteConfig{
		ID:      "societysm",
		Studio:  "SocietySM",
		Code:    "SSM",
		MatchRe: regexp.MustCompile(`^https?://(?:www\.)?societysm\.com`),
	}
}

// splitCards mirrors the cardStartRe-based splitting in fetchCards so we can
// exercise the pure parse path offline (the production fetchCards hits a
// hardcoded network base and cannot be redirected via SiteConfig).
func splitCards(text string) []string {
	locs := cardStartRe.FindAllStringIndex(text, -1)
	cards := make([]string, 0, len(locs))
	for i, loc := range locs {
		end := len(text)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		cards = append(cards, text[loc[0]:end])
	}
	return cards
}

func TestSplitCards(t *testing.T) {
	cards := splitCards(listingHTML())
	if len(cards) != 2 {
		t.Fatalf("got %d cards, want 2", len(cards))
	}
	if !strings.Contains(cards[0], "First Bound Scene") {
		t.Errorf("card[0] missing first title: %q", cards[0])
	}
	if !strings.Contains(cards[1], "Second") {
		t.Errorf("card[1] missing second title")
	}
}

func TestToScene_full(t *testing.T) {
	s := New(testConfig())
	sc, ok := s.toScene("https://www.societysm.com", card1, timeFixed())
	if !ok {
		t.Fatal("toScene returned ok=false")
	}
	if sc.ID != "2013" {
		t.Errorf("ID = %q, want 2013", sc.ID)
	}
	if sc.URL != "https://www.societysm.com/updates/guests/2026/SSM_x_FirstScene/" {
		t.Errorf("URL = %q", sc.URL)
	}
	if sc.Title != "First Bound Scene" {
		t.Errorf("Title = %q", sc.Title)
	}
	if sc.Thumbnail != "https://www.societysm.com/content/SSM_x_FirstScene/thumb.jpg" {
		t.Errorf("Thumbnail = %q", sc.Thumbnail)
	}
	if len(sc.Performers) != 2 || sc.Performers[0] != "Jane Doe" || sc.Performers[1] != "John Roe" {
		t.Errorf("Performers = %v", sc.Performers)
	}
	if sc.Date.Year() != 2026 || sc.Date.Month() != 4 || sc.Date.Day() != 13 {
		t.Errorf("Date = %v, want 2026-04-13", sc.Date)
	}
	if sc.Duration != 43*60 {
		t.Errorf("Duration = %d, want %d", sc.Duration, 43*60)
	}
	if sc.SiteID != "societysm" || sc.Studio != "SocietySM" {
		t.Errorf("SiteID/Studio = %q/%q", sc.SiteID, sc.Studio)
	}
}

func TestToScene_unescapeTitle(t *testing.T) {
	s := New(testConfig())
	sc, ok := s.toScene("https://www.societysm.com", card2, timeFixed())
	if !ok {
		t.Fatal("ok=false")
	}
	if sc.Title != "Second & Final" {
		t.Errorf("Title = %q, want unescaped", sc.Title)
	}
	if sc.ID != "2014" {
		t.Errorf("ID = %q", sc.ID)
	}
	if len(sc.Performers) != 1 || sc.Performers[0] != "Mark Smith" {
		t.Errorf("Performers = %v", sc.Performers)
	}
	if sc.Duration != 30*60 {
		t.Errorf("Duration = %d", sc.Duration)
	}
}

func TestToScene_slugFallback(t *testing.T) {
	s := New(testConfig())
	card := `<a href="https://www.societysm.com/updates/guests/2026/SSM_x_NoID/" title="No ID Scene" class="updatebox">`
	sc, ok := s.toScene("https://www.societysm.com", card, timeFixed())
	if !ok {
		t.Fatal("ok=false")
	}
	if sc.ID != "SSM_x_NoID" {
		t.Errorf("ID = %q, want slug fallback SSM_x_NoID", sc.ID)
	}
}

func TestToScene_rejectsNonCard(t *testing.T) {
	s := New(testConfig())
	if _, ok := s.toScene("https://www.societysm.com", `<div>not a card</div>`, timeFixed()); ok {
		t.Error("expected ok=false for non-card input")
	}
}

func TestSlugFromURL(t *testing.T) {
	if got := slugFromURL("https://www.societysm.com/updates/guests/2026/SSM_x_Foo/"); got != "SSM_x_Foo" {
		t.Errorf("slugFromURL = %q", got)
	}
	if got := slugFromURL("bare"); got != "bare" {
		t.Errorf("slugFromURL = %q", got)
	}
}

func TestMatchesURL(t *testing.T) {
	s := New(testConfig())
	cases := []struct {
		url   string
		match bool
	}{
		{"https://www.societysm.com/", true},
		{"http://societysm.com/updates/guests/", true},
		{"https://example.com/", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.match {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.match)
		}
	}
}

// roundTripFunc lets us serve the fixture offline without touching the real
// (hardcoded) network base, so we can exercise run/fetchCards/ListScenes.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestListScenes_endToEnd(t *testing.T) {
	s := New(testConfig())
	s.Client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		q, _ := url.ParseQuery(r.URL.RawQuery)
		body := `<html><body></body></html>`
		if q.Get("p") == "1" {
			body = listingHTML()
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": {"text/html"}},
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    r,
		}, nil
	})}

	ch, err := s.ListScenes(context.Background(), "https://www.societysm.com", scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for res := range ch {
		switch res.Kind {
		case scraper.KindScene:
			ids = append(ids, res.Scene.ID)
		case scraper.KindError:
			t.Errorf("unexpected error: %v", res.Err)
		}
	}
	if len(ids) != 2 {
		t.Fatalf("got %d scenes %v, want 2", len(ids), ids)
	}
	if ids[0] != "2013" || ids[1] != "2014" {
		t.Errorf("ids = %v, want [2013 2014]", ids)
	}
}
