package manipulativemedia

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

	"github.com/Wasylq/FSS/scraper"
)

// ---- fixtures ----

// releaseJSON is one realistic DTI release object as returned by
// site-api.project1service.com/v2/releases.
const releaseJSON = `{
  "id": 12345,
  "title": "  A Pervy Afternoon  ",
  "dateReleased": "2025-12-01T00:00:00+00:00",
  "description": "  A steamy family scene.  ",
  "actors": [
    {"name": "Jane Doe"},
    {"name": " "},
    {"name": "John Smith"}
  ],
  "tags": [
    {"name": "Family", "isVisible": true},
    {"name": "Hidden", "isVisible": false},
    {"name": "Blonde", "isVisible": true}
  ],
  "videos": {"mediabook": {"length": 1830}},
  "images": {
    "poster": {
      "0": {
        "lg": {"url": "https://cdn.example.com/poster-lg.jpg"},
        "md": {"url": "https://cdn.example.com/poster-md.jpg"}
      },
      "1": {
        "xx": {"url": "https://cdn.example.com/secondary.jpg"}
      },
      "alternateText": "poster",
      "imageVersion": 3
    }
  },
  "stats": {"views": 4200, "likes": 88}
}`

func decodeRelease(t *testing.T, s string) release {
	t.Helper()
	var rel release
	if err := json.Unmarshal([]byte(s), &rel); err != nil {
		t.Fatalf("decode release: %v", err)
	}
	return rel
}

func releasesResponse(total int, body ...string) string {
	results := ""
	for i, b := range body {
		if i > 0 {
			results += ","
		}
		results += b
	}
	return fmt.Sprintf(`{"meta":{"count":%d,"total":%d},"result":[%s]}`, len(body), total, results)
}

// ---- mapper tests ----

func TestToScene(t *testing.T) {
	s := NewMyPervyFamily()
	rel := decodeRelease(t, releaseJSON)
	now := time.Date(2026, 6, 25, 10, 0, 0, 0, time.UTC)

	scene := s.toScene("https://www.mypervyfamily.com", rel, now)

	if scene.ID != "12345" {
		t.Errorf("ID = %q, want 12345", scene.ID)
	}
	if scene.SiteID != "mypervyfamily" {
		t.Errorf("SiteID = %q", scene.SiteID)
	}
	if scene.Title != "A Pervy Afternoon" {
		t.Errorf("Title = %q (want trimmed)", scene.Title)
	}
	if scene.Description != "A steamy family scene." {
		t.Errorf("Description = %q", scene.Description)
	}
	if scene.Studio != "My Pervy Family" {
		t.Errorf("Studio = %q", scene.Studio)
	}
	wantURL := "https://www.mypervyfamily.com/video/12345/a-pervy-afternoon"
	if scene.URL != wantURL {
		t.Errorf("URL = %q, want %q", scene.URL, wantURL)
	}
	if scene.Duration != 1830 {
		t.Errorf("Duration = %d, want 1830", scene.Duration)
	}
	wantDate := time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC)
	if !scene.Date.Equal(wantDate) {
		t.Errorf("Date = %v, want %v", scene.Date, wantDate)
	}
	// Empty/whitespace actors are dropped.
	if len(scene.Performers) != 2 || scene.Performers[0] != "Jane Doe" || scene.Performers[1] != "John Smith" {
		t.Errorf("Performers = %v, want [Jane Doe John Smith]", scene.Performers)
	}
	// Only visible tags survive.
	if len(scene.Tags) != 2 || scene.Tags[0] != "Family" || scene.Tags[1] != "Blonde" {
		t.Errorf("Tags = %v, want [Family Blonde]", scene.Tags)
	}
	// bestPoster prefers the lowest numeric index ("0") then largest size present (lg here, no xx/xl).
	if scene.Thumbnail != "https://cdn.example.com/poster-lg.jpg" {
		t.Errorf("Thumbnail = %q", scene.Thumbnail)
	}
	if scene.Views != 4200 || scene.Likes != 88 {
		t.Errorf("Views/Likes = %d/%d", scene.Views, scene.Likes)
	}
	if len(scene.PriceHistory) != 1 || !scene.PriceHistory[0].IsFree {
		t.Errorf("PriceHistory = %v, want one free snapshot", scene.PriceHistory)
	}
}

func TestToScene_badDate(t *testing.T) {
	s := NewTouchMyWife()
	rel := decodeRelease(t, `{"id":7,"title":"X","dateReleased":"not-a-date"}`)
	scene := s.toScene("https://www.touchmywife.com", rel, time.Now().UTC())
	if !scene.Date.IsZero() {
		t.Errorf("Date = %v, want zero on parse failure", scene.Date)
	}
	if scene.SiteID != "touchmywife" {
		t.Errorf("SiteID = %q", scene.SiteID)
	}
}

