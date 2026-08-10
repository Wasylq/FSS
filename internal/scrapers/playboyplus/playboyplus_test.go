package playboyplus

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
	"github.com/Anastylosis/FSS/scraper"
)

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url  string
		want bool
	}{
		{"https://www.playboyplus.com/", true},
		{"https://playboyplus.com/", true},
		{"https://www.playboyplus.com/en/updates", true},
		{"https://www.playboyplus.com/en/model/view/Alana-Rey/123147", true},
		{"https://www.playboyplus.com/en/update/Some-Title/150693", true},
		{"https://www.example.com/", false},
		{"https://www.playboytv.com/", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.want {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.want)
		}
	}
}

func TestModelURLParsing(t *testing.T) {
	cases := []struct {
		url  string
		want string
	}{
		{"https://www.playboyplus.com/en/model/view/Alana-Rey/123147", "123147"},
		{"https://www.playboyplus.com/en/model/view/Wendy-Hamilton/118867", "118867"},
		{"https://www.playboyplus.com/en/updates", ""},
		{"https://www.playboyplus.com/", ""},
	}
	for _, c := range cases {
		m := modelRe.FindStringSubmatch(c.url)
		got := ""
		if m != nil {
			got = m[1]
		}
		if got != c.want {
			t.Errorf("modelRe(%q) = %q, want %q", c.url, got, c.want)
		}
	}
}

func TestFetchAPIKey(t *testing.T) {
	t.Run("extracts key from page source", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/en/updates" {
				http.NotFound(w, r)
				return
			}
			_, _ = w.Write([]byte(`<html><script>window.env={"api":{"algolia":{"applicationID":"X","apiKey":"abc123def456"}}};</script></html>`))
		}))
		defer ts.Close()

		s := &Scraper{Client: ts.Client(), algoliaHost: ts.URL, siteBaseURL: ts.URL}
		key, err := s.fetchAPIKey(context.Background())
		if err != nil {
			t.Fatalf("fetchAPIKey error: %v", err)
		}
		if key != "abc123def456" {
			t.Errorf("key = %q", key)
		}
	})

	t.Run("returns error when key not in page", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`<html>no algolia config here</html>`))
		}))
		defer ts.Close()

		s := &Scraper{Client: ts.Client(), algoliaHost: ts.URL, siteBaseURL: ts.URL}
		if _, err := s.fetchAPIKey(context.Background()); err == nil {
			t.Error("expected error when API key missing")
		}
	})
}

func TestFetchPage(t *testing.T) {
	hits := []photosetHit{
		{SetID: 100, Title: "Set One", DateOnline: "2026-01-15"},
		{SetID: 200, Title: "Set Two", DateOnline: "2026-01-14"},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(algoliaResponse{Hits: hits, NbHits: 2})
	}))
	defer ts.Close()

	s := &Scraper{Client: ts.Client(), algoliaHost: ts.URL, siteBaseURL: ts.URL}
	got, total, err := s.fetchPage(context.Background(), "test-key", 0, "")
	if err != nil {
		t.Fatalf("fetchPage error: %v", err)
	}
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	if len(got) != 2 || got[0].SetID != 100 || got[1].SetID != 200 {
		t.Errorf("hits = %+v", got)
	}
}

func TestFetchPageModelFilter(t *testing.T) {
	var capturedQuery algoliaQuery
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&capturedQuery)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(algoliaResponse{
			Hits:   []photosetHit{{SetID: 42, Title: "Filtered Set"}},
			NbHits: 1,
		})
	}))
	defer ts.Close()

	s := &Scraper{Client: ts.Client(), algoliaHost: ts.URL, siteBaseURL: ts.URL}
	_, _, err := s.fetchPage(context.Background(), "test-key", 0, "actors.name:Alana Rey")
	if err != nil {
		t.Fatalf("fetchPage error: %v", err)
	}
	if capturedQuery.Filters != "upcoming:0" {
		t.Errorf("filters = %q, want %q", capturedQuery.Filters, "upcoming:0")
	}
	if len(capturedQuery.FacetFilters) != 1 || len(capturedQuery.FacetFilters[0]) != 1 || capturedQuery.FacetFilters[0][0] != "actors.name:Alana Rey" {
		t.Errorf("facetFilters = %v, want [[actors.name:Alana Rey]]", capturedQuery.FacetFilters)
	}
}

