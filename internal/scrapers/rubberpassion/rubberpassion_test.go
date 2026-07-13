package rubberpassion

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/mymemberutil"
	"github.com/Wasylq/FSS/internal/scrapers/testutil"
	"github.com/Wasylq/FSS/scraper"
)

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url  string
		want bool
	}{
		{"https://rubber-passion.com", true},
		{"https://www.rubber-passion.com/", true},
		{"https://rubber-passion.com/19-lola-fucking-toy-pt2", true},
		{"http://rubber-passion.com/005-videos", true},
		{"https://rubberpassion.com", false},
		{"https://rachel-steele.com", false},
		{"https://example.com", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.want {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.want)
		}
	}
}

func TestID(t *testing.T) {
	if got := New().ID(); got != "rubberpassion" {
		t.Errorf("ID() = %q, want rubberpassion", got)
	}
}

// TestKeywordSplit pins the performer/tag split. The site puts performers and
// fetish tags in one comma-joined keywords list, so only the configured
// known-performers map keeps them apart.
func TestKeywordSplit(t *testing.T) {
	keywords := "Latex Lucy, Rebecca Smyth, Catsuit, Bondage, Girl/Girl, Zara Du Rose, Pumped Pussy"

	performers, tags := mymemberutil.SplitKeywords(keywords, mm.Config().KnownPerformers)

	wantPerformers := []string{"Latex Lucy", "Rebecca Smyth", "Zara Du Rose"}
	if !slices.Equal(performers, wantPerformers) {
		t.Errorf("performers = %v, want %v", performers, wantPerformers)
	}
	wantTags := []string{"Catsuit", "Bondage", "Girl/Girl", "Pumped Pussy"}
	if !slices.Equal(tags, wantTags) {
		t.Errorf("tags = %v, want %v", tags, wantTags)
	}
}

// TestKnownPerformersLowercase guards the lookup contract: SplitKeywords
// lowercases each keyword before the map lookup, so a capitalised key would
// silently never match and that performer would be filed as a tag.
func TestKnownPerformersLowercase(t *testing.T) {
	for name := range mm.Config().KnownPerformers {
		if name != lower(name) {
			t.Errorf("known performer %q must be lowercase", name)
		}
	}
}

func lower(s string) string {
	b := []byte(s)
	for i := range b {
		if b[i] >= 'A' && b[i] <= 'Z' {
			b[i] += 'a' - 'A'
		}
	}
	return string(b)
}

func makeTestVideos() []mymemberutil.APIVideo {
	return []mymemberutil.APIVideo{
		{
			ID:                        14,
			Title:                     "Lola Fucking Toy Pt2",
			IsPublished:               true,
			PublishDate:               "2026-07-18T20:08:55.000000Z",
			Duration:                  281,
			ContentMappingID:          19,
			ViewsCount:                1200,
			LikesCount:                40,
			CommentsCount:             3,
			PosterSrc:                 "https://cdn.example.com/thumb1.jpg",
			SystemPreviewVideoFullSrc: "https://cdn.example.com/preview1.mp4",
		},
		{
			ID:               15,
			Title:            "Bathroom Affair - Pt2",
			IsPublished:      true,
			PublishDate:      "2026-07-10T09:00:00.000000Z",
			Duration:         600,
			ContentMappingID: 73,
			PosterSrc:        "https://cdn.example.com/thumb2.jpg",
		},
	}
}

const detailPageHTML = `<!DOCTYPE html><html><head>
<meta property="og:description" content="Latex Lucy plays in the bathroom."/>
<meta property="og:image" content="https://cdn.example.com/og-thumb.jpg"/>
</head><body>
<script>self.__next_f.push([1,"keywords\":\"Latex Lucy, Catsuit, Bondage, Big Tits\""])</script>
</body></html>`

