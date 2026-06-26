package nvg

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

	"github.com/Wasylq/FSS/scraper"
)

// ---- Gatsby (Net Video Girls) ----

const gatsbyJSON = `{
  "result": {
    "data": {
      "allupdates": {
        "nodes": [
          {
            "short_title": "Amber &amp; Friend",
            "release_date": "2026-06-23T21:27:25.000Z",
            "mysqlId": 4242,
            "tour_stats": [
              {
                "tour_thumb": {
                  "thumb_name": "amber",
                  "localImage": {
                    "childImageSharp": {
                      "gatsbyImageData": {
                        "images": { "fallback": { "src": "/static/amber.jpg" } }
                      }
                    }
                  }
                }
              }
            ]
          },
          {
            "short_title": "No ID skipped",
            "release_date": "2026-01-01T00:00:00.000Z",
            "mysqlId": 0,
            "tour_stats": []
          }
        ]
      }
    }
  }
}`

func gatsbyConfig(base string) siteConfig {
	return siteConfig{
		id:     "netvideogirls",
		studio: "Net Video Girls",
		base:   base,
		front:  frontGatsby,
		match:  regexp.MustCompile(`.*`),
	}
}

func TestFetchGatsby(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/page-data/home/page-data.json":
			_, _ = fmt.Fprint(w, gatsbyJSON)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	s := New(gatsbyConfig(ts.URL))
	now := time.Date(2026, 6, 25, 0, 0, 0, 0, time.UTC)
	scenes, err := s.fetchGatsby(context.Background(), ts.URL, now)
	if err != nil {
		t.Fatalf("fetchGatsby: %v", err)
	}
	if len(scenes) != 1 { // mysqlId 0 skipped
		t.Fatalf("got %d scenes, want 1", len(scenes))
	}
	sc := scenes[0]
	if sc.ID != "4242" {
		t.Errorf("ID = %q", sc.ID)
	}
	if sc.SiteID != "netvideogirls" {
		t.Errorf("SiteID = %q", sc.SiteID)
	}
	if sc.Title != "Amber & Friend" {
		t.Errorf("Title = %q", sc.Title)
	}
	if sc.Studio != "Net Video Girls" {
		t.Errorf("Studio = %q", sc.Studio)
	}
	if sc.Thumbnail != ts.URL+"/static/amber.jpg" {
		t.Errorf("Thumbnail = %q", sc.Thumbnail)
	}
	wantDate := time.Date(2026, 6, 23, 21, 27, 25, 0, time.UTC)
	if !sc.Date.Equal(wantDate) {
		t.Errorf("Date = %v, want %v", sc.Date, wantDate)
	}
}

// ---- Next.js (Casting Couch-HD / Net Girl) ----

const nextHTML = `<html><body>
<script id="__NEXT_DATA__" type="application/json">{
  "props": { "pageProps": { "videos": [
    {
      "id": 99,
      "short_title": "Short One",
      "custom_title": "Custom &amp; Title",
      "release_date": "2026-06-19 14:13:45",
      "video_duration": 1830,
      "allModels": "Ignored",
      "models": [ { "model_name": "Jane Doe" }, { "model_name": "Jane Doe" }, { "model_name": "Mary Sue" } ]
    },
    {
      "id": 100,
      "short_title": "Fallback Short",
      "custom_title": "  ",
      "release_date": "2026-05-01 00:00:00",
      "video_duration": 600,
      "allModels": "AllOne, AllTwo",
      "models": []
    },
    { "id": 0, "short_title": "skip" }
  ] } }
}</script>
</body></html>`

func nextConfig(base string) siteConfig {
	return siteConfig{
		id:        "castingcouchhd",
		studio:    "Casting Couch-HD",
		base:      base,
		thumbBase: "https://cdn.example.com/img/",
		front:     frontNext,
		match:     regexp.MustCompile(`.*`),
	}
}

func TestFetchNext(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			_, _ = fmt.Fprint(w, nextHTML)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	s := New(nextConfig(ts.URL))
	now := time.Date(2026, 6, 25, 0, 0, 0, 0, time.UTC)
	scenes, err := s.fetchNext(context.Background(), ts.URL, now)
	if err != nil {
		t.Fatalf("fetchNext: %v", err)
	}
	if len(scenes) != 2 { // id 0 skipped
		t.Fatalf("got %d scenes, want 2", len(scenes))
	}

	first := scenes[0]
	if first.ID != "99" {
		t.Errorf("ID = %q", first.ID)
	}
	if first.Title != "Custom & Title" {
		t.Errorf("Title = %q (want custom, unescaped)", first.Title)
	}
	if first.Duration != 1830 {
		t.Errorf("Duration = %d", first.Duration)
	}
	wantDate := time.Date(2026, 6, 19, 14, 13, 45, 0, time.UTC)
	if !first.Date.Equal(wantDate) {
		t.Errorf("Date = %v, want %v", first.Date, wantDate)
	}
	// models dedupes Jane Doe; allModels ignored when models present.
	if len(first.Performers) != 2 || first.Performers[0] != "Jane Doe" || first.Performers[1] != "Mary Sue" {
		t.Errorf("Performers = %v", first.Performers)
	}
	if first.Thumbnail != "https://cdn.example.com/img/99-1-med.jpg" {
		t.Errorf("Thumbnail = %q", first.Thumbnail)
	}

	second := scenes[1]
	if second.Title != "Fallback Short" {
		t.Errorf("second Title = %q (want short_title fallback)", second.Title)
	}
	// models empty → allModels split.
	if len(second.Performers) != 2 || second.Performers[0] != "AllOne" || second.Performers[1] != "AllTwo" {
		t.Errorf("second Performers = %v", second.Performers)
	}
}

// ---- date parsing ----

func TestParseDates(t *testing.T) {
	if got := parseISODate("2026-06-23T21:27:25.000Z"); got.IsZero() {
		t.Error("parseISODate returned zero for valid input")
	}
	if got := parseISODate(""); !got.IsZero() {
		t.Errorf("parseISODate(\"\") = %v, want zero", got)
	}
	if got := parseISODate("garbage"); !got.IsZero() {
		t.Errorf("parseISODate(garbage) = %v, want zero", got)
	}
	if got := parseNextDate("2026-06-19 14:13:45"); got.IsZero() {
		t.Error("parseNextDate returned zero for valid input")
	}
	if got := parseNextDate("bad"); !got.IsZero() {
		t.Errorf("parseNextDate(bad) = %v, want zero", got)
	}
}

func TestMatchesURL(t *testing.T) {
	s := New(sites[0]) // netvideogirls
	if !s.MatchesURL("https://netvideogirls.com/") {
		t.Error("expected netvideogirls.com to match")
	}
	if s.MatchesURL("https://castingcouch-hd.com/") {
		t.Error("netvideogirls scraper should not match castingcouch-hd.com")
	}
}

// ---- end-to-end run() via httptest ----

func TestListScenes_gatsbyEndToEnd(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/page-data/home/page-data.json":
			_, _ = fmt.Fprint(w, gatsbyJSON)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	s := New(gatsbyConfig(ts.URL))
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
	if count != 1 {
		t.Errorf("got %d scenes, want 1", count)
	}
}

func TestListScenes_nextEndToEnd(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			_, _ = fmt.Fprint(w, nextHTML)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	s := New(nextConfig(ts.URL))
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
	if count != 2 {
		t.Errorf("got %d scenes, want 2", count)
	}
}