func TestResolveActorName(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"hits":[{"name":"Alana Rey","actor_id":123147}],"nbHits":1}`))
	}))
	defer ts.Close()

	s := &Scraper{Client: ts.Client(), algoliaHost: ts.URL, siteBaseURL: ts.URL}
	name, err := s.resolveActorName(context.Background(), "test-key", "123147")
	if err != nil {
		t.Fatalf("resolveActorName error: %v", err)
	}
	if name != "Alana Rey" {
		t.Errorf("name = %q, want %q", name, "Alana Rey")
	}
}

func TestToScene(t *testing.T) {
	hit := photosetHit{
		SetID:       150693,
		Title:       "Alana Rey in Retro Radiance",
		Description: "First line.<br>Second line.&amp; more",
		DateOnline:  "2026-06-08",
		URLTitle:    "Alana-Rey-in-Retro-Radiance",
		SerieName:   "Playboy Plus",
		Actors: []actor{
			{ActorID: "123147", Name: "Alana Rey"},
		},
		Directors: []director{
			{Name: "Casandra Keyes"},
		},
		Categories: []category{
			{Name: "Playboy Muses"},
		},
		MulticontentData: multicontentData{
			NSFW: []multicontentImage{
				{File: "photoset-150693-contentHero-1-nsfw.jpg", Name: "contentHero", Width: "1920", Height: "810"},
				{File: "photoset-150693-halfCard-4-nsfw.jpg", Name: "halfCard", Width: "1027", Height: "683"},
			},
		},
		RatingsUp: 46,
	}

	got := toScene("https://www.playboyplus.com/", hit, parseDate("2026-06-09"))

	if got.ID != "150693" {
		t.Errorf("ID = %q", got.ID)
	}
	if got.SiteID != "playboyplus" {
		t.Errorf("SiteID = %q", got.SiteID)
	}
	if got.Title != "Alana Rey in Retro Radiance" {
		t.Errorf("Title = %q", got.Title)
	}
	wantURL := "https://www.playboyplus.com/en/update/Alana-Rey-in-Retro-Radiance/150693"
	if got.URL != wantURL {
		t.Errorf("URL = %q, want %q", got.URL, wantURL)
	}
	if got.Studio != "Playboy Plus" {
		t.Errorf("Studio = %q", got.Studio)
	}
	wantDesc := "First line.\nSecond line.& more"
	if got.Description != wantDesc {
		t.Errorf("Description = %q, want %q", got.Description, wantDesc)
	}
	wantThumb := imageCDN + "/photoset-150693-contentHero-1-nsfw.jpg"
	if got.Thumbnail != wantThumb {
		t.Errorf("Thumbnail = %q, want %q", got.Thumbnail, wantThumb)
	}
	if len(got.Performers) != 1 || got.Performers[0] != "Alana Rey" {
		t.Errorf("Performers = %v", got.Performers)
	}
	if got.Director != "Casandra Keyes" {
		t.Errorf("Director = %q", got.Director)
	}
	if len(got.Tags) != 1 || got.Tags[0] != "Playboy Muses" {
		t.Errorf("Tags = %v", got.Tags)
	}
	if got.Series != "Playboy Plus" {
		t.Errorf("Series = %q", got.Series)
	}
	if got.Likes != 46 {
		t.Errorf("Likes = %d", got.Likes)
	}
}

func TestBestThumbnail(t *testing.T) {
	t.Run("prefers contentHero NSFW", func(t *testing.T) {
		mc := multicontentData{
			NSFW: []multicontentImage{
				{File: "half.jpg", Name: "halfCard"},
				{File: "hero.jpg", Name: "contentHero"},
			},
		}
		got := bestThumbnail(mc)
		want := imageCDN + "/hero.jpg"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("falls back to halfCard", func(t *testing.T) {
		mc := multicontentData{
			NSFW: []multicontentImage{
				{File: "half.jpg", Name: "halfCard"},
			},
		}
		got := bestThumbnail(mc)
		want := imageCDN + "/half.jpg"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("falls back to SFW when no NSFW", func(t *testing.T) {
		mc := multicontentData{
			SFW: []multicontentImage{
				{File: "sfw.jpg", Name: "contentHero"},
			},
		}
		got := bestThumbnail(mc)
		want := imageCDN + "/sfw.jpg"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("empty returns empty", func(t *testing.T) {
		if got := bestThumbnail(multicontentData{}); got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})
}

func TestParseDate(t *testing.T) {
	cases := []struct {
		in   string
		zero bool
	}{
		{"2026-06-08", false},
		{"", true},
		{"not-a-date", true},
	}
	for _, c := range cases {
		got := parseDate(c.in)
		if got.IsZero() != c.zero {
			t.Errorf("parseDate(%q).IsZero() = %v, want %v", c.in, got.IsZero(), c.zero)
		}
	}
}

func TestRunPagination(t *testing.T) {
	hitPage0 := []photosetHit{
		{SetID: 1, Title: "Set One", DateOnline: "2026-01-15", Actors: []actor{{Name: "Model A"}}},
		{SetID: 2, Title: "Set Two", DateOnline: "2026-01-14", Actors: []actor{{Name: "Model B"}}},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/en/updates":
			_, _ = w.Write([]byte(`<html><script>"algolia":{"apiKey":"testkey123"}</script></html>`))
		default:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(algoliaResponse{Hits: hitPage0, NbHits: 2})
		}
	}))
	defer ts.Close()

	s := &Scraper{Client: ts.Client(), algoliaHost: ts.URL, siteBaseURL: ts.URL}

	ctx := context.Background()
	out := make(chan scraper.SceneResult, 10)
	go s.run(ctx, "https://www.playboyplus.com/", scraper.ListOpts{}, out)

	scenes := testutil.CollectScenes(t, out)
	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2", len(scenes))
	}
	if scenes[0].ID != "1" || scenes[1].ID != "2" {
		t.Errorf("scene IDs = %q, %q", scenes[0].ID, scenes[1].ID)
	}
}

// --- golden fixture ----------------------------------------------------------
//
// The other tests build photosetHit values in Go, so encode and decode share the
// struct tag and a renamed one round-trips unnoticed. This is a byte-verbatim
// slice of a live Algolia response (the first two of 15606 hits; the paging
// counters copied from the same body).
//
// No credential is embedded: the Algolia search key is public and scraped from
// the site's own page source at runtime, exactly as fetchAPIKey does — the same
// arrangement as gammautil. TestGoldenAlgoliaCarriesNoKey asserts the fixture
// does not contain it.
//
// Shapes a hand-written fixture would have got wrong:
//   - **`set_id` (int 151603) and `set_path` (string "190476") are different
//     numbers.** Scene.ID and the scene URL are built from set_id; set_path is a
//     media path that also appears inside objectID. Picking the wrong one still
//     produces a plausible-looking URL that 404s.
//   - `num_of_pictures` is a **string** ("30"), not a number.
//   - `actors[].actor_id` is a **string** ("121963") in an otherwise numeric
//     record — the same mixed-typing gammautil's index shows.
//   - `multicontent_data` is keyed by `sfw`/`nsfw` rather than by size.
//   - Algolia's generated `_highlightResult` is retained here rather than dropped:
//     at 737 bytes it is not the bloat it was in gammautil, and keeping it covers
//     unknown-field tolerance.
func TestGoldenAlgoliaQuery(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("testdata", "algolia_query.json"))
	if err != nil {
		t.Fatal(err)
	}

	var resp algoliaResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("decoding captured payload: %v", err)
	}
	if len(resp.Hits) != 2 {
		t.Fatalf("decoded %d hits, want 2", len(resp.Hits))
	}
	if resp.NbHits != 15606 {
		t.Errorf("NbHits = %d (nbHits), want 15606 — paging depends on it", resp.NbHits)
	}

	h := resp.Hits[0]
	if h.SetID != 151603 {
		t.Errorf("SetID = %d (set_id), want 151603", h.SetID)
	}
	if h.Title != "Floral Garden" {
		t.Errorf("Title = %q (title)", h.Title)
	}
	if h.URLTitle != "Floral-Garden" {
		t.Errorf("URLTitle = %q (url_title) — the scene URL is built from it", h.URLTitle)
	}
	if h.DateOnline != "2026-07-25" {
		t.Errorf("DateOnline = %q (date_online), want a bare Y-m-d", h.DateOnline)
	}
	if _, err := time.Parse("2006-01-02", h.DateOnline); err != nil {
		t.Errorf("date_online %q does not parse as Y-m-d: %v", h.DateOnline, err)
	}
	if h.NumOfPictures != "30" {
		t.Errorf("NumOfPictures = %q (num_of_pictures), want the string \"30\"", h.NumOfPictures)
	}
	if h.SiteName != "playboyplus" {
		t.Errorf("SiteName = %q (sitename)", h.SiteName)
	}
	if h.ObjectID == "" {
		t.Error("ObjectID is empty (objectID)")
	}
	if len(h.Actors) == 0 || h.Actors[0].Name == "" {
		t.Fatalf("actors = %+v (actors[].name)", h.Actors)
	}
	// actor_id is a string even though it looks numeric.
	if h.Actors[0].ActorID != "121963" {
		t.Errorf("actor ActorID = %q (actors[].actor_id), want the string \"121963\"", h.Actors[0].ActorID)
	}
	if bestThumbnail(h.MulticontentData) == "" {
		t.Error("bestThumbnail found nothing in multicontent_data — every scene loses its thumbnail")
	}

	// The set_id / set_path distinction, stated as the invariant it is.
	if got := strconv.Itoa(h.SetID); got == "190476" {
		t.Error("SetID now equals set_path; the two were different values when captured, " +
			"and the scene URL depends on set_id")
	}
}

func TestGoldenAlgoliaCarriesNoKey(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("testdata", "algolia_query.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{"x-algolia-api-key", "apiKey", "algolia\":{"} {
		if bytes.Contains(body, []byte(marker)) {
			t.Errorf("fixture contains %q — the search key belongs in the request, not the body", marker)
		}
	}
	if !bytes.Contains(body, []byte(`"num_of_pictures":"30"`)) {
		t.Error(`fixture lost the quoted "num_of_pictures":"30" — a re-encode may have made it numeric`)
	}
}
