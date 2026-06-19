package alexandrasnow

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/scraper"
)

func loadFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return b
}

// newTestServer serves the listing fixture for /vod/updates/page_1 and the
// detail fixture for any /vod/scenes/ path; later listing pages are empty so
// pagination terminates.
func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	listing := loadFixture(t, "listing.html")
	detail := loadFixture(t, "detail.html")
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/vod/scenes/"):
			_, _ = fmt.Fprint(w, string(detail))
		case strings.Contains(r.URL.Path, "page_1.html"):
			_, _ = fmt.Fprint(w, string(listing))
		default:
			// empty page -> no grid cards -> Paginate stops
			_, _ = fmt.Fprint(w, "<html><body></body></html>")
		}
	}))
}

func collect(t *testing.T, s *Scraper, studioURL string, opts scraper.ListOpts) []models.Scene {
	t.Helper()
	ch, err := s.ListScenes(context.Background(), studioURL, opts)
	if err != nil {
		t.Fatalf("ListScenes: %v", err)
	}
	var scenes []models.Scene
	for res := range ch {
		switch res.Kind {
		case scraper.KindScene:
			scenes = append(scenes, res.Scene)
		case scraper.KindError:
			t.Fatalf("scraper error: %v", res.Err)
		}
	}
	return scenes
}

func TestParseListing(t *testing.T) {
	items := parseListingPage(loadFixture(t, "listing.html"))
	if len(items) != 2 {
		t.Fatalf("expected 2 cards, got %d", len(items))
	}

	first := items[0]
	if first.id != "2829" {
		t.Errorf("id = %q, want 2829", first.id)
	}
	if first.title != "Sissy Strap Face Fuck" {
		t.Errorf("title = %q", first.title)
	}
	if got := first.date.Format("2006-01-02"); got != "2026-06-19" {
		t.Errorf("date = %q, want 2026-06-19", got)
	}
	if first.duration != 4*60 {
		t.Errorf("duration = %d, want 240", first.duration)
	}
	if !first.hasPrice || first.price != 24.99 {
		t.Errorf("price = %v (hasPrice=%v), want 24.99", first.price, first.hasPrice)
	}
	if !strings.Contains(first.url, "Sissy-Strap-Face-Fuck_vids.html") {
		t.Errorf("url = %q", first.url)
	}

	second := items[1]
	if second.title != "Topless Assignment Day" || second.price != 16.99 {
		t.Errorf("second card = %q $%v", second.title, second.price)
	}
}

func TestParseDetail(t *testing.T) {
	d := parseDetailPage(loadFixture(t, "detail.html"))
	if d.description == "" {
		t.Error("expected non-empty description")
	}
	if !strings.Contains(d.description, "Loyalty Series") {
		t.Errorf("description = %q", d.description)
	}
	want := []string{"Femdom POV", "Pov Strap-On", "Sissy Training"}
	if len(d.categories) != len(want) {
		t.Fatalf("categories = %v, want %v", d.categories, want)
	}
	for i, c := range want {
		if d.categories[i] != c {
			t.Errorf("category[%d] = %q, want %q", i, d.categories[i], c)
		}
	}
}

func TestParseDuration(t *testing.T) {
	cases := map[string]int{
		"4&nbsp;min&nbsp;of video":   240,
		"\n5&nbsp;min&nbsp;of video": 300,
		"12:34":                      754,
		"":                           0,
	}
	for in, want := range cases {
		if got := parseDuration(in); got != want {
			t.Errorf("parseDuration(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestEndToEnd(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	s := New()
	s.client = ts.Client()
	s.base = ts.URL

	scenes := collect(t, s, ts.URL+"/vod/updates", scraper.ListOpts{})
	if len(scenes) != 2 {
		t.Fatalf("expected 2 scenes, got %d", len(scenes))
	}

	sc := scenes[0]
	if sc.ID != "2829" {
		t.Errorf("ID = %q", sc.ID)
	}
	if sc.SiteID != "alexandrasnow" {
		t.Errorf("SiteID = %q", sc.SiteID)
	}
	if sc.Studio != "Alexandra Snow" {
		t.Errorf("Studio = %q", sc.Studio)
	}
	if sc.Title != "Sissy Strap Face Fuck" {
		t.Errorf("Title = %q", sc.Title)
	}
	if sc.Duration != 240 {
		t.Errorf("Duration = %d", sc.Duration)
	}
	if !strings.HasPrefix(sc.URL, ts.URL+"/vod/scenes/") {
		t.Errorf("URL = %q", sc.URL)
	}
	// detail-enriched fields
	if !strings.Contains(sc.Description, "Loyalty Series") {
		t.Errorf("Description = %q", sc.Description)
	}
	if len(sc.Categories) != 3 {
		t.Errorf("Categories = %v", sc.Categories)
	}
	// price from the listing card
	if sc.LowestPrice != 24.99 || len(sc.PriceHistory) != 1 {
		t.Errorf("price = %v, history = %v", sc.LowestPrice, sc.PriceHistory)
	}
}

func TestKnownIDsStopEarly(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	s := New()
	s.client = ts.Client()
	s.base = ts.URL

	// First card (id 2829) is known -> Paginate stops before emitting anything.
	scenes := collect(t, s, ts.URL+"/vod/updates", scraper.ListOpts{
		KnownIDs: map[string]bool{"2829": true},
	})
	for _, sc := range scenes {
		if sc.ID == "2829" {
			t.Fatalf("known scene 2829 should not be emitted")
		}
	}
}

func TestMatchesURL(t *testing.T) {
	s := New()
	yes := []string{
		"https://www.goddesssnow.com/",
		"https://goddesssnow.com/vod/updates/page_1.html",
		"https://www.alexandrasnow.com/",
		"http://alexandrasnow.com/blog",
	}
	for _, u := range yes {
		if !s.MatchesURL(u) {
			t.Errorf("MatchesURL(%q) = false, want true", u)
		}
	}
	no := []string{"https://example.com/", "https://thenobleempire.com/"}
	for _, u := range no {
		if s.MatchesURL(u) {
			t.Errorf("MatchesURL(%q) = true, want false", u)
		}
	}
}
