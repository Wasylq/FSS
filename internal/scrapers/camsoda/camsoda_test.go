package camsoda

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
	"github.com/Wasylq/FSS/scraper"
)

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url  string
		want bool
	}{
		{"https://www.camsoda.com", true},
		{"https://www.camsoda.com/", true},
		{"https://camsoda.com/exclusive-videos", true},
		{"https://www.camsoda.com/exclusive-videos/", true},
		{"https://www.camsoda.com/autumnfalls", true},
		{"https://www.camsoda.com/gabbiecarter00", true},
		{"https://www.camsoda.com/nicollemeyer/bio", true},
		{"https://www.camsoda.com/nicollemeyer/media", true},

		// A single scene, not a listing.
		{"https://www.camsoda.com/nicollemeyer/media/braces/6298032", false},
		{"https://www.camsoda.com/exclusive-videos/black_tape", false},
		{"https://www.xxxfollow.com/katanakombat", false},
		{"", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.want {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.want)
		}
	}
}

// Model profiles live at the site root, sharing a namespace with the site's
// own routes, so those must not be claimed as models.
func TestMatchesURLRejectsReservedSegments(t *testing.T) {
	s := New()
	for _, u := range []string{
		"https://www.camsoda.com/support",
		"https://www.camsoda.com/login",
		"https://www.camsoda.com/girls",
		"https://www.camsoda.com/men",
		"https://www.camsoda.com/trans",
		"https://www.camsoda.com/porn",
		"https://www.camsoda.com/media",
		"https://www.camsoda.com/most-liked",
		"https://www.camsoda.com/speed-date",
		"https://www.camsoda.com/voyeur-house",
		"https://www.camsoda.com/de",
		"https://www.camsoda.com/pt-br",
		"https://www.camsoda.com/Support",
	} {
		if s.MatchesURL(u) {
			t.Errorf("MatchesURL(%q) = true, want false", u)
		}
	}
}

func TestParseURL(t *testing.T) {
	cases := []struct {
		url      string
		wantMode mode
		wantUser string
	}{
		{"https://www.camsoda.com", modeExclusive, ""},
		{"https://www.camsoda.com/", modeExclusive, ""},
		{"https://www.camsoda.com/exclusive-videos", modeExclusive, ""},
		{"https://camsoda.com/exclusive-videos/", modeExclusive, ""},
		{"https://www.camsoda.com/autumnfalls", modeModel, "autumnfalls"},
		{"https://www.camsoda.com/nicollemeyer/media", modeModel, "nicollemeyer"},
		{"https://www.camsoda.com/nicollemeyer/bio", modeModel, "nicollemeyer"},
		{"https://www.camsoda.com/support", modeNone, ""},
		// Test-server fallback: host is not camsoda.com.
		{"http://127.0.0.1:1234/exclusive-videos", modeExclusive, ""},
		{"http://127.0.0.1:1234/nicollemeyer/media", modeModel, "nicollemeyer"},
	}
	for _, c := range cases {
		m, user := parseURL(c.url)
		if m != c.wantMode || user != c.wantUser {
			t.Errorf("parseURL(%q) = (%v, %q), want (%v, %q)", c.url, m, user, c.wantMode, c.wantUser)
		}
	}
}

// --- Exclusive videos -------------------------------------------------------

func exclusivePage(videos []exclusiveVideo) string {
	state := preloadedState{}
	state.ExclusiveVideos.VideoList = videos
	b, err := json.Marshal(state)
	if err != nil {
		panic(err)
	}
	// Wrapped the way the real page ships it, amid other markup.
	return `<!DOCTYPE html><html><body><div id="root"></div>` +
		`<script type="application/json" id="__PRELOADED_STATE__">` + string(b) + `</script>` +
		`<script src="/static/js/main.js"></script></body></html>`
}

func newExclusiveServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/exclusive-videos" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, body)
	}))
}

