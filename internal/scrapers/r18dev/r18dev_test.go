package r18dev

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

// ---- fixtures ----

func listJSON(items []listItem, total int) string {
	lr := listResponse{Results: items, TotalResults: total}
	b, _ := json.Marshal(lr)
	return string(b)
}

func detailJSON(dr detailResponse) string {
	b, _ := json.Marshal(dr)
	return string(b)
}

func strPtr(s string) *string { return &s }
func intPtr(i int) *int       { return &i }

func sampleListItems() []listItem {
	return []listItem{
		{ContentID: "abc001", DvdID: strPtr("ABC-001")},
		{ContentID: "def002", DvdID: strPtr("DEF-002")},
	}
}

func sampleDetail(contentID, dvdID string) detailResponse {
	return detailResponse{
		ContentID:      contentID,
		DvdID:          strPtr(dvdID),
		TitleJA:        strPtr("テストタイトル"),
		TitleEN:        strPtr("Test Title"),
		CommentEN:      strPtr("A test description."),
		ReleaseDate:    strPtr("2025-01-15"),
		RuntimeMins:    intPtr(120),
		JacketFullURL:  strPtr("https://pics.example.com/abc001ps.jpg"),
		JacketThumbURL: strPtr("https://pics.example.com/abc001pt.jpg"),
		MakerNameEN:    strPtr("Test Studio"),
		MakerNameJA:    strPtr("テストスタジオ"),
		LabelNameEN:    strPtr("Test Label"),
		Actresses: []person{
			{NameKanji: "田中花子", NameRomaji: "Hanako Tanaka"},
			{NameKanji: "鈴木次郎", NameRomaji: ""},
		},
		Directors: []person{
			{NameKanji: "山田太郎", NameRomaji: "Taro Yamada"},
		},
		Categories: []category{
			{NameEN: "Drama", NameJA: "ドラマ"},
			{NameEN: "", NameJA: "巨乳"},
		},
	}
}

// ---- tests ----

func TestMatchesURL(t *testing.T) {
	s := New()
	tests := []struct {
		url  string
		want bool
	}{
		{"https://r18.dev/videos/vod/movies/list/?id=123&type=actress", true},
		{"https://r18.dev/videos/vod/movies/list/?id=40018&type=studio", true},
		{"https://r18.dev/videos/vod/movies/list/?id=1014&type=category", true},
		{"https://r18.dev/videos/vod/movies/list/?id=107546&type=director", true},
		{"http://r18.dev/videos/vod/movies/list/?id=1&type=actress", true},
		{"https://www.r18.dev/videos/vod/movies/list/?id=1&type=studio", true},
		{"https://r18.dev/videos/vod/movies/list?id=1&type=actress", true},
		{"https://r18.dev/videos/vod/movies/detail/-/id=abc001/", false},
		{"https://example.com/videos/vod/movies/list/?id=1&type=actress", false},
		{"https://r18.dev/", false},
	}
	for _, tt := range tests {
		if got := s.MatchesURL(tt.url); got != tt.want {
			t.Errorf("MatchesURL(%q) = %v, want %v", tt.url, got, tt.want)
		}
	}
}

func TestBuildListURL(t *testing.T) {
	got := buildListURL("https://r18.dev/videos/vod/movies/list/?id=123&type=actress", 1)
	if got != "https://r18.dev/videos/vod/movies/list2/json?id=123&page=1&type=actress" {
		t.Errorf("buildListURL page 1 = %q", got)
	}
	got = buildListURL("https://r18.dev/videos/vod/movies/list/?id=123&type=actress", 3)
	if got != "https://r18.dev/videos/vod/movies/list2/json?id=123&page=3&type=actress" {
		t.Errorf("buildListURL page 3 = %q", got)
	}
}

func TestBuildDetailURL(t *testing.T) {
	got := buildDetailURL("https://r18.dev/videos/vod/movies/list/?id=1&type=studio", "abc001")
	want := "https://r18.dev/videos/vod/movies/detail/-/combined=abc001/json"
	if got != want {
		t.Errorf("buildDetailURL = %q, want %q", got, want)
	}
}

func TestSceneURL(t *testing.T) {
	got := sceneURL("abc001")
	want := "https://r18.dev/videos/vod/movies/detail/-/id=abc001/"
	if got != want {
		t.Errorf("sceneURL = %q, want %q", got, want)
	}
}

