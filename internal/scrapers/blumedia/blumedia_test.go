package blumedia

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

func TestDecodeID(t *testing.T) {
	cases := map[string]string{
		"Mzc1Mw==": "3753",
		"MTQ1Ng==": "1456",
		"Njcy":     "672",
	}
	for token, want := range cases {
		if got := decodeID(token); got != want {
			t.Errorf("decodeID(%q) = %q, want %q", token, got, want)
		}
	}
	// Non-base64 token falls back to the token itself.
	if got := decodeID("not!base64"); got != "not!base64" {
		t.Errorf("decodeID(non-b64) = %q", got)
	}
}

func TestParseListing(t *testing.T) {
	items := parseListing(loadFixture(t, "bsb_listing.html"))
	if len(items) != 3 {
		t.Fatalf("got %d items, want 3", len(items))
	}
	first := items[0]
	if first.id != "3968" {
		t.Errorf("id = %q, want 3968", first.id)
	}
	if first.token != "Mzk2OA==" {
		t.Errorf("token = %q", first.token)
	}
	if first.slug != "fit-bubble-butt-bottom-takes-big-dick" {
		t.Errorf("slug = %q", first.slug)
	}
	if first.title != "Fit Bubble Butt Bottom Takes Big Dick" {
		t.Errorf("title = %q", first.title)
	}
	if first.thumbnail != "https://small1.blumedia.com/thumbs/0/3/9/3968-cover.jpg" {
		t.Errorf("thumbnail = %q", first.thumbnail)
	}
	if got := first.playURL(); got != "/play/Mzk2OA==/fit-bubble-butt-bottom-takes-big-dick" {
		t.Errorf("playURL = %q", got)
	}
	if items[1].id != "3982" || items[2].id != "3965" {
		t.Errorf("ids = %q, %q", items[1].id, items[2].id)
	}
}

func TestParseListingCollegeDudes(t *testing.T) {
	// Different template (vidListC) — the generic /play/ + <img alt> parser
	// must still find scenes.
	items := parseListing(loadFixture(t, "cd_listing.html"))
	if len(items) != 3 {
		t.Fatalf("got %d items, want 3", len(items))
	}
	for _, it := range items {
		if it.id == "" || !isNumeric(it.id) {
			t.Errorf("bad id: %q", it.id)
		}
		if it.title == "" {
			t.Errorf("empty title for %q", it.id)
		}
		if !strings.HasPrefix(it.thumbnail, "https://small1.blumedia.com/thumbs/") {
			t.Errorf("bad thumbnail for %q: %q", it.id, it.thumbnail)
		}
	}
}

func TestParseListingEmpty(t *testing.T) {
	if got := parseListing([]byte("<html><body>no cards here</body></html>")); len(got) != 0 {
		t.Fatalf("got %d items, want 0", len(got))
	}
}

func TestParseDetailBSB(t *testing.T) {
	d := parseDetail(loadFixture(t, "bsb_detail.html"))
	if d.title != "Andy Takes Two Cocks Double Penetrated By Bruce And Ricky" {
		t.Errorf("title = %q", d.title)
	}
	want := []string{"Ricky Bobby", "Andy Adler", "Bruce Garcia"}
	if len(d.performers) != len(want) {
		t.Fatalf("performers = %v, want %v", d.performers, want)
	}
	for i, p := range want {
		if d.performers[i] != p {
			t.Errorf("performer[%d] = %q, want %q", i, d.performers[i], p)
		}
	}
	if d.description == "" || !strings.Contains(d.description, "Double Penetrated") {
		t.Errorf("description = %q", d.description)
	}
}

func TestParseDetailCollegeDudes(t *testing.T) {
	// Older template: h1 title + descD body, no performer links.
	d := parseDetail(loadFixture(t, "cd_detail.html"))
	if d.title != "Teen Twink Breeding Session" {
		t.Errorf("title = %q", d.title)
	}
	if len(d.performers) != 0 {
		t.Errorf("performers = %v, want none", d.performers)
	}
	if d.description == "" || !strings.Contains(d.description, "Cedric") {
		t.Errorf("description = %q", d.description)
	}
}

func TestMaxPageNum(t *testing.T) {
	if got := maxPageNum(loadFixture(t, "bsb_listing.html")); got != 9 {
		t.Errorf("maxPageNum = %d, want 9", got)
	}
}

func TestMatchesURL(t *testing.T) {
	bsb := newFor("brokestraightboys")
	cd := newFor("collegedudes")
	cases := []struct {
		url string
		bsb bool
		cd  bool
	}{
		{"https://www.brokestraightboys.com/", true, false},
		{"https://brokestraightboys.com/episodes.php?page=2", true, false},
		{"https://www.brokestraightboys.com/play/Mzc1Mw==/foo", true, false},
		{"https://www.collegedudes.com/", false, true},
		{"https://www.example.com/", false, false},
	}
	for _, c := range cases {
		if bsb.MatchesURL(c.url) != c.bsb {
			t.Errorf("bsb.MatchesURL(%q) = %v, want %v", c.url, bsb.MatchesURL(c.url), c.bsb)
		}
		if cd.MatchesURL(c.url) != c.cd {
			t.Errorf("cd.MatchesURL(%q) = %v, want %v", c.url, cd.MatchesURL(c.url), c.cd)
		}
	}
}