func TestExclusiveVideos(t *testing.T) {
	v := exclusiveVideo{
		ID:          "veronica-rodriguez-first-time-anal",
		Title:       "Veronica Rodriguez First Time Anal",
		Desc:        "VRod lets us debut her ass to the world.",
		ThumbName:   "https://public-media.example/50/thumb.jpg",
		VideoName:   "https://public-media.example/50/video.mp4",
		VideoWidth:  1280,
		VideoHeight: 720,
	}
	v2 := exclusiveVideo{ID: "sexdoll_threesome", Title: "SexDoll Threesome"}
	v2.Models = []struct {
		Name     string `json:"name"`
		Username string `json:"username"`
	}{{Name: "Vanessa Veracruz", Username: "msveracruzxxx"}, {Name: "Daisy Marie"}}

	ts := newExclusiveServer(t, exclusivePage([]exclusiveVideo{v, v2}))
	defer ts.Close()

	s := &Scraper{client: ts.Client(), base: ts.URL}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/exclusive-videos", scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}
	scenes := testutil.CollectScenes(t, ch)
	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2", len(scenes))
	}

	sc := scenes[0]
	if sc.ID != v.ID {
		t.Errorf("ID = %q", sc.ID)
	}
	if sc.SiteID != "camsoda" {
		t.Errorf("SiteID = %q", sc.SiteID)
	}
	if sc.Title != v.Title {
		t.Errorf("Title = %q", sc.Title)
	}
	if want := ts.URL + "/exclusive-videos/" + v.ID; sc.URL != want {
		t.Errorf("URL = %q, want %q", sc.URL, want)
	}
	if sc.Thumbnail != v.ThumbName || sc.Preview != v.VideoName {
		t.Errorf("media = %q / %q", sc.Thumbnail, sc.Preview)
	}
	if sc.Width != 1280 || sc.Height != 720 {
		t.Errorf("dimensions = %dx%d", sc.Width, sc.Height)
	}
	if sc.Studio != "CamSoda Exclusive Videos" {
		t.Errorf("Studio = %q", sc.Studio)
	}
	// The collection publishes no dates or runtimes.
	if !sc.Date.IsZero() || sc.Duration != 0 {
		t.Errorf("expected no date/duration, got %v / %d", sc.Date, sc.Duration)
	}
	if len(sc.Performers) != 0 {
		t.Errorf("Performers = %v, want none", sc.Performers)
	}

	if got := scenes[1].Performers; len(got) != 2 || got[0] != "Vanessa Veracruz" || got[1] != "Daisy Marie" {
		t.Errorf("Performers = %v", got)
	}
}

// The bare domain is the live-cam directory with no catalogue, so it resolves
// to the exclusive-videos collection rather than failing.
func TestBareDomainScrapesExclusiveVideos(t *testing.T) {
	ts := newExclusiveServer(t, exclusivePage([]exclusiveVideo{{ID: "a", Title: "A"}}))
	defer ts.Close()

	s := &Scraper{client: ts.Client(), base: ts.URL}
	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if scenes := testutil.CollectScenes(t, ch); len(scenes) != 1 {
		t.Fatalf("got %d scenes, want 1", len(scenes))
	}
}

func TestExclusiveVideosMissingStateIsAnError(t *testing.T) {
	ts := newExclusiveServer(t, "<!DOCTYPE html><html><body>no state here</body></html>")
	defer ts.Close()

	s := &Scraper{client: ts.Client(), base: ts.URL}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/exclusive-videos", scraper.ListOpts{})
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
		t.Error("expected an error when __PRELOADED_STATE__ is absent")
	}
}

// --- Model media ------------------------------------------------------------

func newModelServer(t *testing.T, username string, resp modelMediaResponse) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/user/"+username+"/media" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
}

func TestModelMedia(t *testing.T) {
	resp := modelMediaResponse{}
	resp.User.Username = "nicollemeyer"
	resp.User.DisplayName = "nicollemeyer"
	resp.MediaList = []mediaItem{
		{
			ID: 6298032, Name: "Braces", Slug: "braces",
			Description: "A description", TokenPrice: 25, Status: "approved",
			CreatedAt: "2019-06-23 20:14:22", Duration: 312, IsVideo: true,
			ThumbnailURL: "https://media-secure.example/6298032.jpg",
			TypeName:     "videos", Username: "nicollemeyer", DisplayName: "Nicolle Meyer",
		},
		// A picture set: present in the library, not a scene.
		{
			ID: 16524012, Name: "Wednesday cosplay", Slug: "wednesday-cosplay",
			CreatedAt: "2026-01-24 15:17:02", Duration: 0, IsVideo: false,
			TypeName: "pictures", Username: "nicollemeyer",
		},
	}

	ts := newModelServer(t, "nicollemeyer", resp)
	defer ts.Close()

	s := &Scraper{client: ts.Client(), base: ts.URL}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/nicollemeyer/media", scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}
	scenes := testutil.CollectScenes(t, ch)
	if len(scenes) != 1 {
		t.Fatalf("got %d scenes, want 1 (the picture set must be dropped)", len(scenes))
	}

	sc := scenes[0]
	if sc.ID != "6298032" {
		t.Errorf("ID = %q", sc.ID)
	}
	if sc.Title != "Braces" {
		t.Errorf("Title = %q", sc.Title)
	}
	if want := ts.URL + "/nicollemeyer/media/braces/6298032"; sc.URL != want {
		t.Errorf("URL = %q, want %q", sc.URL, want)
	}
	if sc.Date.Format("2006-01-02T15:04:05Z") != "2019-06-23T20:14:22Z" {
		t.Errorf("Date = %v", sc.Date)
	}
	if sc.Duration != 312 {
		t.Errorf("Duration = %d", sc.Duration)
	}
	if sc.Studio != "Nicolle Meyer" || len(sc.Performers) != 1 || sc.Performers[0] != "Nicolle Meyer" {
		t.Errorf("credits = %q / %v", sc.Studio, sc.Performers)
	}
	// token_price is denominated in CamSoda tokens, so it is deliberately not
	// recorded as a USD PriceSnapshot.
	if len(sc.PriceHistory) != 0 || sc.LowestPrice != 0 {
		t.Errorf("expected no price data, got %+v / %v", sc.PriceHistory, sc.LowestPrice)
	}
}

