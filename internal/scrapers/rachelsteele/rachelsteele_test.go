package rachelsteele

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wasylq/FSS/scraper"
)

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url  string
		want bool
	}{
		{"https://rachel-steele.com", true},
		{"https://www.rachel-steele.com/005-videos", true},
		{"https://rachel-steele.com/4943-milf1928-breakfast-fuck-3", true},
		{"https://www.manyvids.com/Profile/123/foo", false},
		{"https://example.com", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.want {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.want)
		}
	}
}

func TestSlugify(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"MILF1928 - Breakfast Fuck 3", "milf1928-breakfast-fuck-3"},
		{"MILF1878 - Cucked Stepson", "milf1878-cucked-stepson"},
		{"Fetish186 - Jerk Off Encouragement, Rachel and Addie", "fetish186-jerk-off-encouragement-rachel-and-addie"},
		{"Simple Title", "simple-title"},
	}
	for _, c := range cases {
		if got := slugify(c.input); got != c.want {
			t.Errorf("slugify(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

func TestParseDate(t *testing.T) {
	cases := []struct {
		input string
		year  int
		month time.Month
		day   int
	}{
		{"2026-04-17T16:00:13.000000Z", 2026, 4, 17},
		{"2025-12-08T23:45:22.000000Z", 2025, 12, 8},
		{"", 1, 1, 1},
	}
	for _, c := range cases {
		d := parseDate(c.input)
		if c.input == "" {
			if !d.IsZero() {
				t.Errorf("parseDate(%q) should be zero", c.input)
			}
			continue
		}
		if d.Year() != c.year || d.Month() != c.month || d.Day() != c.day {
			t.Errorf("parseDate(%q) = %v", c.input, d)
		}
	}
}

func TestSplitKeywords(t *testing.T) {
	kw := "Rachel Steele, Tyler Cruise, 4K, B/G, Big Ass, MILF, Step-Mother Fantasy"
	performers, tags := splitKeywords(kw)

	if len(performers) != 2 || performers[0] != "Rachel Steele" || performers[1] != "Tyler Cruise" {
		t.Errorf("performers = %v", performers)
	}
	if len(tags) != 5 || tags[0] != "4K" || tags[4] != "Step-Mother Fantasy" {
		t.Errorf("tags = %v", tags)
	}
}

func TestSplitKeywordsEmpty(t *testing.T) {
	performers, tags := splitKeywords("")
	if len(performers) != 0 || len(tags) != 0 {
		t.Errorf("expected empty, got performers=%v tags=%v", performers, tags)
	}
}

func TestVideoPrice(t *testing.T) {
	cases := []struct {
		name  string
		price any
		want  float64
	}{
		{"float", 24.99, 24.99},
		{"string", "19.99", 19.99},
		{"nil", nil, 0},
	}
	for _, c := range cases {
		vid := apiVideo{StreamPrice: c.price}
		if got := vid.price(); got != c.want {
			t.Errorf("%s: price() = %v, want %v", c.name, got, c.want)
		}
	}
}

func makeTestVideos() []apiVideo {
	return []apiVideo{
		{
			ID:                        100,
			Title:                     "MILF100 - Test Scene One",
			IsPublished:               true,
			PublishDate:               "2026-04-01T12:00:00.000000Z",
			Duration:                  900,
			ContentMappingID:          101,
			ViewsCount:                500,
			LikesCount:                20,
			CommentsCount:             5,
			StreamPrice:               19.99,
			PosterSrc:                 "https://cdn.example.com/thumb1.jpg",
			SystemPreviewVideoFullSrc: "https://cdn.example.com/preview1.mp4",
		},
		{
			ID:                        200,
			Title:                     "MILF200 - Test Scene Two",
			IsPublished:               true,
			PublishDate:               "2026-03-15T10:00:00.000000Z",
			Duration:                  1200,
			ContentMappingID:          201,
			ViewsCount:                300,
			LikesCount:                10,
			StreamPrice:               "24.99",
			PosterSrc:                 "https://cdn.example.com/thumb2.jpg",
			SystemPreviewVideoFullSrc: "https://cdn.example.com/preview2.mp4",
			Has4K:                     true,
		},
	}
}

const detailPageTemplate = `<!DOCTYPE html><html><head>
<meta property="og:description" content="Scene description for testing."/>
<meta property="og:image" content="https://cdn.example.com/og-thumb.jpg"/>
</head><body>
<script>self.__next_f.push([1,"keywords\":\"Rachel Steele, Tyler Cruise, 4K, MILF, Step-Mother Fantasy\""])</script>
</body></html>`

func newTestServer(videos []apiVideo) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case apiPath:
			page := videosPage{
				CurrentPage: 1,
				LastPage:    1,
				Total:       len(videos),
				PerPage:     30,
				Data:        videos,
			}
			outer := apiResponse{OK: true}
			outer.Data, _ = json.Marshal(page)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(outer)
		default:
			_, _ = w.Write([]byte(detailPageTemplate))
		}
	}))
}

