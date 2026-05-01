package nakednews

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wasylq/FSS/scraper"
)

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func sampleItems() []listItem {
	return []listItem{
		{ProgramSegmentID: 100, Date: 1777521600000, Title: "Scene One", Image: "https://img.example.com/1.jpg", Slug: "scene-one-w100"},
		{ProgramSegmentID: 200, Date: 1777435200000, Title: "Scene Two", Image: "https://img.example.com/2.jpg", Slug: "scene-two-w200"},
	}
}

func sampleDetail() detail {
	return detail{
		Description: "<p>A test description.</p>",
		Segment:     detailSegment{Name: "News off the Top"},
		Clip:        detailClip{ImageURL: "https://img.example.com/frame.jpg"},
		Anchors:     []detailAnchor{{Name: "Jane Doe"}, {Name: "Alice Smith"}},
		LikesCount:  42,
		Tags:        []detailTag{{Name: "politics"}, {Name: "world"}},
	}
}

func testHandler(items []listItem, d detail) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/rest/v1/program/programSegment/") {
			_, _ = fmt.Fprint(w, mustJSON(d))
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/rest/v1/anchor/") && strings.Contains(r.URL.Path, "/segment") {
			_, _ = fmt.Fprint(w, mustJSON(items))
			return
		}
		if r.URL.Path == "/api/rest/v1/audition" {
			_, _ = fmt.Fprint(w, mustJSON(listResponse{Segments: items, Count: len(items)}))
			return
		}
		if r.URL.Path == "/api/rest/v2/featured" {
			_, _ = fmt.Fprint(w, mustJSON(featuredResponse{Content: items, TotalContent: len(items)}))
			return
		}
		_, _ = fmt.Fprint(w, mustJSON(listResponse{Segments: items, Count: len(items)}))
	}
}

// ---- tests ----

func TestMatchesURL(t *testing.T) {
	s := New()
	tests := []struct {
		url  string
		want bool
	}{
		{"https://www.nakednews.com/", true},
		{"https://www.nakednews.com", true},
		{"https://nakednews.com/", true},
		{"https://nakednews.com", true},
		{"https://www.nakednews.com/archives", true},
		{"https://www.nakednews.com/archives?segmentid=217", true},
		{"https://www.nakednews.com/archives?anchorid=64", true},
		{"https://www.nakednews.com/naked-news-anchor-alana-blaire-a104", true},
		{"https://www.nakednews.com/naked-news-anchor-alana-blaire-a104?filter=fanzone", true},
		{"https://www.nakednews.com/naked-news-anchor-yoyo-wu-a47884", true},
		{"https://www.nakednews.com/naked-news-anchor-victoria-sinclair-a44", true},
		{"https://www.nakednews.com/2022/03", true},
		{"https://www.nakednews.com/auditions", true},
		{"https://www.nakednews.com/clip-store", true},
		{"https://example.com/archives", false},
		{"https://www.nakednews.com/some-random-page", false},
		{"https://www.nakednews.com/2022/3", false},
	}
	for _, tt := range tests {
		if got := s.MatchesURL(tt.url); got != tt.want {
			t.Errorf("MatchesURL(%q) = %v, want %v", tt.url, got, tt.want)
		}
	}
}

