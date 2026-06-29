package vrhush

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wasylq/FSS/scraper"
)

const homeHTML = `<html><body>
<script id="__NEXT_DATA__" type="application/json">{"props":{},"buildId":"TESTBUILD123"}</script>
</body></html>`

const scenesPage1 = `{
  "pageProps": {
    "contents": {
      "total": 3,
      "total_pages": 2,
      "data": [
        {
          "id": 70422,
          "title": "Wait It Out &amp; More",
          "slug": "wait-it-out",
          "publish_date": "2026/06/23 00:00:00",
          "videos_duration": "2527.12",
          "tags": ["Hardcore", "Big Tits"],
          "models": ["Aleksa Mink", " "],
          "description": "Aleksa stands in your entryway.",
          "thumbnail": "//cloud.example.com/vrh/thumb.jpg",
          "views": 19
        },
        {
          "id": 70421,
          "title": "Second Scene",
          "slug": "second-scene",
          "publish_date": "2026/06/16 00:00:00",
          "videos_duration": "1800",
          "tags": [],
          "models": ["Jane Doe"],
          "description": "",
          "thumbnail": "https://abs.example.com/x.jpg",
          "views": 5
        }
      ]
    }
  }
}`

const scenesPage2 = `{
  "pageProps": {
    "contents": {
      "total": 3,
      "total_pages": 2,
      "data": [
        {
          "id": 70400,
          "title": "Old Scene",
          "slug": "old-scene",
          "publish_date": "2026/01/01 00:00:00",
          "videos_duration": "900",
          "tags": ["Solo"],
          "models": ["Mary Sue"],
          "description": "desc",
          "thumbnail": "",
          "views": 1
        }
      ]
    }
  }
}`

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/":
			_, _ = fmt.Fprint(w, homeHTML)
		case strings.Contains(r.URL.Path, "/scenes.json"):
			if r.URL.Query().Get("page") == "2" {
				_, _ = fmt.Fprint(w, scenesPage2)
			} else {
				_, _ = fmt.Fprint(w, scenesPage1)
			}
		default:
			http.NotFound(w, r)
		}
	}))
}

func TestFetchBuildID(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()
	old := siteBase
	siteBase = ts.URL
	defer func() { siteBase = old }()

	s := New()
	s.Client = ts.Client()
	id, err := s.fetchBuildID(context.Background())
	if err != nil {
		t.Fatalf("fetchBuildID: %v", err)
	}
	if id != "TESTBUILD123" {
		t.Errorf("buildId = %q", id)
	}
}

func TestFetchPage(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()
	old := siteBase
	siteBase = ts.URL
	defer func() { siteBase = old }()

	s := New()
	s.Client = ts.Client()
	vids, total, totalPages, err := s.fetchPage(context.Background(), "TESTBUILD123", 1)
	if err != nil {
		t.Fatalf("fetchPage: %v", err)
	}
	if total != 3 || totalPages != 2 {
		t.Errorf("total=%d totalPages=%d", total, totalPages)
	}
	if len(vids) != 2 {
		t.Fatalf("got %d videos, want 2", len(vids))
	}

	now := time.Date(2026, 6, 25, 0, 0, 0, 0, time.UTC)
	sc := toScene(ts.URL, vids[0], now)
	if sc.ID != "70422" {
		t.Errorf("ID = %q", sc.ID)
	}
	if sc.Title != "Wait It Out & More" {
		t.Errorf("Title = %q (want unescaped)", sc.Title)
	}
	if sc.URL != siteBase+"/scenes/wait-it-out" {
		t.Errorf("URL = %q", sc.URL)
	}
	if sc.Duration != 2527 {
		t.Errorf("Duration = %d, want 2527", sc.Duration)
	}
	if sc.Views != 19 {
		t.Errorf("Views = %d", sc.Views)
	}
	// blank model entry dropped.
	if len(sc.Performers) != 1 || sc.Performers[0] != "Aleksa Mink" {
		t.Errorf("Performers = %v", sc.Performers)
	}
	if len(sc.Tags) != 2 {
		t.Errorf("Tags = %v", sc.Tags)
	}
	if sc.Thumbnail != "https://cloud.example.com/vrh/thumb.jpg" {
		t.Errorf("Thumbnail = %q (want //-prefixed scheme added)", sc.Thumbnail)
	}
	wantDate := time.Date(2026, 6, 23, 0, 0, 0, 0, time.UTC)
	if !sc.Date.Equal(wantDate) {
		t.Errorf("Date = %v, want %v", sc.Date, wantDate)
	}

	// Absolute thumbnail left as-is.
	sc2 := toScene(ts.URL, vids[1], now)
	if sc2.Thumbnail != "https://abs.example.com/x.jpg" {
		t.Errorf("sc2 Thumbnail = %q", sc2.Thumbnail)
	}
}

func TestListScenesEndToEnd(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()
	old := siteBase
	siteBase = ts.URL
	defer func() { siteBase = old }()

	s := New()
	s.Client = ts.Client()
	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{})
	if err != nil {
		t.Fatalf("ListScenes: %v", err)
	}
	var count int
	for r := range ch {
		switch r.Kind {
		case scraper.KindScene:
			count++
		case scraper.KindError:
			t.Errorf("unexpected error: %v", r.Err)
		}
	}
	// page 1 has 2, page 2 has 1, then Done.
	if count != 3 {
		t.Errorf("got %d scenes, want 3", count)
	}
}

func TestMatchesURL(t *testing.T) {
	s := New()
	if !s.MatchesURL("https://vrhush.com/") {
		t.Error("expected vrhush.com to match")
	}
	if s.MatchesURL("https://dezyred.com/") {
		t.Error("should not match dezyred.com")
	}
}

func TestParseDuration(t *testing.T) {
	if got := parseDuration("2527.12"); got != 2527 {
		t.Errorf("parseDuration(2527.12) = %d", got)
	}
	if got := parseDuration(""); got != 0 {
		t.Errorf("parseDuration(\"\") = %d", got)
	}
	if got := parseDuration("bad"); got != 0 {
		t.Errorf("parseDuration(bad) = %d", got)
	}
}