func TestNewFor(t *testing.T) {
	for _, id := range []string{"brokestraightboys", "boygusher", "collegeboyphysicals", "collegedudes"} {
		if s := newFor(id); s == nil || s.ID() != id {
			t.Errorf("newFor(%q) failed", id)
		}
	}
	if newFor("nope") != nil {
		t.Error("newFor unknown should be nil")
	}
}

// testServer serves the fixtures for a site: the episodes listing on page 1
// and the play/ detail page; any later page is empty so pagination stops.
func testServer(t *testing.T, listing, detail []byte) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/play/"):
			_, _ = fmt.Fprint(w, string(detail))
		case r.URL.Path == "/episodes.php" && r.URL.Query().Get("page") == "1":
			_, _ = fmt.Fprint(w, string(listing))
		default:
			_, _ = fmt.Fprint(w, "<html><body></body></html>")
		}
	}))
}

func collect(t *testing.T, s *Scraper, opts scraper.ListOpts) ([]models.Scene, []scraper.SceneResult) {
	t.Helper()
	ch, err := s.ListScenes(context.Background(), s.base+"/", opts)
	if err != nil {
		t.Fatalf("ListScenes: %v", err)
	}
	var scenes []models.Scene
	var all []scraper.SceneResult
	for r := range ch {
		all = append(all, r)
		if r.Kind == scraper.KindScene {
			scenes = append(scenes, r.Scene)
		}
		if r.Kind == scraper.KindError {
			t.Errorf("error result: %v", r.Err)
		}
	}
	return scenes, all
}

func TestRunBSB(t *testing.T) {
	ts := testServer(t, loadFixture(t, "bsb_listing.html"), loadFixture(t, "bsb_detail.html"))
	defer ts.Close()

	s := newFor("brokestraightboys")
	s.base = ts.URL
	s.Client = ts.Client()

	scenes, _ := collect(t, s, scraper.ListOpts{Workers: 2})
	if len(scenes) != 3 {
		t.Fatalf("got %d scenes, want 3", len(scenes))
	}
	sc := scenes[0]
	if sc.ID != "3968" {
		t.Errorf("ID = %q", sc.ID)
	}
	if sc.SiteID != "brokestraightboys" {
		t.Errorf("SiteID = %q", sc.SiteID)
	}
	if sc.Studio != "Broke Straight Boys" {
		t.Errorf("Studio = %q", sc.Studio)
	}
	if sc.StudioURL != ts.URL {
		t.Errorf("StudioURL = %q", sc.StudioURL)
	}
	if sc.URL != ts.URL+"/play/Mzk2OA==/fit-bubble-butt-bottom-takes-big-dick" {
		t.Errorf("URL = %q", sc.URL)
	}
	// Detail-page h1 title overrides the listing-card title for all scenes
	// (the fixture serves the same detail for every play link).
	if sc.Title != "Andy Takes Two Cocks Double Penetrated By Bruce And Ricky" {
		t.Errorf("Title = %q", sc.Title)
	}
	if len(sc.Performers) != 3 {
		t.Errorf("Performers = %v", sc.Performers)
	}
	if sc.Description == "" {
		t.Error("Description empty")
	}
	if sc.Thumbnail != "https://small1.blumedia.com/thumbs/0/3/9/3968-cover.jpg" {
		t.Errorf("Thumbnail = %q", sc.Thumbnail)
	}
	if sc.ScrapedAt.IsZero() {
		t.Error("ScrapedAt zero")
	}
}

func TestRunCollegeDudes(t *testing.T) {
	ts := testServer(t, loadFixture(t, "cd_listing.html"), loadFixture(t, "cd_detail.html"))
	defer ts.Close()

	s := newFor("collegedudes")
	s.base = ts.URL
	s.Client = ts.Client()

	scenes, _ := collect(t, s, scraper.ListOpts{Workers: 2})
	if len(scenes) != 3 {
		t.Fatalf("got %d scenes, want 3", len(scenes))
	}
	for _, sc := range scenes {
		if sc.SiteID != "collegedudes" || sc.Studio != "CollegeDudes" {
			t.Errorf("wrong site/studio: %q / %q", sc.SiteID, sc.Studio)
		}
		if sc.ID == "" || sc.Title == "" || sc.URL == "" {
			t.Errorf("missing core fields: %+v", sc)
		}
	}
}

func TestRunKnownIDsEarlyStop(t *testing.T) {
	ts := testServer(t, loadFixture(t, "bsb_listing.html"), loadFixture(t, "bsb_detail.html"))
	defer ts.Close()

	s := newFor("brokestraightboys")
	s.base = ts.URL
	s.Client = ts.Client()

	known := "3968" // first scene on the page
	ch, err := s.ListScenes(context.Background(), s.base+"/", scraper.ListOpts{
		Workers:  2,
		KnownIDs: map[string]bool{known: true},
	})
	if err != nil {
		t.Fatalf("ListScenes: %v", err)
	}
	stopped := false
	for r := range ch {
		if r.Kind == scraper.KindStoppedEarly {
			stopped = true
		}
		if r.Kind == scraper.KindScene && r.Scene.ID == known {
			t.Errorf("known scene should not be emitted: %q", known)
		}
	}
	if !stopped {
		t.Error("expected StoppedEarly result")
	}
}
