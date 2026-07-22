package fetishkitsch

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wasylq/FSS/internal/httpx"
	"github.com/Wasylq/FSS/internal/scrapers/testutil"
	"github.com/Wasylq/FSS/scraper"
)

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("reading fixture %s: %v", name, err)
	}
	return b
}

func TestID(t *testing.T) {
	if got := New().ID(); got != siteID {
		t.Errorf("ID() = %q, want %q", got, siteID)
	}
}

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := map[string]bool{
		"https://fetishkitsch.com":                                   true,
		"https://www.fetishkitsch.com/post/638fbf7638f6ae3df300a850": true,
		"https://fetishkitsch.com.evil.test/":                        false,
		"":                                                           false,
	}
	for u, want := range cases {
		if got := s.MatchesURL(u); got != want {
			t.Errorf("MatchesURL(%q) = %v, want %v", u, got, want)
		}
	}
}

// The catalogue arrives in one ~500 KB response, so the read cap has to clear
// the httpx default with room for growth.
func TestMaxCatalogueBytesExceedsDefault(t *testing.T) {
	if maxCatalogueBytes <= httpx.MaxPageBytes {
		t.Errorf("maxCatalogueBytes = %d, must exceed httpx.MaxPageBytes = %d", maxCatalogueBytes, httpx.MaxPageBytes)
	}
}

// Values arrive underscore-separated throughout the API.
func TestDeslug(t *testing.T) {
	cases := map[string]string{
		"Rubber_Toy_Red_Part_3": "Rubber Toy Red Part 3",
		"Red_August":            "Red August",
		"Strap-On":              "Strap-On",
		"2020":                  "2020",
		"":                      "",
		"__":                    "",
	}
	for in, want := range cases {
		if got := deslug(in); got != want {
			t.Errorf("deslug(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDeslugAllDedupes(t *testing.T) {
	got := deslugAll([]string{"Red_August", "Red August", "", "__", "Samantha_Grace"})
	if !slices.Equal(got, []string{"Red August", "Samantha Grace"}) {
		t.Errorf("deslugAll = %v", got)
	}
}

func TestToScene(t *testing.T) {
	p := apiPost{
		ID:             "638fbf7638f6ae3df300a850",
		Title:          "Rubber_Toy_Red_Part_3",
		People:         []string{"Red_August", "Samantha_Grace"},
		Tags:           []string{"2020", "Bondage_Mitts"},
		PublishDate:    "Mar 09, 2020",
		ShootDate:      "Jan 15, 2020",
		VideoLength:    1891,
		VideoThumbnail: "https://cdn/x.jpg",
		Public:         true,
	}
	sc := toScene("https://fetishkitsch.com", p, time.Now())

	if sc.ID != p.ID || sc.URL != siteBase+"/post/"+p.ID {
		t.Errorf("ID/URL = %q / %q", sc.ID, sc.URL)
	}
	if sc.Title != "Rubber Toy Red Part 3" {
		t.Errorf("Title = %q", sc.Title)
	}
	// videoLength is already whole seconds.
	if sc.Duration != 1891 {
		t.Errorf("Duration = %d", sc.Duration)
	}
	if want := time.Date(2020, time.March, 9, 0, 0, 0, 0, time.UTC); !sc.Date.Equal(want) {
		t.Errorf("Date = %v, want the publish date %v", sc.Date, want)
	}
	if !slices.Equal(sc.Performers, []string{"Red August", "Samantha Grace"}) {
		t.Errorf("Performers = %v", sc.Performers)
	}
	// Shoot years are filed as tags by the site itself and are kept.
	if !slices.Equal(sc.Tags, []string{"2020", "Bondage Mitts"}) {
		t.Errorf("Tags = %v", sc.Tags)
	}
	// The API has no description field anywhere in the catalogue.
	if sc.Description != "" {
		t.Errorf("Description = %q, want empty", sc.Description)
	}
}

// shootDate covers the posts that carry no publishDate.
func TestShootDateIsTheDateFallback(t *testing.T) {
	sc := toScene("x", apiPost{ID: "1", ShootDate: "Jan 15, 2020"}, time.Now())
	if want := time.Date(2020, time.January, 15, 0, 0, 0, 0, time.UTC); !sc.Date.Equal(want) {
		t.Errorf("Date = %v, want %v", sc.Date, want)
	}

	sc = toScene("x", apiPost{ID: "1"}, time.Now())
	if !sc.Date.IsZero() {
		t.Errorf("Date = %v, want zero when neither date is present", sc.Date)
	}
}

func TestImagesAreTheThumbnailFallback(t *testing.T) {
	sc := toScene("x", apiPost{ID: "1", Images: []string{"https://cdn/first.jpg", "https://cdn/second.jpg"}}, time.Now())
	if sc.Thumbnail != "https://cdn/first.jpg" {
		t.Errorf("Thumbnail = %q", sc.Thumbnail)
	}
}

// ---- end-to-end ----

func TestListScenes(t *testing.T) {
	catalogue := readFixture(t, "catalogue.json")
	var requests atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.URL.Path != "/api/post" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(catalogue)
	}))
	defer srv.Close()

	orig := siteBase
	siteBase = srv.URL
	defer func() { siteBase = orig }()

	s := New()
	s.Client = srv.Client()

	ch, err := s.ListScenes(context.Background(), srv.URL, scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}
	scenes := testutil.CollectScenes(t, ch)

	// The fixture holds three posts, one of them not public.
	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2 — non-public posts must be dropped", len(scenes))
	}
	for _, sc := range scenes {
		if sc.SiteID != siteID || sc.Studio != studioName {
			t.Errorf("scene %s: SiteID=%q Studio=%q", sc.ID, sc.SiteID, sc.Studio)
		}
		if sc.Title == "" || strings.Contains(sc.Title, "_") {
			t.Errorf("scene %s title not de-slugged: %q", sc.ID, sc.Title)
		}
		if sc.Date.IsZero() {
			t.Errorf("scene %s has no date", sc.ID)
		}
	}
	// The endpoint has no pagination, so the whole run is one request.
	if got := requests.Load(); got != 1 {
		t.Errorf("made %d requests, want exactly 1 — the catalogue is unpaginated", got)
	}
}

func TestCatalogueErrorIsReported(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	orig := siteBase
	siteBase = srv.URL
	defer func() { siteBase = orig }()

	s := New()
	s.Client = srv.Client()

	ch, err := s.ListScenes(context.Background(), srv.URL, scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}
	sawErr := false
	for res := range ch {
		if res.Kind == scraper.KindError {
			sawErr = true
		}
	}
	if !sawErr {
		t.Error("a catalogue failure produced no error result")
	}
}

func TestContextCancellation(t *testing.T) {
	catalogue := readFixture(t, "catalogue.json")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(catalogue)
	}))
	defer srv.Close()

	orig := siteBase
	siteBase = srv.URL
	defer func() { siteBase = orig }()

	s := New()
	s.Client = srv.Client()

	ctx, cancel := context.WithCancel(context.Background())
	ch, err := s.ListScenes(ctx, srv.URL, scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}
	cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for range ch {
		}
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("channel did not close after context cancellation")
	}
}