func TestParseMode(t *testing.T) {
	tests := []struct {
		url       string
		wantMode  fetchMode
		wantParam string
		wantYear  string
		wantMonth string
	}{
		{"https://www.nakednews.com/", modeAll, "", "", ""},
		{"https://www.nakednews.com/archives", modeAll, "", "", ""},
		{"https://www.nakednews.com/archives?segmentid=217", modeSegmentType, "217", "", ""},
		{"https://www.nakednews.com/archives?anchorid=64", modeAnchor, "64", "", ""},
		{"https://www.nakednews.com/naked-news-anchor-alana-blaire-a104", modeAnchor, "104", "", ""},
		{"https://www.nakednews.com/naked-news-anchor-alana-blaire-a104?filter=fanzone", modeAnchor, "104", "", ""},
		{"https://www.nakednews.com/2022/03", modeDate, "", "2022", "3"},
		{"https://www.nakednews.com/auditions", modeAuditions, "", "", ""},
		{"https://www.nakednews.com/clip-store", modeFeatured, "", "", ""},
	}
	for _, tt := range tests {
		cfg, err := parseMode(tt.url)
		if err != nil {
			t.Errorf("parseMode(%q) error: %v", tt.url, err)
			continue
		}
		if cfg.mode != tt.wantMode {
			t.Errorf("parseMode(%q) mode = %d, want %d", tt.url, cfg.mode, tt.wantMode)
		}
		if tt.wantParam != "" && cfg.param != tt.wantParam {
			t.Errorf("parseMode(%q) param = %q, want %q", tt.url, cfg.param, tt.wantParam)
		}
		if tt.wantYear != "" && cfg.year != tt.wantYear {
			t.Errorf("parseMode(%q) year = %q, want %q", tt.url, cfg.year, tt.wantYear)
		}
		if tt.wantMonth != "" && cfg.month != tt.wantMonth {
			t.Errorf("parseMode(%q) month = %q, want %q", tt.url, cfg.month, tt.wantMonth)
		}
	}
}

func TestToScene(t *testing.T) {
	item := listItem{
		ProgramSegmentID: 71344,
		Date:             1777521600000,
		Title:            "Eila Adams In News off the Top",
		Image:            "https://img.example.com/thumb.jpg",
		Slug:             "eila-adams-news-off-the-top-w71344",
	}
	d := &detail{
		Description: "<p>Some description.</p>",
		Segment:     detailSegment{Name: "News off the Top"},
		Clip:        detailClip{ImageURL: "https://img.example.com/frame.jpg"},
		Anchors:     []detailAnchor{{Name: "Eila Adams"}},
		Tags:        []detailTag{{Name: "Iran"}, {Name: "oil prices"}},
		LikesCount:  14,
	}

	scene := toScene("https://www.nakednews.com/archives", item, d)

	if scene.ID != "71344" {
		t.Errorf("ID = %q", scene.ID)
	}
	if scene.SiteID != "nakednews" {
		t.Errorf("SiteID = %q", scene.SiteID)
	}
	if scene.Title != "Eila Adams In News off the Top" {
		t.Errorf("Title = %q", scene.Title)
	}
	if scene.URL != "https://www.nakednews.com/eila-adams-news-off-the-top-w71344" {
		t.Errorf("URL = %q", scene.URL)
	}
	if scene.Thumbnail != "https://img.example.com/frame.jpg" {
		t.Errorf("Thumbnail = %q (should prefer detail clip image)", scene.Thumbnail)
	}
	if scene.Studio != "Naked News" {
		t.Errorf("Studio = %q", scene.Studio)
	}
	if scene.Description != "<p>Some description.</p>" {
		t.Errorf("Description = %q", scene.Description)
	}
	if scene.Likes != 14 {
		t.Errorf("Likes = %d", scene.Likes)
	}
	wantDate := time.UnixMilli(1777521600000).UTC()
	if !scene.Date.Equal(wantDate) {
		t.Errorf("Date = %v, want %v", scene.Date, wantDate)
	}
	if len(scene.Performers) != 1 || scene.Performers[0] != "Eila Adams" {
		t.Errorf("Performers = %v", scene.Performers)
	}
	if len(scene.Tags) != 2 || scene.Tags[0] != "Iran" {
		t.Errorf("Tags = %v", scene.Tags)
	}
	if len(scene.Categories) != 1 || scene.Categories[0] != "News off the Top" {
		t.Errorf("Categories = %v", scene.Categories)
	}
}

func TestToSceneNoDetail(t *testing.T) {
	item := listItem{
		ProgramSegmentID: 555,
		Date:             1777521600000,
		Title:            "Basic Scene",
		Image:            "https://img.example.com/basic.jpg",
		Slug:             "basic-scene-w555",
	}

	scene := toScene("https://www.nakednews.com/archives", item, nil)

	if scene.ID != "555" {
		t.Errorf("ID = %q", scene.ID)
	}
	if scene.Thumbnail != "https://img.example.com/basic.jpg" {
		t.Errorf("Thumbnail = %q (should use listing image when no detail)", scene.Thumbnail)
	}
	if len(scene.Performers) != 0 {
		t.Errorf("Performers = %v", scene.Performers)
	}
}