func TestBestPoster(t *testing.T) {
	rel := decodeRelease(t, releaseJSON)
	if got := bestPoster(rel); got != "https://cdn.example.com/poster-lg.jpg" {
		t.Errorf("bestPoster = %q", got)
	}

	// No poster at all.
	empty := decodeRelease(t, `{"id":1,"title":"x"}`)
	if got := bestPoster(empty); got != "" {
		t.Errorf("bestPoster(empty) = %q, want empty", got)
	}

	// Only metadata keys, no numeric index -> empty.
	meta := decodeRelease(t, `{"id":1,"images":{"poster":{"alternateText":"a"}}}`)
	if got := bestPoster(meta); got != "" {
		t.Errorf("bestPoster(meta-only) = %q, want empty", got)
	}

	// xx beats lg in preference order.
	pref := decodeRelease(t, `{"id":1,"images":{"poster":{"0":{"lg":{"url":"L"},"xx":{"url":"XX"}}}}}`)
	if got := bestPoster(pref); got != "XX" {
		t.Errorf("bestPoster pref = %q, want XX", got)
	}
}

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"A Pervy Afternoon": "a-pervy-afternoon",
		"  Hello, World!  ": "hello-world",
		"Multiple   Spaces": "multiple-spaces",
		"Café & Bar":        "caf-bar",
	}
	for in, want := range cases {
		if got := slugify(in); got != want {
			t.Errorf("slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMatchesURL(t *testing.T) {
	mpf := NewMyPervyFamily()
	tmw := NewTouchMyWife()
	if !mpf.MatchesURL("https://www.mypervyfamily.com/videos") {
		t.Error("MPF should match its own domain")
	}
	if mpf.MatchesURL("https://www.touchmywife.com/") {
		t.Error("MPF should not match touchmywife")
	}
	if !tmw.MatchesURL("http://touchmywife.com/video/1/x") {
		t.Error("TMW should match its own domain")
	}
}

func TestIDAndPatterns(t *testing.T) {
	mpf := NewMyPervyFamily()
	if mpf.ID() != "mypervyfamily" {
		t.Errorf("ID = %q", mpf.ID())
	}
	pats := mpf.Patterns()
	if len(pats) == 0 || pats[0] != "mypervyfamily.com" {
		t.Errorf("Patterns = %v", pats)
	}
}

// ---- end-to-end run() via httptest ----

func TestListScenes_endToEnd(t *testing.T) {
	// Two releases on page 1, total=2 so pagination stops after one page.
	rel1 := releaseJSON
	rel2 := `{"id":222,"title":"Second Scene","dateReleased":"2025-11-01T00:00:00+00:00","videos":{"mediabook":{"length":600}}}`

	var releasesHits int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			http.SetCookie(w, &http.Cookie{Name: "instance_token", Value: "tok-abc"})
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprint(w, "<html><body>ok</body></html>")
		case "/v2/releases":
			releasesHits++
			if r.Header.Get("Instance") != "tok-abc" {
				t.Errorf("missing/incorrect Instance header: %q", r.Header.Get("Instance"))
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, releasesResponse(2, rel1, rel2))
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	// Point both the tour base and the api base at the test server.
	oldAPI := apiBase
	apiBase = ts.URL + "/v2/releases"
	defer func() { apiBase = oldAPI }()

	s := newScraper(site{
		id:      "mypervyfamily",
		name:    "My Pervy Family",
		base:    ts.URL,
		matchRe: regexp.MustCompile(`.*`),
	})
	s.client = ts.Client()

	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{})
	if err != nil {
		t.Fatalf("ListScenes: %v", err)
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
	if got["12345"] != "A Pervy Afternoon" || got["222"] != "Second Scene" {
		t.Errorf("scenes = %v", got)
	}
}

func TestListScenes_tokenBootstrapFails(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Homepage never sets instance_token.
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, "<html></html>")
	}))
	defer ts.Close()

	s := newScraper(site{
		id:      "mypervyfamily",
		name:    "My Pervy Family",
		base:    ts.URL,
		matchRe: regexp.MustCompile(`.*`),
	})
	s.client = ts.Client()

	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{})
	if err != nil {
		t.Fatalf("ListScenes: %v", err)
	}
	var sawErr bool
	for r := range ch {
		if r.Kind == scraper.KindError {
			sawErr = true
		}
	}
	if !sawErr {
		t.Error("expected a bootstrap error when instance_token is missing")
	}
}
