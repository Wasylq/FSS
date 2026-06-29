package dezyred

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wasylq/FSS/scraper"
)

const modelsJSON = `[
  {"id":"567","firstName":"Liz","lastName":"Jordan","pageUrl":"/pornstars/liz-jordan"},
  {"id":"181","firstName":"Xxlayna","lastName":"Marie","pageUrl":"/pornstars/xxlayna-marie"},
  {"id":"129","firstName":"Madison","lastName":"Wilde","pageUrl":"/pornstars/madison-wilde"}
]`

const gamesJSON = `[
  {
    "id":"471",
    "pageUrl":"/games/sorority-hookup-greek-week",
    "title":"Sorority Hookup: Greek Week &amp; Friends",
    "annotation":"Sneak into a sorority house.",
    "description":"<div>Dive deeper into the chaos.</div><div><br></div><div>How far will you go?</div>",
    "rating":4,
    "createdAt":"2026-06-19T21:23:50.000000Z",
    "models":[567,181,129,9999],
    "categories":[{"id":"2","title":"8K"},{"id":"4","title":"Teen"}],
    "posters":{"item":"https://dezyred.com/media/item.jpeg","listItem":"https://dezyred.com/media/list.jpeg"}
  },
  {
    "id":"50",
    "pageUrl":"/games/no-models-game",
    "title":"No Models",
    "annotation":"Annotation only.",
    "description":"",
    "createdAt":"2025-01-02T00:00:00.000000Z",
    "models":[],
    "categories":[],
    "posters":{"item":"","listItem":"https://dezyred.com/media/fallback.jpeg"}
  }
]`

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/models":
			_, _ = fmt.Fprint(w, modelsJSON)
		case "/api/games":
			_, _ = fmt.Fprint(w, gamesJSON)
		default:
			http.NotFound(w, r)
		}
	}))
}

func TestFetchGamesAndModels(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	old := siteBase
	siteBase = ts.URL
	defer func() { siteBase = old }()

	s := New()
	s.Client = ts.Client()
	ctx := context.Background()

	modelNames, err := s.fetchModels(ctx)
	if err != nil {
		t.Fatalf("fetchModels: %v", err)
	}
	if modelNames[567] != "Liz Jordan" {
		t.Errorf("model 567 = %q, want Liz Jordan", modelNames[567])
	}

	games, err := s.fetchGames(ctx)
	if err != nil {
		t.Fatalf("fetchGames: %v", err)
	}
	if len(games) != 2 {
		t.Fatalf("got %d games, want 2", len(games))
	}

	now := time.Date(2026, 6, 25, 0, 0, 0, 0, time.UTC)
	sc := toScene(ts.URL, games[0], modelNames, now)

	if sc.ID != "471" {
		t.Errorf("ID = %q", sc.ID)
	}
	if sc.SiteID != siteID {
		t.Errorf("SiteID = %q", sc.SiteID)
	}
	if sc.Title != "Sorority Hookup: Greek Week & Friends" {
		t.Errorf("Title = %q (want unescaped)", sc.Title)
	}
	if sc.URL != ts.URL+"/games/sorority-hookup-greek-week" {
		t.Errorf("URL = %q", sc.URL)
	}
	if sc.Studio != studioName {
		t.Errorf("Studio = %q", sc.Studio)
	}
	if sc.Description != "Dive deeper into the chaos. How far will you go?" {
		t.Errorf("Description = %q (want tags stripped)", sc.Description)
	}
	// 9999 has no model entry → dropped; the three known IDs resolve.
	if len(sc.Performers) != 3 || sc.Performers[0] != "Liz Jordan" {
		t.Errorf("Performers = %v", sc.Performers)
	}
	if len(sc.Categories) != 2 || sc.Categories[0] != "8K" {
		t.Errorf("Categories = %v", sc.Categories)
	}
	if sc.Thumbnail != "https://dezyred.com/media/item.jpeg" {
		t.Errorf("Thumbnail = %q", sc.Thumbnail)
	}
	wantDate := time.Date(2026, 6, 19, 21, 23, 50, 0, time.UTC)
	if !sc.Date.Equal(wantDate) {
		t.Errorf("Date = %v, want %v", sc.Date, wantDate)
	}

	// Second game: annotation fallback, listItem poster fallback, no performers.
	sc2 := toScene(ts.URL, games[1], modelNames, now)
	if sc2.Description != "Annotation only." {
		t.Errorf("sc2 Description = %q (want annotation fallback)", sc2.Description)
	}
	if sc2.Thumbnail != "https://dezyred.com/media/fallback.jpeg" {
		t.Errorf("sc2 Thumbnail = %q (want listItem fallback)", sc2.Thumbnail)
	}
	if len(sc2.Performers) != 0 {
		t.Errorf("sc2 Performers = %v, want none", sc2.Performers)
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
	if count != 2 {
		t.Errorf("got %d scenes, want 2", count)
	}
}

func TestMatchesURL(t *testing.T) {
	s := New()
	if !s.MatchesURL("https://dezyred.com/") {
		t.Error("expected dezyred.com to match")
	}
	if !s.MatchesURL("https://www.dezyred.com/games/foo") {
		t.Error("expected www.dezyred.com to match")
	}
	if s.MatchesURL("https://vrhush.com/") {
		t.Error("should not match vrhush.com")
	}
}

func TestParseDate(t *testing.T) {
	if got := parseDate("2026-06-19T21:23:50.000000Z"); got.IsZero() {
		t.Error("parseDate returned zero for valid input")
	}
	if got := parseDate(""); !got.IsZero() {
		t.Errorf("parseDate(\"\") = %v, want zero", got)
	}
	if got := parseDate("garbage"); !got.IsZero() {
		t.Errorf("parseDate(garbage) = %v, want zero", got)
	}
}
