package loyalfans

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
	"github.com/Anastylosis/FSS/scraper"
)

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url  string
		want bool
	}{
		{"https://www.loyalfans.com/bettie_bondage", true},
		{"https://loyalfans.com/bettie_bondage", true},
		{"https://www.loyalfans.com/some-creator", true},
		{"https://www.loyalfans.com/bettie_bondage/video/some-slug", false},
		{"https://www.loyalfans.com", false},
		{"https://www.manyvids.com/Profile/123", false},
		{"", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.want {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.want)
		}
	}
}

func TestToScene(t *testing.T) {
	v := video{
		Slug:     "my-scene-123456",
		Title:    "My Scene",
		Content:  "A description<br />with linebreaks #tag1 #tag2",
		Hashtags: []string{"#tag1", "#tag2"},
	}
	v.Owner.Slug = "creator1"
	v.Owner.DisplayName = "Creator One"
	v.CreatedAt.Date = "2026-03-15 12:30:00"
	v.VideoObject.Duration = 900
	v.VideoObject.Poster = "https://cdn.example.com/poster.jpg"
	v.Reactions.Total = 42

	sc := toScene("https://www.loyalfans.com/creator1", "creator1", v)

	if sc.ID != "my-scene-123456" {
		t.Errorf("ID = %q", sc.ID)
	}
	if sc.SiteID != "loyalfans" {
		t.Errorf("SiteID = %q", sc.SiteID)
	}
	if sc.URL != "https://www.loyalfans.com/creator1/video/my-scene-123456" {
		t.Errorf("URL = %q", sc.URL)
	}
	if sc.Title != "My Scene" {
		t.Errorf("Title = %q", sc.Title)
	}
	if sc.Duration != 900 {
		t.Errorf("Duration = %d", sc.Duration)
	}
	if sc.Thumbnail != "https://cdn.example.com/poster.jpg" {
		t.Errorf("Thumbnail = %q", sc.Thumbnail)
	}
	if sc.Studio != "Creator One" {
		t.Errorf("Studio = %q", sc.Studio)
	}
	if sc.Likes != 42 {
		t.Errorf("Likes = %d", sc.Likes)
	}
	if len(sc.Performers) != 1 || sc.Performers[0] != "Creator One" {
		t.Errorf("Performers = %v", sc.Performers)
	}
	if len(sc.Tags) != 2 || sc.Tags[0] != "tag1" || sc.Tags[1] != "tag2" {
		t.Errorf("Tags = %v", sc.Tags)
	}
	if sc.Date.Year() != 2026 || sc.Date.Month() != 3 || sc.Date.Day() != 15 {
		t.Errorf("Date = %v", sc.Date)
	}
	if sc.Description != "A description\nwith linebreaks" {
		t.Errorf("Description = %q", sc.Description)
	}
}

func TestStripHashtags(t *testing.T) {
	cases := []struct {
		input, want string
	}{
		{"hello #tag1 world #tag2", "hello  world "},
		{"no tags here", "no tags here"},
		{"#only #tags", " "},
		{"", ""},
	}
	for _, c := range cases {
		if got := stripHashtags(c.input); got != c.want {
			t.Errorf("stripHashtags(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

func makeVideos(slug string, n int) []video {
	videos := make([]video, n)
	for i := range n {
		v := video{
			Slug:     fmt.Sprintf("scene-%d", i+1),
			Title:    fmt.Sprintf("Scene %d", i+1),
			Hashtags: []string{},
		}
		v.Owner.Slug = slug
		v.Owner.DisplayName = "Test Creator"
		v.CreatedAt.Date = "2026-01-15 12:00:00"
		v.VideoObject.Duration = 600
		videos[i] = v
	}
	return videos
}

func newTestServer(pages [][]video) *httptest.Server {
	pageIdx := 0
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Path == "/api/v2/system-status" {
			http.SetCookie(w, &http.Cookie{Name: "session", Value: "test"})
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
			return
		}

		if r.URL.Path == "/api/v2/advanced-search" {
			if pageIdx >= len(pages) {
				_ = json.NewEncoder(w).Encode(searchResponse{Success: true})
				return
			}
			data := pages[pageIdx]
			pageIdx++

			var nextToken *string
			if pageIdx < len(pages) {
				tok := "next-page-token"
				nextToken = &tok
			}
			_ = json.NewEncoder(w).Encode(searchResponse{
				Success:   true,
				Data:      data,
				PageToken: nextToken,
			})
			return
		}

		http.NotFound(w, r)
	}))
}

func TestListScenes(t *testing.T) {
	slug := "test_creator"
	page1 := makeVideos(slug, 3)

	ts := newTestServer([][]video{page1})
	defer ts.Close()

	s := &Scraper{client: ts.Client(), base: ts.URL}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/test_creator", scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}

	scenes := testutil.CollectScenes(t, ch)
	if len(scenes) != 3 {
		t.Fatalf("got %d scenes, want 3", len(scenes))
	}
	if scenes[0].Title != "Scene 1" {
		t.Errorf("first scene title = %q", scenes[0].Title)
	}
	if scenes[2].Title != "Scene 3" {
		t.Errorf("last scene title = %q", scenes[2].Title)
	}
}

func TestListScenesPagination(t *testing.T) {
	slug := "test_creator"
	page1 := makeVideos(slug, 20)
	page2 := makeVideos(slug, 5)
	// Offset page2 slugs so they don't duplicate page1.
	for i := range page2 {
		page2[i].Slug = fmt.Sprintf("scene-%d", 21+i)
		page2[i].Title = fmt.Sprintf("Scene %d", 21+i)
	}

	ts := newTestServer([][]video{page1, page2})
	defer ts.Close()

	s := &Scraper{client: ts.Client(), base: ts.URL}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/test_creator", scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}

	scenes := testutil.CollectScenes(t, ch)
	if len(scenes) != 25 {
		t.Fatalf("got %d scenes, want 25", len(scenes))
	}
}

func TestListScenesKnownIDs(t *testing.T) {
	slug := "test_creator"
	page1 := makeVideos(slug, 5)

	ts := newTestServer([][]video{page1})
	defer ts.Close()

	s := &Scraper{client: ts.Client(), base: ts.URL}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/test_creator", scraper.ListOpts{
		KnownIDs: map[string]bool{"scene-3": true},
	})
	if err != nil {
		t.Fatal(err)
	}

	scenes, stoppedEarly := testutil.CollectScenesWithStop(t, ch)
	if !stoppedEarly {
		t.Error("expected StoppedEarly signal")
	}
	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2 (early stop at known ID)", len(scenes))
	}
	if scenes[0].ID != "scene-1" || scenes[1].ID != "scene-2" {
		t.Errorf("scenes = %v", scenes)
	}
}