func TestFetchPage(t *testing.T) {
	videos := makeTestVideos()
	ts := newTestServer(videos)
	defer ts.Close()

	s := &Scraper{client: ts.Client(), siteBase: ts.URL}
	vp, err := s.fetchPage(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}

	if vp.Total != 2 {
		t.Errorf("Total = %d, want 2", vp.Total)
	}
	if len(vp.Data) != 2 {
		t.Fatalf("got %d items, want 2", len(vp.Data))
	}
	if vp.Data[0].Title != "MILF100 - Test Scene One" {
		t.Errorf("Title = %q", vp.Data[0].Title)
	}
}

func TestFetchDetail(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(detailPageTemplate))
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client(), siteBase: ts.URL}
	detail, err := s.fetchDetail(context.Background(), ts.URL+"/101-milf100-test-scene-one")
	if err != nil {
		t.Fatal(err)
	}

	if detail.description != "Scene description for testing." {
		t.Errorf("description = %q", detail.description)
	}
	if detail.thumbnail != "https://cdn.example.com/og-thumb.jpg" {
		t.Errorf("thumbnail = %q", detail.thumbnail)
	}
	if len(detail.performers) != 2 || detail.performers[0] != "Rachel Steele" || detail.performers[1] != "Tyler Cruise" {
		t.Errorf("performers = %v", detail.performers)
	}
	if len(detail.tags) != 3 || detail.tags[0] != "4K" {
		t.Errorf("tags = %v", detail.tags)
	}
}

func TestBuildScene(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(detailPageTemplate))
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client(), siteBase: ts.URL}
	vid := makeTestVideos()[1]

	scene, err := s.buildScene(context.Background(), ts.URL, vid)
	if err != nil {
		t.Fatal(err)
	}

	if scene.ID != "200" {
		t.Errorf("ID = %q", scene.ID)
	}
	if scene.SiteID != "rachelsteele" {
		t.Errorf("SiteID = %q", scene.SiteID)
	}
	if scene.Title != "MILF200 - Test Scene Two" {
		t.Errorf("Title = %q", scene.Title)
	}
	if scene.Duration != 1200 {
		t.Errorf("Duration = %d", scene.Duration)
	}
	if scene.Width != 3840 || scene.Height != 2160 {
		t.Errorf("Resolution: %dx%d", scene.Width, scene.Height)
	}
	if scene.Resolution != "2160p" {
		t.Errorf("Resolution = %q", scene.Resolution)
	}
	if scene.Studio != studioName {
		t.Errorf("Studio = %q", scene.Studio)
	}
	if scene.Views != 300 {
		t.Errorf("Views = %d", scene.Views)
	}
	if scene.Description != "Scene description for testing." {
		t.Errorf("Description = %q", scene.Description)
	}
	if len(scene.Performers) != 2 {
		t.Errorf("Performers = %v", scene.Performers)
	}
	if len(scene.Tags) != 3 {
		t.Errorf("Tags = %v", scene.Tags)
	}
	if scene.Thumbnail != "https://cdn.example.com/og-thumb.jpg" {
		t.Errorf("Thumbnail = %q", scene.Thumbnail)
	}
	if len(scene.PriceHistory) != 1 || scene.PriceHistory[0].Regular != 24.99 {
		t.Errorf("PriceHistory = %v", scene.PriceHistory)
	}
}