func TestItemID(t *testing.T) {
	tests := []struct {
		item listItem
		want string
	}{
		{listItem{ContentID: "abc001", DvdID: strPtr("ABC-001")}, "ABC-001"},
		{listItem{ContentID: "abc00001", DvdID: nil}, "ABC00001"},
		{listItem{ContentID: "xyz123", DvdID: strPtr("")}, "XYZ123"},
	}
	for _, tt := range tests {
		if got := itemID(tt.item); got != tt.want {
			t.Errorf("itemID(%v) = %q, want %q", tt.item.ContentID, got, tt.want)
		}
	}
}

func TestToScene(t *testing.T) {
	dr := sampleDetail("abc001", "ABC-001")
	scene := toScene("https://r18.dev/videos/vod/movies/list/?id=123&type=studio", dr)

	if scene.ID != "ABC-001" {
		t.Errorf("ID = %q, want ABC-001", scene.ID)
	}
	if scene.SiteID != "r18dev" {
		t.Errorf("SiteID = %q", scene.SiteID)
	}
	if scene.Title != "テストタイトル" {
		t.Errorf("Title = %q", scene.Title)
	}
	if scene.Description != "A test description." {
		t.Errorf("Description = %q", scene.Description)
	}
	if scene.Thumbnail != "https://pics.example.com/abc001ps.jpg" {
		t.Errorf("Thumbnail = %q", scene.Thumbnail)
	}
	wantDate := time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)
	if !scene.Date.Equal(wantDate) {
		t.Errorf("Date = %v, want %v", scene.Date, wantDate)
	}
	if scene.Duration != 7200 {
		t.Errorf("Duration = %d, want 7200", scene.Duration)
	}
	if len(scene.Performers) != 2 || scene.Performers[0] != "Hanako Tanaka" || scene.Performers[1] != "鈴木次郎" {
		t.Errorf("Performers = %v", scene.Performers)
	}
	if scene.Director != "Taro Yamada" {
		t.Errorf("Director = %q", scene.Director)
	}
	if scene.Studio != "Test Studio" {
		t.Errorf("Studio = %q", scene.Studio)
	}
	if len(scene.Tags) != 2 || scene.Tags[0] != "Drama" || scene.Tags[1] != "巨乳" {
		t.Errorf("Tags = %v", scene.Tags)
	}
	if scene.URL != "https://r18.dev/videos/vod/movies/detail/-/id=abc001/" {
		t.Errorf("URL = %q", scene.URL)
	}
}

func TestToSceneNoDvdID(t *testing.T) {
	dr := sampleDetail("abc00001", "")
	dr.DvdID = nil
	scene := toScene("https://r18.dev/videos/vod/movies/list/?id=1&type=actress", dr)
	if scene.ID != "ABC00001" {
		t.Errorf("ID = %q, want ABC00001", scene.ID)
	}
}

func TestToSceneTitleFallback(t *testing.T) {
	dr := detailResponse{
		ContentID: "xyz001",
		TitleJA:   nil,
		TitleEN:   strPtr("English Only Title"),
	}
	scene := toScene("https://r18.dev/videos/vod/movies/list/?id=1&type=studio", dr)
	if scene.Title != "English Only Title" {
		t.Errorf("Title = %q, want English Only Title", scene.Title)
	}
}

func TestToSceneNoOptionalFields(t *testing.T) {
	dr := detailResponse{
		ContentID: "bare001",
		TitleJA:   strPtr("タイトル"),
	}
	scene := toScene("https://r18.dev/videos/vod/movies/list/?id=1&type=actress", dr)
	if scene.ID != "BARE001" {
		t.Errorf("ID = %q", scene.ID)
	}
	if scene.Title != "タイトル" {
		t.Errorf("Title = %q", scene.Title)
	}
	if scene.Description != "" {
		t.Errorf("Description should be empty, got %q", scene.Description)
	}
	if scene.Duration != 0 {
		t.Errorf("Duration should be 0, got %d", scene.Duration)
	}
	if len(scene.Performers) != 0 {
		t.Errorf("Performers should be empty, got %v", scene.Performers)
	}
}

