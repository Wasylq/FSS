package clubdom

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

func TestParseListing(t *testing.T) {
	items := parseListing(loadFixture(t, "clubdom_listing.html"))
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	first := items[0]
	if first.id != "CD-230825-02-Whipping-Mistress-Blaze-Mistress-Macy-Nikole" {
		t.Errorf("id = %q", first.id)
	}
	if first.url != "/tour/trailers/CD-230825-02-Whipping-Mistress-Blaze-Mistress-Macy-Nikole.html" {
		t.Errorf("url = %q", first.url)
	}
	if first.title != "Whipping Mistress Blaze Mistress Macy Nikole 2" {
		t.Errorf("title = %q", first.title)
	}
	if first.thumbnail != "https://www.clubdom.com/tour/content//contentthumbs/54/72/85472-2x.jpg" {
		t.Errorf("thumbnail = %q", first.thumbnail)
	}
	if first.duration != 11*60+42 {
		t.Errorf("duration = %d, want 702", first.duration)
	}
	if got := first.date.Format("2006-01-02"); got != "2026-05-30" {
		t.Errorf("date = %q", got)
	}
	// Title with HTML entity (&amp;) must be unescaped.
	if !strings.Contains(items[1].title, "&") {
		t.Errorf("second title not unescaped: %q", items[1].title)
	}
}

func TestParseListingEmpty(t *testing.T) {
	if got := parseListing([]byte("<html><body>no cards here</body></html>")); len(got) != 0 {
		t.Fatalf("got %d items, want 0", len(got))
	}
}

func TestParseDetail(t *testing.T) {
	d := parseDetail(loadFixture(t, "clubdom_detail.html"))
	if d.duration != 11*60+42 {
		t.Errorf("duration = %d, want 702", d.duration)
	}
	if got := d.date.Format("2006-01-02"); got != "2026-05-30" {
		t.Errorf("date = %q, want 2026-05-30", got)
	}
	if len(d.tags) != 1 || d.tags[0] != "Whipping" {
		t.Errorf("tags = %v, want [Whipping]", d.tags)
	}
}

func TestMaxPageNum(t *testing.T) {
	if got := maxPageNum(loadFixture(t, "clubdom_listing.html")); got < 5 {
		t.Errorf("maxPageNum = %d, want >= 5", got)
	}
}

func TestMatchesURL(t *testing.T) {
	cd := New(SiteConfig{SiteID: "clubdom", Domain: "clubdom.com", StudioName: "Club Dom"})
	sh := New(SiteConfig{SiteID: "subbyhubby", Domain: "subbyhubby.com", StudioName: "Subby Hubby"})
	cases := []struct {
		url string
		cd  bool
		sh  bool
	}{
		{"https://www.clubdom.com/", true, false},
		{"https://clubdom.com/tour/categories/movies/1/latest/", true, false},
		{"https://www.subbyhubby.com/", false, true},
		{"https://www.example.com/", false, false},
	}
	for _, c := range cases {
		if cd.MatchesURL(c.url) != c.cd {
			t.Errorf("clubdom.MatchesURL(%q) = %v, want %v", c.url, cd.MatchesURL(c.url), c.cd)
		}
		if sh.MatchesURL(c.url) != c.sh {
			t.Errorf("subbyhubby.MatchesURL(%q) = %v, want %v", c.url, sh.MatchesURL(c.url), c.sh)
		}
	}
}

func TestNewFor(t *testing.T) {
	if newFor("clubdom").ID() != "clubdom" {
		t.Error("newFor clubdom")
	}
	if newFor("subbyhubby").ID() != "subbyhubby" {
		t.Error("newFor subbyhubby")
	}
	if newFor("nope") != nil {
		t.Error("newFor unknown should be nil")
	}
}