func TestListScenes(t *testing.T) {
	videos := makeTestVideos()
	ts := newTestServer(videos)
	defer ts.Close()

	s := &Scraper{client: ts.Client(), siteBase: ts.URL}
	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{Workers: 1})
	if err != nil {
		t.Fatal(err)
	}

	var scenes []string
	for r := range ch {
		if r.Total > 0 || r.StoppedEarly {
			continue
		}
		if r.Err != nil {
			t.Errorf("unexpected error: %v", r.Err)
			continue
		}
		scenes = append(scenes, r.Scene.Title)
	}

	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2", len(scenes))
	}
}

func TestListScenesKnownIDs(t *testing.T) {
	videos := makeTestVideos()
	ts := newTestServer(videos)
	defer ts.Close()

	s := &Scraper{client: ts.Client(), siteBase: ts.URL}
	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{
		Workers:  1,
		KnownIDs: map[string]bool{"200": true},
	})
	if err != nil {
		t.Fatal(err)
	}

	var scenes []string
	var stoppedEarly bool
	for r := range ch {
		if r.Total > 0 {
			continue
		}
		if r.StoppedEarly {
			stoppedEarly = true
			continue
		}
		if r.Err != nil {
			t.Errorf("unexpected error: %v", r.Err)
			continue
		}
		scenes = append(scenes, r.Scene.ID)
	}

	if !stoppedEarly {
		t.Error("expected StoppedEarly signal")
	}
	if len(scenes) != 1 || scenes[0] != "100" {
		t.Errorf("got scenes %v, want [100]", scenes)
	}
}

func TestListScenesSkipsUnpublished(t *testing.T) {
	videos := []apiVideo{
		{ID: 1, Title: "Published", IsPublished: true, PublishDate: "2026-01-01T00:00:00.000000Z", ContentMappingID: 1},
		{ID: 2, Title: "Unpublished", IsPublished: false, PublishDate: "2026-01-02T00:00:00.000000Z", ContentMappingID: 2},
	}
	ts := newTestServer(videos)
	defer ts.Close()

	s := &Scraper{client: ts.Client(), siteBase: ts.URL}
	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{Workers: 1})
	if err != nil {
		t.Fatal(err)
	}

	var count int
	for r := range ch {
		if r.Total > 0 || r.StoppedEarly {
			continue
		}
		if r.Err != nil {
			t.Errorf("unexpected error: %v", r.Err)
			continue
		}
		count++
		if r.Scene.Title != "Published" {
			t.Errorf("got %q, expected only Published", r.Scene.Title)
		}
	}

	if count != 1 {
		t.Errorf("got %d scenes, want 1 (unpublished should be skipped)", count)
	}
}

func TestMultiPage(t *testing.T) {
	pageData := map[int][]apiVideo{
		1: {
			{ID: 1, Title: "Scene A", IsPublished: true, PublishDate: "2026-04-01T00:00:00.000000Z", ContentMappingID: 1},
		},
		2: {
			{ID: 2, Title: "Scene B", IsPublished: true, PublishDate: "2026-03-01T00:00:00.000000Z", ContentMappingID: 2},
		},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case apiPath:
			args := r.URL.Query().Get("args")
			page := 1
			for p := 1; p <= 3; p++ {
				expected, _ := json.Marshal([]any{[]string{fmt.Sprintf("page=%d", p)}})
				if args == string(expected) {
					page = p
					break
				}
			}
			videos := pageData[page]
			lastPage := 2
			if page > 2 {
				videos = nil
			}
			vp := videosPage{
				CurrentPage: page,
				LastPage:    lastPage,
				Total:       2,
				PerPage:     1,
				Data:        videos,
			}
			outer := apiResponse{OK: true}
			outer.Data, _ = json.Marshal(vp)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(outer)
		default:
			_, _ = w.Write([]byte(detailPageTemplate))
		}
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client(), siteBase: ts.URL}
	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{Workers: 1})
	if err != nil {
		t.Fatal(err)
	}

	var scenes []string
	for r := range ch {
		if r.Total > 0 || r.StoppedEarly {
			continue
		}
		if r.Err != nil {
			t.Errorf("unexpected error: %v", r.Err)
			continue
		}
		scenes = append(scenes, r.Scene.Title)
	}

	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2: %v", len(scenes), scenes)
	}
}