func newTestServer(videos []mymemberutil.APIVideo) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/cancellable-request" {
			page := mymemberutil.VideosPage{
				CurrentPage: 1,
				LastPage:    1,
				Total:       len(videos),
				PerPage:     30,
				Data:        videos,
			}
			outer := struct {
				OK   bool            `json:"ok"`
				Data json.RawMessage `json:"data"`
			}{OK: true}
			outer.Data, _ = json.Marshal(page)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(outer)
			return
		}
		_, _ = w.Write([]byte(detailPageHTML))
	}))
}

// withTestServer points the shared mymemberutil scraper at a stub and restores
// it afterwards — mm is a package-level singleton.
func withTestServer(t *testing.T, videos []mymemberutil.APIVideo) (*Scraper, *httptest.Server) {
	t.Helper()
	ts := newTestServer(videos)
	t.Cleanup(ts.Close)

	s := New()
	origClient, origBase := s.mm.Client, s.mm.SiteBase
	s.mm.Client = ts.Client()
	s.mm.SiteBase = ts.URL
	t.Cleanup(func() { s.mm.Client, s.mm.SiteBase = origClient, origBase })
	return s, ts
}

func TestBuildScene(t *testing.T) {
	s, ts := withTestServer(t, nil)

	scene, err := s.mm.BuildScene(context.Background(), ts.URL, makeTestVideos()[0])
	if err != nil {
		t.Fatal(err)
	}

	if scene.ID != "14" {
		t.Errorf("ID = %q, want 14", scene.ID)
	}
	if scene.SiteID != "rubberpassion" {
		t.Errorf("SiteID = %q", scene.SiteID)
	}
	if scene.Studio != "Rubber Passion" {
		t.Errorf("Studio = %q", scene.Studio)
	}
	if scene.Title != "Lola Fucking Toy Pt2" {
		t.Errorf("Title = %q", scene.Title)
	}
	if scene.Duration != 281 {
		t.Errorf("Duration = %d, want 281", scene.Duration)
	}
	if scene.Views != 1200 {
		t.Errorf("Views = %d", scene.Views)
	}
	if scene.Description != "Latex Lucy plays in the bathroom." {
		t.Errorf("Description = %q", scene.Description)
	}
	// "Latex Lucy" is a known performer; the rest are fetish tags.
	if !slices.Equal(scene.Performers, []string{"Latex Lucy"}) {
		t.Errorf("Performers = %v, want [Latex Lucy]", scene.Performers)
	}
	if !slices.Equal(scene.Tags, []string{"Catsuit", "Bondage", "Big Tits"}) {
		t.Errorf("Tags = %v", scene.Tags)
	}
	if scene.Thumbnail != "https://cdn.example.com/og-thumb.jpg" {
		t.Errorf("Thumbnail = %q", scene.Thumbnail)
	}
	if scene.Date.IsZero() {
		t.Error("Date is zero")
	}
	if scene.ScrapedAt.IsZero() {
		t.Error("ScrapedAt is zero")
	}
}

func TestListScenes(t *testing.T) {
	s, ts := withTestServer(t, makeTestVideos())

	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{Workers: 1})
	if err != nil {
		t.Fatal(err)
	}

	scenes := testutil.CollectScenes(t, ch)
	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2", len(scenes))
	}
}

func TestListScenesKnownIDs(t *testing.T) {
	s, ts := withTestServer(t, makeTestVideos())

	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{
		Workers:  1,
		KnownIDs: map[string]bool{"15": true},
	})
	if err != nil {
		t.Fatal(err)
	}

	scenes, stoppedEarly := testutil.CollectScenesWithStop(t, ch)
	if !stoppedEarly {
		t.Error("expected StoppedEarly signal")
	}
	if len(scenes) != 1 || scenes[0].ID != "14" {
		t.Errorf("got %d scenes, want [14]", len(scenes))
	}
}

func TestContextCancellation(t *testing.T) {
	s, ts := withTestServer(t, makeTestVideos())

	ctx, cancel := context.WithCancel(context.Background())
	ch, err := s.ListScenes(ctx, ts.URL, scraper.ListOpts{Workers: 1})
	if err != nil {
		t.Fatal(err)
	}
	cancel()

	for range ch { //nolint:revive // draining until close is the assertion
	}
}