func TestListScenesFiltersOwner(t *testing.T) {
	slug := "test_creator"
	videos := makeVideos(slug, 2)
	// Add a video from a different creator.
	other := video{Slug: "other-1", Title: "Other Scene"}
	other.Owner.Slug = "someone_else"
	other.Owner.DisplayName = "Someone Else"
	videos = append(videos, other)

	ts := newTestServer([][]video{videos})
	defer ts.Close()

	s := &Scraper{client: ts.Client(), base: ts.URL}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/test_creator", scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}

	scenes := testutil.CollectScenes(t, ch)
	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2 (other creator filtered out)", len(scenes))
	}
}

// The store URL is what StashDB records for LoyalFans creators. Before it was
// matched, slugFromURL's last-segment fallback would have turned it into the
// creator slug "store".
func TestMatchesStoreURL(t *testing.T) {
	s := New()
	cases := map[string]string{
		"https://www.loyalfans.com/goddexxdaphne":       "goddexxdaphne",
		"https://www.loyalfans.com/goddexxdaphne/store": "goddexxdaphne",
		"https://loyalfans.com/goddexxdaphne/store/":    "goddexxdaphne",
		"https://www.loyalfans.com/goddexxdaphne/":      "goddexxdaphne",
	}
	for u, wantSlug := range cases {
		if !s.MatchesURL(u) {
			t.Errorf("MatchesURL(%q) = false, want true", u)
			continue
		}
		if got := slugFromURL(u); got != wantSlug {
			t.Errorf("slugFromURL(%q) = %q, want %q", u, got, wantSlug)
		}
	}

	for _, u := range []string{
		"https://www.loyalfans.com/goddexxdaphne/store/extra",
		"https://example.com/goddexxdaphne/store",
	} {
		if s.MatchesURL(u) {
			t.Errorf("MatchesURL(%q) = true, want false", u)
		}
	}
}

