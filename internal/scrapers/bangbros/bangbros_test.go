package bangbros

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/ayloutil"
	"github.com/Wasylq/FSS/internal/scrapers/testutil"
	"github.com/Wasylq/FSS/scraper"
)

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url  string
		want bool
	}{
		{"https://www.bangbros.com", true},
		{"https://bangbros.com/model/1234/some-model", true},
		{"https://www.bangbros.com/category/79/milf", true},
		{"https://www.bangbros.com/category/brunette", true},
		{"https://www.bangbros.com/site/96/bang-bus", true},
		{"https://www.bangbros.com/websites/MomIsHorny", true},
		{"https://www.bangbros.com/series/11174721/some-series", true},
		{"https://www.bangbros.com/video/11205761/some-scene", true},
		{"https://www.brazzers.com", false},
		{"https://example.com", false},
		{"", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.want {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.want)
		}
	}
}

func TestParseFilter(t *testing.T) {
	cases := []struct {
		url      string
		wantType ayloutil.FilterType
		wantID   int
	}{
		{"https://www.bangbros.com", ayloutil.FilterAll, 0},
		{"https://www.bangbros.com/model/1234/some-model", ayloutil.FilterActor, 1234},
		{"https://www.bangbros.com/category/79/milf", ayloutil.FilterTag, 79},
		{"https://www.bangbros.com/site/96/bang-bus", ayloutil.FilterCollection, 96},
		{"https://www.bangbros.com/series/11174721/some-series", ayloutil.FilterSeries, 11174721},
		{"https://www.bangbros.com/videos", ayloutil.FilterAll, 0},
	}
	for _, c := range cases {
		f := ayloutil.ParseFilter(c.url)
		if f.Type != c.wantType || f.ID != c.wantID {
			t.Errorf("ParseFilter(%q) = {Type:%d, ID:%d}, want {Type:%d, ID:%d}",
				c.url, f.Type, f.ID, c.wantType, c.wantID)
		}
	}
}

