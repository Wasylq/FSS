package propertysex

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/ayloutil"
	"github.com/Wasylq/FSS/scraper"
)

var _ scraper.StudioScraper = New()

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url  string
		want bool
	}{
		{"https://www.propertysex.com", true},
		{"https://propertysex.com/pornstar/123/some-star", true},
		{"https://www.propertysex.com/category/79/milf", true},
		{"https://www.propertysex.com/site/96/some-site", true},
		{"https://www.propertysex.com/series/1234/some-series", true},
		{"https://www.propertysex.com/video/12345/some-video", true},
		{"https://www.pornhub.com", false},
		{"https://example.com", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.want {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.want)
		}
	}
}

func newTestServer(releases []ayloutil.Release, total int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			http.SetCookie(w, &http.Cookie{
				Name:  "instance_token",
				Value: "test-token",
			})
			_, _ = w.Write([]byte("<html></html>"))
		case "/v2/releases":
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

func TestListScenes(t *testing.T) {
	releases := []ayloutil.Release{
		{
			ID:           1001,
			Type:         "scene",
			Title:        "Scene One",
			Description:  "First scene",
			DateReleased: "2026-01-15T12:00:00+00:00",
			Actors:       []ayloutil.Actor{{ID: 1, Name: "Actor A"}},
			Tags:         []ayloutil.Tag{{ID: 1, Name: "Tag1"}},
			RawImages:    json.RawMessage(`[]`),
			RawVideos:    json.RawMessage(`{"mediabook":{"length":600,"files":{}}}`),
		},
		{
			ID:           1002,
			Type:         "scene",
			Title:        "Scene Two",
			Description:  "Second scene",
			DateReleased: "2026-01-10T12:00:00+00:00",
			Actors:       []ayloutil.Actor{{ID: 2, Name: "Actor B"}},
			Tags:         []ayloutil.Tag{{ID: 2, Name: "Tag2"}},
			RawImages:    json.RawMessage(`[]`),
			RawVideos:    json.RawMessage(`{"mediabook":{"length":900,"files":{}}}`),
		},
	}

	ts := newTestServer(releases, 2)
	defer ts.Close()

	cfg := ayloutil.SiteConfig{SiteID: "propertysex", SiteBase: ts.URL, StudioName: "PropertySex"}
	s := &Scraper{aylo: &ayloutil.Scraper{Client: ts.Client(), Config: cfg, APIHost: ts.URL}}

	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}

	var scenes []string
	for r := range ch {
		if r.Total > 0 || r.StoppedEarly {
			continue
		}
		if r.Err != nil {
			t.Errorf("unexpected error: %v", r.Err)
			continue
		}
		scenes = append(scenes, r.Scene.Title)
	}

	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2", len(scenes))
	}
	if scenes[0] != "Scene One" || scenes[1] != "Scene Two" {
		t.Errorf("scenes = %v", scenes)
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

	ts := newTestServer(releases, 2)
	defer ts.Close()

	cfg := ayloutil.SiteConfig{SiteID: "propertysex", SiteBase: ts.URL, StudioName: "PropertySex"}
	s := &Scraper{aylo: &ayloutil.Scraper{Client: ts.Client(), Config: cfg, APIHost: ts.URL}}

	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{
		KnownIDs: map[string]bool{"2002": true},
	})
	if err != nil {
		t.Fatal(err)
	}

	var scenes []string
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
			t.Errorf("unexpected error: %v", r.Err)
			continue
		}
		scenes = append(scenes, r.Scene.ID)
	}

	if !stoppedEarly {
		t.Error("expected StoppedEarly signal")
	}
	if len(scenes) != 1 || scenes[0] != "2001" {
		t.Errorf("got scenes %v, want [2001]", scenes)
	}
}
