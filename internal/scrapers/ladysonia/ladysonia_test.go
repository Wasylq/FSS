package ladysonia

import (
	"context"
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
		{"https://www.lady-sonia.com", true},
		{"https://tour.lady-sonia.com/scenes", true},
		{"https://lady-sonia.com/scenes?page=2", true},
		{"https://www.example.com", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.want {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.want)
		}
	}
}

const nextDataTemplate = `<html><body>
<script id="__NEXT_DATA__" type="application/json">%s</script>
</body></html>`

func makePage(scenes []scene, total, totalPages int) string {
	var items string
	for i, sc := range scenes {
		if i > 0 {
			items += ","
		}
		items += fmt.Sprintf(`{
			"id": %d,
			"title": %q,
			"slug": %q,
			"description": %q,
			"publish_date": %q,
			"seconds_duration": %d,
			"models": [%s],
			"tags": [%s],
			"thumb": %q,
			"trailer_screencap": %q,
			"site": "Lady Sonia",
			"videos": {"orig": {"width": %d, "height": %d}},
			"rating": 0
		}`,
			sc.ID, sc.Title, sc.Slug, sc.Description, sc.PublishDate,
			sc.SecondsDuration, quotedList(sc.Models), quotedList(sc.Tags),
			sc.Thumb, sc.TrailerScreencap,
			1920, 1080,
		)
	}
	payload := fmt.Sprintf(`{"props":{"pageProps":{"contents":{"total":%d,"page":"1","total_pages":%d,"data":[%s]}}}}`,
		total, totalPages, items)
	return fmt.Sprintf(nextDataTemplate, payload)
}

func quotedList(ss []string) string {
	var parts string
	for i, s := range ss {
		if i > 0 {
			parts += ","
		}
		parts += fmt.Sprintf("%q", s)
	}
	return parts
}

func TestParsePage(t *testing.T) {
	html := makePage([]scene{
		{ID: 100, Title: "Test Scene", Slug: "test-scene", PublishDate: "2024/03/15 12:00:00", SecondsDuration: 600, Models: []string{"Lady Sonia"}, Tags: []string{"JOI"}},
	}, 50, 3)

	contents, err := parsePage([]byte(html))
	if err != nil {
		t.Fatal(err)
	}
	if contents.Total != 50 {
		t.Errorf("Total = %d, want 50", contents.Total)
	}
	if contents.TotalPages != 3 {
		t.Errorf("TotalPages = %d, want 3", contents.TotalPages)
	}
	if len(contents.Data) != 1 {
		t.Fatalf("len(Data) = %d, want 1", len(contents.Data))
	}
	if contents.Data[0].Title != "Test Scene" {
		t.Errorf("Title = %q", contents.Data[0].Title)
	}
}

func TestToScene(t *testing.T) {
	sc := scene{
		ID:               200,
		Title:            "A Test",
		Slug:             "a-test",
		Description:      "&amp;nbsp;Description&nbsp;text",
		PublishDate:      "2024/04/05 12:00:00",
		SecondsDuration:  518,
		Models:           []string{"Lady Sonia", "Guest"},
		Tags:             []string{"Big Tits", "JOI"},
		Thumb:            "https://cdn.example.com/thumb.jpg",
		TrailerScreencap: "https://cdn.example.com/screenshot.jpg",
		Site:             "Lady Sonia",
		Videos:           map[string]videoFormat{"orig": {Width: 1920, Height: 1080}},
	}

	result := toScene(defaultSiteBase, sc)

	if result.ID != "200" {
		t.Errorf("ID = %q", result.ID)
	}
	if result.SiteID != "ladysonia" {
		t.Errorf("SiteID = %q", result.SiteID)
	}
	if result.URL != "https://tour.lady-sonia.com/scenes/a-test" {
		t.Errorf("URL = %q", result.URL)
	}
	if result.Studio != "Lady Sonia" {
		t.Errorf("Studio = %q", result.Studio)
	}
	if result.Duration != 518 {
		t.Errorf("Duration = %d", result.Duration)
	}
	if result.Thumbnail != "https://cdn.example.com/screenshot.jpg" {
		t.Errorf("Thumbnail = %q (should prefer trailer_screencap)", result.Thumbnail)
	}
	if result.Width != 1920 || result.Height != 1080 {
		t.Errorf("Width=%d Height=%d", result.Width, result.Height)
	}
	if result.Resolution != "1080p" {
		t.Errorf("Resolution = %q", result.Resolution)
	}
	wantDate := time.Date(2024, 4, 5, 12, 0, 0, 0, time.UTC)
	if !result.Date.Equal(wantDate) {
		t.Errorf("Date = %v, want %v", result.Date, wantDate)
	}
	if len(result.Performers) != 2 || result.Performers[0] != "Lady Sonia" {
		t.Errorf("Performers = %v", result.Performers)
	}
	if len(result.Tags) != 2 || result.Tags[0] != "Big Tits" {
		t.Errorf("Tags = %v", result.Tags)
	}
	if result.Description != "&nbsp;Description text" {
		t.Errorf("Description = %q", result.Description)
	}
}

func TestToSceneThumbFallback(t *testing.T) {
	sc := scene{
		ID:    300,
		Slug:  "fallback",
		Thumb: "https://cdn.example.com/thumb.jpg",
	}
	result := toScene(defaultSiteBase, sc)
	if result.Thumbnail != "https://cdn.example.com/thumb.jpg" {
		t.Errorf("Thumbnail = %q, want thumb fallback", result.Thumbnail)
	}
}

func TestListScenes(t *testing.T) {
	scenes := []scene{
		{ID: 1, Title: "Scene A", Slug: "scene-a", PublishDate: "2024/01/01 12:00:00", Models: []string{"Lady Sonia"}, Tags: []string{"Tag1"}},
		{ID: 2, Title: "Scene B", Slug: "scene-b", PublishDate: "2024/01/02 12:00:00", Models: []string{"Lady Sonia"}, Tags: []string{"Tag2"}},
	}
	page := makePage(scenes, 2, 1)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, page)
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client(), siteBase: ts.URL}

	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}

	var titles []string
	for r := range ch {
		if r.Total > 0 || r.StoppedEarly {
			continue
		}
		if r.Err != nil {
			t.Fatalf("unexpected error: %v", r.Err)
		}
		titles = append(titles, r.Scene.Title)
	}
	if len(titles) != 2 {
		t.Fatalf("got %d scenes, want 2", len(titles))
	}
}

func TestKnownIDsStopsEarly(t *testing.T) {
	scenes := []scene{
		{ID: 10, Title: "New", Slug: "new", PublishDate: "2024/02/01 12:00:00"},
		{ID: 11, Title: "Known", Slug: "known", PublishDate: "2024/01/01 12:00:00"},
	}
	page := makePage(scenes, 2, 1)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, page)
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client(), siteBase: ts.URL}
	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{
		KnownIDs: map[string]bool{"11": true},
	})
	if err != nil {
		t.Fatal(err)
	}

	var ids []string
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
			t.Fatalf("unexpected error: %v", r.Err)
		}
		ids = append(ids, r.Scene.ID)
	}
	if !stoppedEarly {
		t.Error("expected StoppedEarly")
	}
	if len(ids) != 1 || ids[0] != "10" {
		t.Errorf("got IDs %v, want [10]", ids)
	}
}

func TestScraperInterface(t *testing.T) {
	var _ scraper.StudioScraper = New()
}