// The library is not strictly date-ordered and arrives in a single response,
// so a known ID must neither stop the scrape nor drop later scenes.
func TestModelMediaIgnoresKnownIDs(t *testing.T) {
	resp := modelMediaResponse{}
	resp.User.Username = "gabbiecarter00"
	for i := 1; i <= 4; i++ {
		resp.MediaList = append(resp.MediaList, mediaItem{
			ID: i, Name: fmt.Sprintf("Video %d", i), Slug: fmt.Sprintf("v%d", i),
			CreatedAt: "2024-01-0" + fmt.Sprint(i) + " 10:00:00", IsVideo: true,
			Username: "gabbiecarter00", DisplayName: "Gabbie Carter",
		})
	}

	ts := newModelServer(t, "gabbiecarter00", resp)
	defer ts.Close()

	s := &Scraper{client: ts.Client(), base: ts.URL}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/gabbiecarter00", scraper.ListOpts{
		KnownIDs: map[string]bool{"2": true},
	})
	if err != nil {
		t.Fatal(err)
	}
	scenes, stoppedEarly := testutil.CollectScenesWithStop(t, ch)
	if stoppedEarly {
		t.Error("single-response listing must not report an early stop")
	}
	if len(scenes) != 4 {
		t.Fatalf("got %d scenes, want 4", len(scenes))
	}
}

func TestModelMediaTitleAndURLFallbacks(t *testing.T) {
	resp := modelMediaResponse{}
	resp.User.Username = "autumnfalls"
	resp.MediaList = []mediaItem{
		{ID: 77, Name: "", Slug: "", IsVideo: true, Username: "autumnfalls"},
	}

	ts := newModelServer(t, "autumnfalls", resp)
	defer ts.Close()

	s := &Scraper{client: ts.Client(), base: ts.URL}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/autumnfalls", scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}
	scenes := testutil.CollectScenes(t, ch)
	if len(scenes) != 1 {
		t.Fatalf("got %d scenes, want 1", len(scenes))
	}
	if scenes[0].Title != "Video 77" {
		t.Errorf("Title = %q, want the id fallback", scenes[0].Title)
	}
	// With no slug there is no item route, so the library page is the target.
	if want := ts.URL + "/autumnfalls/media"; scenes[0].URL != want {
		t.Errorf("URL = %q, want %q", scenes[0].URL, want)
	}
}

// The detail fetch that CamSoda's rate limiter punishes is avoided entirely:
// each mode issues exactly one request.
func TestScrapeIssuesOneRequest(t *testing.T) {
	var hits int
	resp := modelMediaResponse{}
	resp.User.Username = "autumnfalls"
	resp.MediaList = []mediaItem{
		{ID: 1, Name: "A", Slug: "a", IsVideo: true, Username: "autumnfalls"},
		{ID: 2, Name: "B", Slug: "b", IsVideo: true, Username: "autumnfalls"},
		{ID: 3, Name: "C", Slug: "c", IsVideo: true, Username: "autumnfalls"},
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.URL.Path != "/api/v1/user/autumnfalls/media" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client(), base: ts.URL}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/autumnfalls", scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if scenes := testutil.CollectScenes(t, ch); len(scenes) != 3 {
		t.Fatalf("got %d scenes, want 3", len(scenes))
	}
	if hits != 1 {
		t.Errorf("made %d requests, want exactly 1", hits)
	}
}

func TestUnsupportedURLIsRejected(t *testing.T) {
	s := New()
	for _, u := range []string{
		"https://www.camsoda.com/support",
		"https://www.camsoda.com/nicollemeyer/media/braces/6298032",
	} {
		if _, err := s.ListScenes(context.Background(), u, scraper.ListOpts{}); err == nil {
			t.Errorf("ListScenes(%q) = nil error, want a rejection", u)
		} else if !strings.Contains(err.Error(), "camsoda") {
			t.Errorf("error = %v, want it to name the scraper", err)
		}
	}
}
