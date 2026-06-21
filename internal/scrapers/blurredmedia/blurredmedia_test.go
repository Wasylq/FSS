package blurredmedia

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/scraper"
)

// newTestServer serves the trimmed API fixtures: one listing page and two
// detail pages keyed by slug.
func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	read := func(name string) []byte {
		b, err := os.ReadFile(filepath.Join("testdata", name))
		if err != nil {
			t.Fatalf("read fixture %s: %v", name, err)
		}
		return b
	}
	listing := read("videos_page1.json")
	details := map[string][]byte{
		"bryce-beckett-has-after-hours-3some-with-indicaflower-and-bff-riley-holt":                        read("detail_698.json"),
		"a-cuckn-love-triangle-atlas-eros-and-tate-harper-take-trev-anthony-and-hayden-mallers-to-school": read("detail_4616.json"),
	}

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("SITE") == "" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"no site"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/videos":
			// Single page of fixtures; serve only on page 1 so the loop stops.
			if r.URL.Query().Get("page") != "1" && r.URL.Query().Get("page") != "" {
				_, _ = w.Write([]byte(`{"videos":{"data":[],"current_page":2,"last_page":1,"per_page":24,"total":2}}`))
				return
			}
			_, _ = w.Write(listing)
		case "/api/video":
			slug := r.URL.Query().Get("slug")
			if body, ok := details[slug]; ok {
				_, _ = w.Write(body)
				return
			}
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"not found"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func collect(t *testing.T, s *Scraper, opts scraper.ListOpts) ([]models.Scene, int, bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ch, err := s.ListScenes(ctx, "https://www."+s.cfg.Domain+"/", opts)
	if err != nil {
		t.Fatalf("ListScenes: %v", err)
	}
	var scenes []models.Scene
	total := 0
	stoppedEarly := false
	for res := range ch {
		switch res.Kind {
		case scraper.KindScene:
			scenes = append(scenes, res.Scene)
		case scraper.KindTotal:
			total = res.Total
		case scraper.KindStoppedEarly:
			stoppedEarly = true
		case scraper.KindError:
			t.Fatalf("scrape error: %v", res.Err)
		}
	}
	return scenes, total, stoppedEarly
}

func TestScrapeListing(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	s := newFor("hotguysfuck")
	s.Client = ts.Client()
	s.apiBase = ts.URL

	scenes, total, _ := collect(t, s, scraper.ListOpts{})
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2", len(scenes))
	}

	sc := scenes[0]
	if sc.ID != "698" {
		t.Errorf("ID = %q, want 698", sc.ID)
	}
	if sc.SiteID != "hotguysfuck" {
		t.Errorf("SiteID = %q, want hotguysfuck", sc.SiteID)
	}
	if sc.Studio != "HotGuysFuck" {
		t.Errorf("Studio = %q, want HotGuysFuck", sc.Studio)
	}
	if sc.Title == "" {
		t.Error("Title empty")
	}
	wantURL := "https://www.hotguysfuck.com/video/bryce-beckett-has-after-hours-3some-with-indicaflower-and-bff-riley-holt"
	if sc.URL != wantURL {
		t.Errorf("URL = %q, want %q", sc.URL, wantURL)
	}
	if sc.StudioURL != "https://www.hotguysfuck.com/" {
		t.Errorf("StudioURL = %q", sc.StudioURL)
	}
	if sc.Description == "" {
		t.Error("Description empty (detail not merged)")
	}
	if sc.Duration != 19*60+11 {
		t.Errorf("Duration = %d, want %d", sc.Duration, 19*60+11)
	}
	wantDate := time.Date(2026, 6, 19, 0, 0, 0, 0, time.UTC)
	if !sc.Date.Equal(wantDate) {
		t.Errorf("Date = %v, want %v", sc.Date, wantDate)
	}
	if len(sc.Performers) != 3 {
		t.Errorf("Performers = %v, want 3", sc.Performers)
	}
	if len(sc.Tags) != 2 {
		t.Errorf("Tags = %v, want 2", sc.Tags)
	}
	if sc.ScrapedAt.IsZero() {
		t.Error("ScrapedAt zero")
	}
	if sc.Thumbnail == "" {
		t.Error("Thumbnail empty")
	}

	// Second scene has a longer tag list — confirm detail merge per-item.
	if len(scenes[1].Tags) < 5 {
		t.Errorf("second scene tags = %d, want >=5", len(scenes[1].Tags))
	}
}

func TestKnownIDsEarlyStop(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	s := newFor("hotguysfuck")
	s.Client = ts.Client()
	s.apiBase = ts.URL

	_, _, stopped := collect(t, s, scraper.ListOpts{KnownIDs: map[string]bool{"698": true}})
	if !stopped {
		t.Error("expected StoppedEarly when first ID is known")
	}
}

func TestPerSiteConfig(t *testing.T) {
	for _, want := range []struct {
		id, studio, header, domain string
	}{
		{"hotguysfuck", "HotGuysFuck", "2", "hotguysfuck.com"},
		{"biguysfuck", "BiGuysFuck", "5", "biguysfuck.com"},
		{"gayhoopla", "GayHoopla", "1", "gayhoopla.com"},
		{"sugardaddyporn", "SugarDaddyPorn", "4", "sugardaddyporn.com"},
	} {
		s := newFor(want.id)
		if s == nil {
			t.Fatalf("newFor(%q) returned nil", want.id)
		}
		if s.cfg.StudioName != want.studio || s.cfg.SiteHeader != want.header || s.cfg.Domain != want.domain {
			t.Errorf("%s config = %+v", want.id, s.cfg)
		}
	}
}

func TestMatchesURL(t *testing.T) {
	s := newFor("hotguysfuck")
	cases := map[string]bool{
		"https://hotguysfuck.com/":           true,
		"https://www.hotguysfuck.com/videos": true,
		"http://hotguysfuck.com/video/foo":   true,
		"https://biguysfuck.com/":            false,
		"https://nothotguysfuck.com/":        false,
		"https://hotguysfuck.com.evil.com/":  false,
	}
	for u, want := range cases {
		if got := s.MatchesURL(u); got != want {
			t.Errorf("MatchesURL(%q) = %v, want %v", u, got, want)
		}
	}
}