func TestToSceneNoDate(t *testing.T) {
	item := listItem{ProgramSegmentID: 1, Slug: "s-w1"}
	scene := toScene("https://www.nakednews.com/archives", item, nil)
	if !scene.Date.IsZero() {
		t.Errorf("Date should be zero, got %v", scene.Date)
	}
}

func TestBuildSceneURL(t *testing.T) {
	tests := []struct {
		studioURL string
		slug      string
		want      string
	}{
		{"https://www.nakednews.com/archives", "eila-w71344", "https://www.nakednews.com/eila-w71344"},
		{"http://localhost:8080/archives", "test-w1", "http://localhost:8080/test-w1"},
	}
	for _, tt := range tests {
		if got := buildSceneURL(tt.studioURL, tt.slug); got != tt.want {
			t.Errorf("buildSceneURL(%q, %q) = %q, want %q", tt.studioURL, tt.slug, got, tt.want)
		}
	}
}

func TestApiBase(t *testing.T) {
	tests := []struct {
		studioURL string
		want      string
	}{
		{"https://www.nakednews.com/archives", "https://www.nakednews.com/api/rest"},
		{"http://localhost:8080/archives", "http://localhost:8080/api/rest"},
	}
	for _, tt := range tests {
		if got := apiBase(tt.studioURL); got != tt.want {
			t.Errorf("apiBase(%q) = %q, want %q", tt.studioURL, got, tt.want)
		}
	}
}