func TestListScenes(t *testing.T) {
	items := sampleListItems()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/videos/vod/movies/list2/json":
			_, _ = fmt.Fprint(w, listJSON(items, 2))
		case r.URL.Path == "/videos/vod/movies/detail/-/combined=abc001/json":
			_, _ = fmt.Fprint(w, detailJSON(sampleDetail("abc001", "ABC-001")))
		case r.URL.Path == "/videos/vod/movies/detail/-/combined=def002/json":
			_, _ = fmt.Fprint(w, detailJSON(sampleDetail("def002", "DEF-002")))
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	s := New()
	studioURL := ts.URL + "/videos/vod/movies/list/?id=1&type=studio"
	ch, err := s.ListScenes(context.Background(), studioURL, scraper.ListOpts{Delay: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}

	var scenes int
	for r := range ch {
		switch r.Kind {
		case scraper.KindScene:
			scenes++
		case scraper.KindError:
			t.Errorf("unexpected error: %v", r.Err)
		}
	}
	if scenes != 2 {
		t.Errorf("got %d scenes, want 2", scenes)
	}
}

func TestListScenesKnownIDs(t *testing.T) {
	items := []listItem{
		{ContentID: "new001", DvdID: strPtr("NEW-001")},
		{ContentID: "old001", DvdID: strPtr("OLD-001")},
		{ContentID: "old002", DvdID: strPtr("OLD-002")},
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/videos/vod/movies/list2/json":
			_, _ = fmt.Fprint(w, listJSON(items, 3))
		case r.URL.Path == "/videos/vod/movies/detail/-/combined=new001/json":
			_, _ = fmt.Fprint(w, detailJSON(sampleDetail("new001", "NEW-001")))
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	s := New()
	studioURL := ts.URL + "/videos/vod/movies/list/?id=1&type=actress"
	ch, err := s.ListScenes(context.Background(), studioURL, scraper.ListOpts{
		KnownIDs: map[string]bool{"OLD-001": true},
		Delay:    time.Millisecond,
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
			t.Errorf("unexpected error: %v", r.Err)
		}
	}
	if scenes != 1 {
		t.Errorf("got %d scenes, want 1", scenes)
	}
	if stopped != 1 {
		t.Errorf("got %d stopped, want 1", stopped)
	}
}

func TestListScenesPagination(t *testing.T) {
	page1 := make([]listItem, 100)
	for i := range page1 {
		page1[i] = listItem{ContentID: fmt.Sprintf("p1_%03d", i), DvdID: strPtr(fmt.Sprintf("P1-%03d", i))}
	}
	page2 := []listItem{
		{ContentID: "p2_000", DvdID: strPtr("P2-000")},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/videos/vod/movies/list2/json":
			pg := r.URL.Query().Get("page")
			if pg == "2" {
				_, _ = fmt.Fprint(w, listJSON(page2, 101))
			} else {
				_, _ = fmt.Fprint(w, listJSON(page1, 101))
			}
		case r.URL.Path[:len("/videos/vod/movies/detail/")] == "/videos/vod/movies/detail/":
			_, _ = fmt.Fprint(w, detailJSON(detailResponse{
				ContentID: "x",
				TitleJA:   strPtr("Title"),
			}))
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	s := New()
	studioURL := ts.URL + "/videos/vod/movies/list/?id=1&type=category"
	ch, err := s.ListScenes(context.Background(), studioURL, scraper.ListOpts{Delay: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}

	var scenes int
	for r := range ch {
		switch r.Kind {
		case scraper.KindScene:
			scenes++
		case scraper.KindError:
			t.Errorf("unexpected error: %v", r.Err)
		}
	}
	if scenes != 101 {
		t.Errorf("got %d scenes, want 101", scenes)
	}
}

func TestListScenesDedup(t *testing.T) {
	items := []listItem{
		{ContentID: "abc001", DvdID: strPtr("ABC-001")},
		{ContentID: "abc001", DvdID: strPtr("ABC-001")},
		{ContentID: "def002", DvdID: strPtr("DEF-002")},
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/videos/vod/movies/list2/json":
			_, _ = fmt.Fprint(w, listJSON(items, 3))
		default:
			_, _ = fmt.Fprint(w, detailJSON(detailResponse{
				ContentID: "x",
				DvdID:     strPtr("X-001"),
				TitleJA:   strPtr("Title"),
			}))
		}
	}))
	defer ts.Close()

	s := New()
	studioURL := ts.URL + "/videos/vod/movies/list/?id=1&type=studio"
	ch, err := s.ListScenes(context.Background(), studioURL, scraper.ListOpts{Delay: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}

	var scenes int
	for r := range ch {
		if r.Kind == scraper.KindScene {
			scenes++
		}
	}
	if scenes != 2 {
		t.Errorf("got %d scenes, want 2 (dedup should remove duplicate)", scenes)
	}
}
