package realitystudioutil

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"regexp"
	"testing"
	"time"

	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/scraper"
)

// clipsJS mirrors the real /js/clips.js shape: a static ClipDatabase JS array.
// Row layout: [ filename, title, date, pictureCount, performers, fetishes, flag ]
// The first row is the empty header/placeholder, dates vary between - and /,
// and one row has whitespace before its commas.
const clipsJS = `var ClipDatabase = [
["","","","","","",""],
["scene-one","First Scene","01-15-26","120","Jane Doe, Mary Smith","Bondage, Latex","1"],
["scene-two","Second &amp; Scene","12/31/25","80","John Smith","Spanking","0"],
["scene-three" , "Third Scene" , "01/02/2006" , "5" , "Anna" , "Tag One, Tag Two" , "1"],
["scene-four","","03-04-2026","1","","",""]
];`

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/js/clips.js":
			w.Header().Set("Content-Type", "application/javascript")
			_, _ = fmt.Fprint(w, clipsJS)
		default:
			http.NotFound(w, r)
		}
	}))
}

func drain(t *testing.T, ch <-chan scraper.SceneResult) ([]models.Scene, int) {
	t.Helper()
	var scenes []models.Scene
	total := -1
	for r := range ch {
		switch r.Kind {
		case scraper.KindScene:
			scenes = append(scenes, r.Scene)
		case scraper.KindTotal:
			total = r.Total
		case scraper.KindError:
			t.Errorf("unexpected error: %v", r.Err)
		}
	}
	return scenes, total
}

func TestListScenes_endToEnd(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	s := New(SiteConfig{
		ID:       "subbygirls",
		Studio:   "Subby Girls",
		SiteBase: ts.URL,
		MatchRe:  regexp.MustCompile(`.*`),
	})

	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}
	scenes, total := drain(t, ch)

	// 4 non-empty rows (header skipped).
	if total != 4 {
		t.Errorf("Progress total = %d, want 4", total)
	}
	if len(scenes) != 4 {
		t.Fatalf("got %d scenes, want 4", len(scenes))
	}

	first := scenes[0]
	if first.ID != "scene-one" {
		t.Errorf("ID = %q", first.ID)
	}
	if first.SiteID != "subbygirls" || first.Studio != "Subby Girls" {
		t.Errorf("SiteID/Studio = %q/%q", first.SiteID, first.Studio)
	}
	if first.Title != "First Scene" {
		t.Errorf("Title = %q", first.Title)
	}
	// idx 1 in the rows slice (header is idx 0).
	if first.URL != ts.URL+"/gallery.html?1" {
		t.Errorf("URL = %q", first.URL)
	}
	if first.Thumbnail != ts.URL+"/images/gallery/scene-one/Pics/scene-one1.jpg" {
		t.Errorf("Thumbnail = %q", first.Thumbnail)
	}
	if first.Date.Year() != 2026 || first.Date.Month() != 1 || first.Date.Day() != 15 {
		t.Errorf("Date = %v, want 2026-01-15", first.Date)
	}
	if !reflect.DeepEqual(first.Performers, []string{"Jane Doe", "Mary Smith"}) {
		t.Errorf("Performers = %v", first.Performers)
	}
	if !reflect.DeepEqual(first.Tags, []string{"Bondage", "Latex"}) {
		t.Errorf("Tags = %v", first.Tags)
	}

	second := scenes[1]
	if second.URL != ts.URL+"/gallery.html?2" {
		t.Errorf("second URL = %q", second.URL)
	}
	if second.Title != "Second & Scene" {
		t.Errorf("second Title (unescaped) = %q", second.Title)
	}
	if second.Date.Year() != 2025 || second.Date.Month() != 12 || second.Date.Day() != 31 {
		t.Errorf("second Date = %v, want 2025-12-31", second.Date)
	}

	third := scenes[2]
	// whitespace-before-comma row still parses, 4-digit year layout.
	if third.ID != "scene-three" {
		t.Errorf("third ID = %q", third.ID)
	}
	if third.Date.Year() != 2006 || third.Date.Month() != 1 || third.Date.Day() != 2 {
		t.Errorf("third Date = %v, want 2006-01-02", third.Date)
	}
	if !reflect.DeepEqual(third.Tags, []string{"Tag One", "Tag Two"}) {
		t.Errorf("third Tags = %v", third.Tags)
	}

	fourth := scenes[3]
	// empty title falls back to filename; empty performer/tag lists are nil.
	if fourth.Title != "scene-four" {
		t.Errorf("fourth Title fallback = %q", fourth.Title)
	}
	if fourth.Performers != nil {
		t.Errorf("fourth Performers = %v, want nil", fourth.Performers)
	}
	if fourth.Tags != nil {
		t.Errorf("fourth Tags = %v, want nil", fourth.Tags)
	}
	if fourth.Date.Year() != 2026 || fourth.Date.Month() != 3 || fourth.Date.Day() != 4 {
		t.Errorf("fourth Date = %v, want 2026-03-04", fourth.Date)
	}
}

func TestListScenes_fetchError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer ts.Close()

	s := New(SiteConfig{
		ID:       "subbygirls",
		Studio:   "Subby Girls",
		SiteBase: ts.URL,
		MatchRe:  regexp.MustCompile(`.*`),
	})
	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}
	var gotErr bool
	for r := range ch {
		if r.Kind == scraper.KindError {
			gotErr = true
		}
	}
	if !gotErr {
		t.Error("expected an error when clips.js is unreachable")
	}
}

func TestSplitList(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"   ", nil},
		{"Jane Doe", []string{"Jane Doe"}},
		{"Jane Doe, Mary Smith", []string{"Jane Doe", "Mary Smith"}},
		{" A , , B ,", []string{"A", "B"}},
		{"X &amp; Y", []string{"X & Y"}},
	}
	for _, c := range cases {
		if got := splitList(c.in); !reflect.DeepEqual(got, c.want) {
			t.Errorf("splitList(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestDateLayouts(t *testing.T) {
	s := New(SiteConfig{ID: "x", Studio: "X", SiteBase: "https://x.test", MatchRe: regexp.MustCompile(`.*`)})
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		raw     string
		y, m, d int
	}{
		{"01-15-26", 2026, 1, 15},
		{"12/31/25", 2025, 12, 31},
		{"01-02-2006", 2006, 1, 2},
		{"01/02/2006", 2006, 1, 2},
	}
	for _, c := range cases {
		row := []string{"", "fn", "title", c.raw, "0", "", ""}
		scene := s.toScene("https://x.test", 1, row, now)
		if scene.Date.Year() != c.y || int(scene.Date.Month()) != c.m || scene.Date.Day() != c.d {
			t.Errorf("date %q parsed to %v, want %04d-%02d-%02d", c.raw, scene.Date, c.y, c.m, c.d)
		}
	}
}

func TestMatchesURL(t *testing.T) {
	s := New(SiteConfig{
		ID:       "subbygirls",
		SiteBase: "https://www.subbygirls.com",
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?subbygirls\.com`),
	})
	if !s.MatchesURL("https://www.subbygirls.com/") {
		t.Error("expected match")
	}
	if s.MatchesURL("https://example.com/") {
		t.Error("unexpected match")
	}
}