func TestListScenes(t *testing.T) {
	ts := httptest.NewServer(testHandler(sampleItems(), sampleDetail()))
	defer ts.Close()

	s := New()
	ch, err := s.ListScenes(context.Background(), ts.URL+"/archives", scraper.ListOpts{Delay: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}

	var count int
	for r := range ch {
		switch r.Kind {
		case scraper.KindScene:
			count++
			if r.Scene.Studio != "Naked News" {
				t.Errorf("Studio = %q", r.Scene.Studio)
			}
			if len(r.Scene.Performers) == 0 {
				t.Error("expected performers from detail fetch")
			}
		case scraper.KindError:
			t.Errorf("unexpected error: %v", r.Err)
		}
	}
	if count != 2 {
		t.Errorf("got %d scenes, want 2", count)
	}
}

func TestListScenesKnownIDs(t *testing.T) {
	ts := httptest.NewServer(testHandler(sampleItems(), sampleDetail()))
	defer ts.Close()

	s := New()
	ch, err := s.ListScenes(context.Background(), ts.URL+"/archives", scraper.ListOpts{
		Delay:    time.Millisecond,
		KnownIDs: map[string]bool{"200": true},
	})
	if err != nil {
		t.Fatal(err)
	}

	var scenes, stopped int
	for r := range ch {
		switch r.Kind {
		case scraper.KindScene:
			scenes++
		case scraper.KindStoppedEarly:
			stopped++
		case scraper.KindError:
			t.Errorf("error: %v", r.Err)
		}
	}
	if scenes != 1 {
		t.Errorf("got %d scenes, want 1", scenes)
	}
	if stopped != 1 {
		t.Errorf("got %d stopped, want 1", stopped)
	}
}

func TestListScenesAnchorMode(t *testing.T) {
	items := []listItem{
		{ProgramSegmentID: 300, Date: 1777521600000, Title: "Anchor Scene", Image: "img.jpg", Slug: "anchor-w300"},
	}
	d := detail{Anchors: []detailAnchor{{Name: "Alana Blaire"}}}
	ts := httptest.NewServer(testHandler(items, d))
	defer ts.Close()

	s := New()
	ch, err := s.ListScenes(context.Background(), ts.URL+"/naked-news-anchor-alana-blaire-a104", scraper.ListOpts{Delay: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}

	var count int
	for r := range ch {
		switch r.Kind {
		case scraper.KindScene:
			count++
			if len(r.Scene.Performers) != 1 || r.Scene.Performers[0] != "Alana Blaire" {
				t.Errorf("Performers = %v", r.Scene.Performers)
			}
		case scraper.KindError:
			t.Errorf("error: %v", r.Err)
		}
	}
	if count != 1 {
		t.Errorf("got %d scenes, want 1", count)
	}
}

func TestListScenesAuditionsMode(t *testing.T) {
	items := []listItem{
		{ProgramSegmentID: 400, Date: 1777521600000, Title: "Audition", Image: "img.jpg", Slug: "audition-w400"},
	}
	ts := httptest.NewServer(testHandler(items, detail{}))
	defer ts.Close()

	s := New()
	ch, err := s.ListScenes(context.Background(), ts.URL+"/auditions", scraper.ListOpts{Delay: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}

	var count int
	for r := range ch {
		switch r.Kind {
		case scraper.KindScene:
			count++
		case scraper.KindError:
			t.Errorf("error: %v", r.Err)
		}
	}
	if count != 1 {
		t.Errorf("got %d scenes, want 1", count)
	}
}

func TestListScenesFeaturedMode(t *testing.T) {
	items := []listItem{
		{ProgramSegmentID: 500, Date: 1777521600000, Title: "Featured", Image: "img.jpg", Slug: "featured-w500"},
	}
	ts := httptest.NewServer(testHandler(items, detail{}))
	defer ts.Close()

	s := New()
	ch, err := s.ListScenes(context.Background(), ts.URL+"/clip-store", scraper.ListOpts{Delay: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}

	var count int
	for r := range ch {
		switch r.Kind {
		case scraper.KindScene:
			count++
		case scraper.KindError:
			t.Errorf("error: %v", r.Err)
		}
	}
	if count != 1 {
		t.Errorf("got %d scenes, want 1", count)
	}
}

func TestListScenesPagination(t *testing.T) {
	page0 := make([]listItem, pageSize)
	for i := range page0 {
		page0[i] = listItem{ProgramSegmentID: i + 1, Title: "Scene", Slug: fmt.Sprintf("s-w%d", i+1)}
	}
	page1 := []listItem{{ProgramSegmentID: pageSize + 1, Title: "Last", Slug: fmt.Sprintf("s-w%d", pageSize+1)}}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/rest/v1/program/programSegment/") {
			_, _ = fmt.Fprint(w, mustJSON(detail{}))
			return
		}
		page := r.URL.Query().Get("page")
		if page == "1" {
			_, _ = fmt.Fprint(w, mustJSON(listResponse{Segments: page1, Count: pageSize + 1}))
		} else {
			_, _ = fmt.Fprint(w, mustJSON(listResponse{Segments: page0, Count: pageSize + 1}))
		}
	}))
	defer ts.Close()

	s := New()
	ch, err := s.ListScenes(context.Background(), ts.URL+"/archives", scraper.ListOpts{Delay: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}

	var count int
	for r := range ch {
		switch r.Kind {
		case scraper.KindScene:
			count++
		case scraper.KindError:
			t.Errorf("error: %v", r.Err)
		}
	}
	if count != pageSize+1 {
		t.Errorf("got %d scenes, want %d", count, pageSize+1)
	}
}

func TestListScenesDedup(t *testing.T) {
	items := []listItem{
		{ProgramSegmentID: 100, Date: 1777521600000, Title: "Scene", Image: "img.jpg", Slug: "scene-w100"},
		{ProgramSegmentID: 100, Date: 1777521600000, Title: "Scene", Image: "img.jpg", Slug: "scene-w100"},
		{ProgramSegmentID: 200, Date: 1777435200000, Title: "Other", Image: "img.jpg", Slug: "other-w200"},
	}
	ts := httptest.NewServer(testHandler(items, detail{}))
	defer ts.Close()

	s := New()
	ch, err := s.ListScenes(context.Background(), ts.URL+"/archives", scraper.ListOpts{Delay: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}

	var count int
	for r := range ch {
		switch r.Kind {
		case scraper.KindScene:
			count++
		case scraper.KindError:
			t.Errorf("error: %v", r.Err)
		}
	}
	if count != 2 {
		t.Errorf("got %d scenes, want 2 (deduped)", count)
	}
}
