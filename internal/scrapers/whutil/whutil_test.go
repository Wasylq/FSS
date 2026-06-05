package whutil

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
	"github.com/Wasylq/FSS/scraper"
)

func TestParseCount(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"4.418", 4418},
		{"9.590", 9590},
		{"93", 93},
		{"1.444", 1444},
		{"12.345.678", 12345678},
		{"", 0},
	}
	for _, c := range cases {
		if got := ParseCount(c.in); got != c.want {
			t.Errorf("ParseCount(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestParseDate(t *testing.T) {
	cases := []struct {
		in   string
		want time.Time
		ok   bool
	}{
		{"29/05/2026", time.Date(2026, 5, 29, 0, 0, 0, 0, time.UTC), true},
		{"01/12/2020", time.Date(2020, 12, 1, 0, 0, 0, 0, time.UTC), true},
		{"", time.Time{}, false},
		{"invalid", time.Time{}, false},
	}
	for _, c := range cases {
		got, ok := ParseDate(c.in)
		if ok != c.ok || got != c.want {
			t.Errorf("ParseDate(%q) = (%v, %v), want (%v, %v)", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestFlattenItems(t *testing.T) {
	nested := [][]listItem{
		{{SetID: "a"}, {SetID: "b"}},
		{{SetID: "c"}},
	}
	got := flattenItems(nested)
	if len(got) != 3 {
		t.Fatalf("got %d items, want 3", len(got))
	}
	if got[0].SetID != "a" || got[1].SetID != "b" || got[2].SetID != "c" {
		t.Errorf("got %v", got)
	}
}

func TestFlattenItemsEmpty(t *testing.T) {
	got := flattenItems(nil)
	if len(got) != 0 {
		t.Errorf("got %d items, want 0", len(got))
	}
}

const listingJSON = `{
  "latest": [[
    {"id":1,"setid":"26060951","title":"Test Scene One","category":"Hot Ass","date":"29/05/2026","image":"/thumbs/26/26060951/thumbnails/thumb.jpg","cs_ribbon":0},
    {"id":2,"setid":"26052701","title":"Test Scene Two","category":"Casting","date":"27/05/2026","image":"/thumbs/26/26052701/thumbnails/thumb.jpg","cs_ribbon":0}
  ]],
  "popular":[],
  "count":"93",
  "pages":1,
  "page":1
}`

const detailJSON = `{
  "name":"Test Scene One",
  "public_date":"29/05/2026",
  "videoduration":"20:11",
  "videoduration_min":20,
  "videoduration_sec":11,
  "category_name":"Hot Ass",
  "info":"A test description.",
  "main_image":"https://cdn.example.com/main.jpg"
}`

func TestRun(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/set/list":
			_, _ = fmt.Fprint(w, listingJSON)
		case "/api/v1/set/data":
			_, _ = fmt.Fprint(w, detailJSON)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	s := New(SiteConfig{
		SiteID:     "testsite",
		Domain:     "example.com",
		StudioName: "Test Studio",
		APIBase:    ts.URL + "/api/v1/",
		DetailPath: "/set/detail/",
	})
	s.Client = ts.Client()

	out := make(chan scraper.SceneResult)
	go s.Run(context.Background(), "https://www.example.com", scraper.ListOpts{Workers: 2}, out)

	scenes := testutil.CollectScenes(t, out)

	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2", len(scenes))
	}

	sc := scenes[0]
	if sc.ID != "26060951" && sc.ID != "26052701" {
		t.Errorf("unexpected ID %q", sc.ID)
	}
}

func TestRunWithDetail(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/set/list":
			_, _ = fmt.Fprint(w, `{"latest":[[{"id":1,"setid":"26060951","title":"Scene","category":"Cat","date":"29/05/2026","image":"/thumb.jpg","cs_ribbon":0}]],"count":"1","pages":1}`)
		case "/api/v1/set/data":
			_, _ = fmt.Fprint(w, detailJSON)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	s := New(SiteConfig{
		SiteID:     "testsite",
		Domain:     "example.com",
		StudioName: "Test Studio",
		APIBase:    ts.URL + "/api/v1/",
		DetailPath: "/set/detail/",
	})
	s.Client = ts.Client()

	out := make(chan scraper.SceneResult)
	go s.Run(context.Background(), "https://www.example.com", scraper.ListOpts{Workers: 1}, out)

	scenes := testutil.CollectScenes(t, out)

	if len(scenes) != 1 {
		t.Fatalf("got %d scenes, want 1", len(scenes))
	}

	sc := scenes[0]
	if sc.Duration != 20*60+11 {
		t.Errorf("Duration = %d, want %d", sc.Duration, 20*60+11)
	}
	if sc.Description != "A test description." {
		t.Errorf("Description = %q", sc.Description)
	}
	if sc.Thumbnail != "https://cdn.example.com/main.jpg" {
		t.Errorf("Thumbnail = %q", sc.Thumbnail)
	}
	if sc.SiteID != "testsite" {
		t.Errorf("SiteID = %q", sc.SiteID)
	}
	if sc.URL != "https://www.example.com/set/detail/26060951" {
		t.Errorf("URL = %q", sc.URL)
	}
}

func TestRunKnownIDs(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/set/list":
			_, _ = fmt.Fprint(w, listingJSON)
		case "/api/v1/set/data":
			_, _ = fmt.Fprint(w, detailJSON)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	s := New(SiteConfig{
		SiteID:     "testsite",
		Domain:     "example.com",
		StudioName: "Test Studio",
		APIBase:    ts.URL + "/api/v1/",
		DetailPath: "/set/detail/",
	})
	s.Client = ts.Client()

	out := make(chan scraper.SceneResult)
	go s.Run(context.Background(), "https://www.example.com", scraper.ListOpts{
		Workers:  1,
		KnownIDs: map[string]bool{"26052701": true},
	}, out)

	scenes, stoppedEarly := testutil.CollectScenesWithStop(t, out)

	if !stoppedEarly {
		t.Error("expected StoppedEarly signal")
	}
	if len(scenes) != 1 || scenes[0].ID != "26060951" {
		t.Errorf("got %d scenes, want 1 with ID 26060951", len(scenes))
	}
}

func TestRunSkipsComingSoon(t *testing.T) {
	listing := `{"latest":[[
		{"id":1,"setid":"111","title":"Available","category":"","date":"01/01/2026","image":"/a.jpg","cs_ribbon":0},
		{"id":2,"setid":"222","title":"Coming Soon","category":"","date":"01/01/2026","image":"/b.jpg","cs_ribbon":1}
	]],"count":"2","pages":1}`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/set/list":
			_, _ = fmt.Fprint(w, listing)
		case "/api/v1/set/data":
			_, _ = fmt.Fprint(w, `{}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	s := New(SiteConfig{
		SiteID:     "testsite",
		Domain:     "example.com",
		StudioName: "Test",
		APIBase:    ts.URL + "/api/v1/",
		DetailPath: "/set/detail/",
	})
	s.Client = ts.Client()

	out := make(chan scraper.SceneResult)
	go s.Run(context.Background(), "https://www.example.com", scraper.ListOpts{Workers: 1}, out)

	scenes := testutil.CollectScenes(t, out)
	if len(scenes) != 1 {
		t.Fatalf("got %d scenes, want 1 (cs_ribbon=1 should be skipped)", len(scenes))
	}
	if scenes[0].ID != "111" {
		t.Errorf("ID = %q, want 111", scenes[0].ID)
	}
}

func TestAbsURL(t *testing.T) {
	s := &Scraper{cfg: SiteConfig{Domain: "str8hell.com"}}
	cases := []struct {
		in   string
		want string
	}{
		{"/thumbs/26/thumb.jpg", "https://www.str8hell.com/thumbs/26/thumb.jpg"},
		{"thumbs/26/thumb.jpg", "https://www.str8hell.com/thumbs/26/thumb.jpg"},
		{"https://cdn.example.com/img.jpg", "https://cdn.example.com/img.jpg"},
	}
	for _, c := range cases {
		if got := s.absURL(c.in); got != c.want {
			t.Errorf("absURL(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