// --- golden fixture ----------------------------------------------------------
//
// The other tests build video/searchResponse values in Go, so encode and decode
// share the struct tag and a renamed one round-trips unnoticed. This is a
// byte-verbatim slice of a live POST to
// https://www.loyalfans.com/api/v2/advanced-search (first two videos; success,
// httpCode and page_token copied from the same body).
//
// Two steps, no account: POST /api/v2/system-status to pick up the XSRF and AWS
// load-balancer cookies, then send them with the search. None of those cookies
// appears in the response — checked, not assumed.
//
// **About the retained `page_token`.** It is double-base64 wrapping a Laravel
// encrypted payload (`{"iv":…,"value":…,"mac":…}`) — ciphertext with a MAC, not a
// plaintext secret, and inert without the site's APP_KEY. It is kept verbatim
// because loyalfans is the one cursor-paginated scraper in the suite and the
// *shape* of this field is the thing worth pinning: a non-null string means "more
// pages", and the walk ends when it is null. Truncating it would make this file
// stop being a capture, which is the property every other fixture here relies on.
//
// Shapes a hand-written fixture would have got wrong:
//   - **`created_at` is an object, not a string**: `{"date":"2026-07-14 14:47:40",
//     "timezone_type":3,"timezone":"UTC"}` — PHP's DateTime serialisation. The
//     inner date has a **space separator and no zone marker**, so it needs its own
//     layout rather than RFC3339.
//   - `video_object` nests `poster` and `duration`; duration is seconds.
//   - `uid` is an opaque 45-character token and is the scene ID — it is not
//     numeric and not derived from the slug.
//   - `short_url` carries escaped forward slashes (`\/`), evidence the body is a
//     capture rather than a re-encode.
func TestGoldenAdvancedSearch(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("testdata", "advanced_search.json"))
	if err != nil {
		t.Fatal(err)
	}

	var resp searchResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("decoding captured payload: %v", err)
	}
	if !resp.Success {
		t.Fatal("success is false (success)")
	}
	if len(resp.Data) != 2 {
		t.Fatalf("decoded %d videos, want 2", len(resp.Data))
	}

	// The cursor: a non-nil pointer means there is another page.
	if resp.PageToken == nil {
		t.Fatal("PageToken is nil (page_token) — the cursor walk would stop after page 1")
	}
	if *resp.PageToken == "" {
		t.Error("PageToken is the empty string; the walk treats non-nil as 'more pages'")
	}

	v := resp.Data[0]
	if v.UID == "" {
		t.Error("UID is empty (uid) — it is the scene ID")
	}
	if len(v.UID) < 20 {
		t.Errorf("UID = %q (uid) is unexpectedly short; it is an opaque token, not a number", v.UID)
	}
	if v.Slug != "aftermath-1646650364673" {
		t.Errorf("Slug = %q (slug)", v.Slug)
	}
	if v.Owner.Slug != "bettie_bondage" {
		t.Errorf("Owner.Slug = %q (owner.slug) — the scene URL is built from it", v.Owner.Slug)
	}
	if v.VideoObject.Poster == "" {
		t.Error("VideoObject.Poster is empty (video_object.poster) — every scene loses its thumbnail")
	}
	if v.VideoObject.Duration == 0 {
		t.Error("VideoObject.Duration is 0 (video_object.duration)")
	}
	if v.Reactions.Total == 0 {
		t.Error("Reactions.Total is 0 (reactions.total)")
	}

	// created_at as a nested object with a space-separated date.
	if v.CreatedAt.Date != "2026-07-14 14:47:40" {
		t.Errorf("CreatedAt.Date = %q (created_at.date), want a space-separated timestamp — "+
			"created_at is an object, not a string", v.CreatedAt.Date)
	}
	if _, err := time.Parse(dateFormat, v.CreatedAt.Date); err != nil {
		t.Errorf("created_at.date %q does not parse with dateFormat %q: %v",
			v.CreatedAt.Date, dateFormat, err)
	}
}

func TestGoldenAdvancedSearchCarriesNoSessionCookie(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("testdata", "advanced_search.json"))
	if err != nil {
		t.Fatal(err)
	}
	// The XSRF and AWS load-balancer cookies travel in the request; none of them
	// may appear in a committed body. page_token is deliberately exempt — see the
	// fixture comment for why it is safe and why it is kept.
	for _, marker := range []string{"XSRF-TOKEN", "AWSALB", "Set-Cookie", "laravel_session"} {
		if bytes.Contains(body, []byte(marker)) {
			t.Errorf("fixture contains %q — re-capture without the session material", marker)
		}
	}
	if !bytes.Contains(body, []byte(`\/`)) {
		t.Error(`fixture lost the escaped forward slashes (\/) — it looks re-encoded`)
	}
	if !bytes.Contains(body, []byte(`"created_at":{"date":`)) {
		t.Error(`fixture lost the created_at object form; a re-encode may have flattened it`)
	}
}

// TestListScenesSkipsRepeatsAcrossPages reproduces what the live cursor API
// does: hand back videos already returned on an earlier page. A saved catalogue
// had 187 of 493 videos emitted twice, differing only in when they were
// scraped. Re-sending them inflates the live progress count and repeats
// downstream work, so the walk must emit each video once.
func TestListScenesSkipsRepeatsAcrossPages(t *testing.T) {
	slug := "test_creator"
	page1 := makeVideos(slug, 20)

	// Page 2 overlaps page 1 by half, then adds five genuinely new videos —
	// the shape a shifting window produces.
	page2 := make([]video, 0, 15)
	page2 = append(page2, page1[10:]...) // 10 repeats
	fresh := makeVideos(slug, 5)
	for i := range fresh {
		fresh[i].Slug = fmt.Sprintf("scene-%d", 21+i)
		fresh[i].Title = fmt.Sprintf("Scene %d", 21+i)
	}
	page2 = append(page2, fresh...)

	ts := newTestServer([][]video{page1, page2})
	defer ts.Close()

	s := &Scraper{client: ts.Client(), base: ts.URL}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/test_creator", scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}

	scenes := testutil.CollectScenes(t, ch)
	if len(scenes) != 25 {
		t.Fatalf("got %d scenes, want 25 (20 + 5 new; 10 repeats dropped)", len(scenes))
	}

	seen := map[string]bool{}
	for _, sc := range scenes {
		if seen[sc.ID] {
			t.Errorf("scene %q emitted more than once", sc.ID)
		}
		seen[sc.ID] = true
	}
}