// testServer serves the fixtures for a given site, mapping the tour listing
// and trailer detail paths.
func testServer(t *testing.T, listing, detail []byte) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/tour/categories/movies/1/"):
			_, _ = fmt.Fprint(w, string(listing))
		case strings.HasPrefix(r.URL.Path, "/tour/trailers/"):
			_, _ = fmt.Fprint(w, string(detail))
		default:
			// Any later page is empty -> stops pagination.
			_, _ = fmt.Fprint(w, "<html><body></body></html>")
		}
	}))
}

func collect(t *testing.T, s *Scraper) ([]models.Scene, []scraper.SceneResult) {
	t.Helper()
	ch, err := s.ListScenes(context.Background(), s.base+"/", scraper.ListOpts{Workers: 2})
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

func TestRunClubDom(t *testing.T) {
	ts := testServer(t, loadFixture(t, "clubdom_listing.html"), loadFixture(t, "clubdom_detail.html"))
	defer ts.Close()

	s := New(SiteConfig{SiteID: "clubdom", Domain: "clubdom.com", StudioName: "Club Dom"})
	s.base = ts.URL
	s.Client = ts.Client()

	scenes, _ := collect(t, s)
	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2", len(scenes))
	}
	sc := scenes[0]
	if sc.ID != "CD-230825-02-Whipping-Mistress-Blaze-Mistress-Macy-Nikole" {
		t.Errorf("ID = %q", sc.ID)
	}
	if sc.SiteID != "clubdom" {
		t.Errorf("SiteID = %q", sc.SiteID)
	}
	if sc.Studio != "Club Dom" {
		t.Errorf("Studio = %q", sc.Studio)
	}
	if sc.Title == "" {
		t.Error("Title empty")
	}
	if sc.URL != ts.URL+"/tour/trailers/CD-230825-02-Whipping-Mistress-Blaze-Mistress-Macy-Nikole.html" {
		t.Errorf("URL = %q", sc.URL)
	}
	if sc.StudioURL != ts.URL {
		t.Errorf("StudioURL = %q", sc.StudioURL)
	}
	if sc.Duration != 11*60+42 {
		t.Errorf("Duration = %d", sc.Duration)
	}
	if got := sc.Date.Format("2006-01-02"); got != "2026-05-30" {
		t.Errorf("Date = %q", got)
	}
	if len(sc.Tags) != 1 || sc.Tags[0] != "Whipping" {
		t.Errorf("Tags = %v", sc.Tags)
	}
	if sc.ScrapedAt.IsZero() {
		t.Error("ScrapedAt zero")
	}
	if sc.Thumbnail == "" {
		t.Error("Thumbnail empty")
	}
}

func TestRunSubbyHubby(t *testing.T) {
	ts := testServer(t, loadFixture(t, "subbyhubby_listing.html"), loadFixture(t, "subbyhubby_detail.html"))
	defer ts.Close()

	s := New(SiteConfig{SiteID: "subbyhubby", Domain: "subbyhubby.com", StudioName: "Subby Hubby"})
	s.base = ts.URL
	s.Client = ts.Client()

	scenes, _ := collect(t, s)
	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2", len(scenes))
	}
	for _, sc := range scenes {
		if sc.SiteID != "subbyhubby" || sc.Studio != "Subby Hubby" {
			t.Errorf("wrong site/studio: %q / %q", sc.SiteID, sc.Studio)
		}
		if sc.ID == "" || sc.Title == "" {
			t.Errorf("missing id/title: %+v", sc)
		}
	}
}

func TestRunKnownIDsEarlyStop(t *testing.T) {
	ts := testServer(t, loadFixture(t, "clubdom_listing.html"), loadFixture(t, "clubdom_detail.html"))
	defer ts.Close()

	s := New(SiteConfig{SiteID: "clubdom", Domain: "clubdom.com", StudioName: "Club Dom"})
	s.base = ts.URL
	s.Client = ts.Client()

	known := "CD-230825-02-Whipping-Mistress-Blaze-Mistress-Macy-Nikole"
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