// newResolvingServer returns a test server that handles both the token endpoint
// and the resolution API calls (tags + collection search).
func newResolvingServer(t *testing.T, releases []ayloutil.Release, total int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			http.SetCookie(w, &http.Cookie{Name: "instance_token", Value: "test-token"})
			_, _ = w.Write([]byte("<html></html>"))

		case "/v2/tags":
			name := strings.ToLower(r.URL.Query().Get("name"))
			tagMap := map[string]int{"brunette": 127, "milf": 79}
			id, ok := tagMap[name]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"result":[],"meta":{"count":0,"total":0}}`))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"meta":   map[string]int{"count": 1, "total": 1},
				"result": []map[string]any{{"id": id, "name": name}},
			})

		case "/v2/releases":
			search := r.URL.Query().Get("search")
			if search != "" {
				// Collection resolution: return a scene with matching collection.
				collectionMap := map[string]struct {
					id   int
					name string
				}{
					"momishorny": {116221, "MomIsHorny"},
					"bangbus":    {55001, "BangBus"},
				}
				key := strings.ToLower(search)
				col, ok := collectionMap[key]
				if !ok {
					_, _ = w.Write([]byte(`{"result":[],"meta":{"count":0,"total":0}}`))
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{
					"meta": map[string]int{"count": 1, "total": 1},
					"result": []map[string]any{{
						"id": 9999, "type": "scene", "title": "Test Scene",
						"collections": []map[string]any{{"id": col.id, "name": col.name}},
					}},
				})
				return
			}
			// Normal scene listing.
			resp := ayloutil.ReleasesResponse{
				Meta:   ayloutil.APIMeta{Count: len(releases), Total: total},
				Result: releases,
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)

		default:
			http.NotFound(w, r)
		}
	}))
}

func TestResolveSlugURL(t *testing.T) {
	ts := newResolvingServer(t, nil, 0)
	defer ts.Close()

	cfg := ayloutil.SiteConfig{SiteID: "bangbros", SiteBase: ts.URL, StudioName: "Bang Bros"}
	s := &Scraper{aylo: &ayloutil.Scraper{Client: ts.Client(), Config: cfg, APIHost: ts.URL}}

	cases := []struct {
		input   string
		want    string
		wantErr bool
	}{
		{
			input: "https://www.bangbros.com/websites/MomIsHorny",
			want:  "https://www.bangbros.com/site/116221/momishorny",
		},
		{
			input: "https://www.bangbros.com/category/brunette",
			want:  "https://www.bangbros.com/category/127/brunette",
		},
		{
			// Already has numeric ID — no resolution needed.
			input: "https://www.bangbros.com/model/395971/lisa-ann",
			want:  "https://www.bangbros.com/model/395971/lisa-ann",
		},
		{
			// Numeric category — no resolution needed.
			input: "https://www.bangbros.com/category/79/milf",
			want:  "https://www.bangbros.com/category/79/milf",
		},
		{
			input:   "https://www.bangbros.com/websites/UnknownSite",
			wantErr: true,
		},
	}

	for _, c := range cases {
		got, err := s.resolveSlugURL(context.Background(), c.input)
		if c.wantErr {
			if err == nil {
				t.Errorf("resolveSlugURL(%q) expected error, got %q", c.input, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("resolveSlugURL(%q) error: %v", c.input, err)
			continue
		}
		if got != c.want {
			t.Errorf("resolveSlugURL(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

func TestListScenes(t *testing.T) {
	releases := []ayloutil.Release{
		{
			ID: 1001, Type: "scene", Title: "Scene One",
			DateReleased: "2026-01-15T12:00:00+00:00",
			Actors:       []ayloutil.Actor{{ID: 1, Name: "Actor A"}},
			RawImages:    json.RawMessage(`[]`),
			RawVideos:    json.RawMessage(`{"mediabook":{"length":600,"files":{}}}`),
		},
		{
			ID: 1002, Type: "scene", Title: "Scene Two",
			DateReleased: "2026-01-10T12:00:00+00:00",
			Actors:       []ayloutil.Actor{{ID: 2, Name: "Actor B"}},
			RawImages:    json.RawMessage(`[]`),
			RawVideos:    json.RawMessage(`{"mediabook":{"length":900,"files":{}}}`),
		},
	}

	ts := newResolvingServer(t, releases, 2)
	defer ts.Close()

	cfg := ayloutil.SiteConfig{SiteID: "bangbros", SiteBase: ts.URL, StudioName: "Bang Bros"}
	s := &Scraper{aylo: &ayloutil.Scraper{Client: ts.Client(), Config: cfg, APIHost: ts.URL}}

	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}

	results := testutil.CollectScenes(t, ch)
	if len(results) != 2 {
		t.Fatalf("got %d scenes, want 2", len(results))
	}
	if results[0].Title != "Scene One" || results[1].Title != "Scene Two" {
		t.Errorf("scenes = %v", results)
	}
}

func TestListScenesKnownIDs(t *testing.T) {
	releases := []ayloutil.Release{
		{
			ID: 2001, Type: "scene", Title: "New Scene",
			DateReleased: "2026-02-01T12:00:00+00:00",
			RawImages:    json.RawMessage(`[]`), RawVideos: json.RawMessage(`[]`),
		},
		{
			ID: 2002, Type: "scene", Title: "Known Scene",
			DateReleased: "2026-01-01T12:00:00+00:00",
			RawImages:    json.RawMessage(`[]`), RawVideos: json.RawMessage(`[]`),
		},
	}

	ts := newResolvingServer(t, releases, 2)
	defer ts.Close()

	cfg := ayloutil.SiteConfig{SiteID: "bangbros", SiteBase: ts.URL, StudioName: "Bang Bros"}
	s := &Scraper{aylo: &ayloutil.Scraper{Client: ts.Client(), Config: cfg, APIHost: ts.URL}}

	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{
		KnownIDs: map[string]bool{"2002": true},
	})
	if err != nil {
		t.Fatal(err)
	}

	results, stoppedEarly := testutil.CollectScenesWithStop(t, ch)
	if !stoppedEarly {
		t.Error("expected StoppedEarly signal")
	}
	if len(results) != 1 || results[0].ID != "2001" {
		t.Errorf("got scenes %v, want [2001]", results)
	}
}
