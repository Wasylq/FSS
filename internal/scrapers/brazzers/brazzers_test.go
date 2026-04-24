package brazzers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wasylq/FSS/internal/scrapers/ayloutil"
	"github.com/Wasylq/FSS/scraper"
)

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url  string
		want bool
	}{
		{"https://www.brazzers.com", true},
		{"https://brazzers.com/pornstar/2719/reagan-foxx", true},
		{"https://www.brazzers.com/category/79/milf", true},
		{"https://www.brazzers.com/site/96/brazzers-exxtra", true},
		{"https://www.brazzers.com/series/11174721/milf-quest", true},
		{"https://www.brazzers.com/video/11205761/leather-and-bubble-butts", true},
		{"https://www.pornhub.com", false},
		{"https://example.com", false},
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
		{"https://www.brazzers.com", ayloutil.FilterAll, 0},
		{"https://www.brazzers.com/pornstar/2719/reagan-foxx", ayloutil.FilterActor, 2719},
		{"https://www.brazzers.com/category/79/milf", ayloutil.FilterTag, 79},
		{"https://www.brazzers.com/site/96/brazzers-exxtra", ayloutil.FilterCollection, 96},
		{"https://www.brazzers.com/series/11174721/milf-quest", ayloutil.FilterSeries, 11174721},
		{"https://www.brazzers.com/videos", ayloutil.FilterAll, 0},
	}
	for _, c := range cases {
		f := ayloutil.ParseFilter(c.url)
		if f.Type != c.wantType || f.ID != c.wantID {
			t.Errorf("ParseFilter(%q) = {Type:%d, ID:%d}, want {Type:%d, ID:%d}",
				c.url, f.Type, f.ID, c.wantType, c.wantID)
		}
	}
}

func TestSlugify(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"Leather And Bubble Butts", "leather-and-bubble-butts"},
		{"MILF Quest", "milf-quest"},
		{"Something (with parens)", "something-with-parens"},
		{"  Spaces  ", "spaces"},
		{"Apostrophe's Test!", "apostrophe-s-test"},
	}
	for _, c := range cases {
		if got := ayloutil.Slugify(c.input); got != c.want {
			t.Errorf("Slugify(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

func TestParseDate(t *testing.T) {
	cases := []struct {
		input string
		year  int
		month time.Month
		day   int
	}{
		{"2025-07-01T12:00:00+00:00", 2025, 7, 1},
		{"2026-01-15T00:00:00+00:00", 2026, 1, 15},
		{"", 1, 1, 1},
	}
	for _, c := range cases {
		d := ayloutil.ParseDate(c.input)
		if c.input == "" {
			if !d.IsZero() {
				t.Errorf("ParseDate(%q) should be zero", c.input)
			}
			continue
		}
		if d.Year() != c.year || d.Month() != c.month || d.Day() != c.day {
			t.Errorf("ParseDate(%q) = %v", c.input, d)
		}
	}
}

func TestThumbnailURL(t *testing.T) {
	cases := []struct {
		name string
		raw  json.RawMessage
		want string
	}{
		{
			name: "poster with xl",
			raw:  json.RawMessage(`{"poster":{"0":{"xl":{"width":1280,"height":720,"url":"https://cdn.example.com/poster.jpg","urls":{"default":"https://cdn.example.com/poster_xl.jpg"}}}}}`),
			want: "https://cdn.example.com/poster_xl.jpg",
		},
		{
			name: "poster with xx",
			raw:  json.RawMessage(`{"poster":{"0":{"xx":{"urls":{"default":"https://cdn.example.com/poster_xx.jpg"}},"xl":{"urls":{"default":"https://cdn.example.com/poster_xl.jpg"}}}}}`),
			want: "https://cdn.example.com/poster_xx.jpg",
		},
		{
			name: "empty array",
			raw:  json.RawMessage(`[]`),
			want: "",
		},
		{
			name: "empty object",
			raw:  json.RawMessage(`{}`),
			want: "",
		},
		{
			name: "null",
			raw:  nil,
			want: "",
		},
	}
	for _, c := range cases {
		if got := ayloutil.ThumbnailURL(c.raw); got != c.want {
			t.Errorf("%s: ThumbnailURL = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestPreviewURL(t *testing.T) {
	cases := []struct {
		name string
		raw  json.RawMessage
		want string
	}{
		{
			name: "mediabook with 720p and 320p",
			raw:  json.RawMessage(`{"mediabook":{"length":1815,"files":{"720p":{"format":"720p","urls":{"view":"https://cdn.example.com/720p.mp4"}},"320p":{"format":"320p","urls":{"view":"https://cdn.example.com/320p.mp4"}}}}}`),
			want: "https://cdn.example.com/720p.mp4",
		},
		{
			name: "empty array",
			raw:  json.RawMessage(`[]`),
			want: "",
		},
		{
			name: "empty object",
			raw:  json.RawMessage(`{}`),
			want: "",
		},
		{
			name: "null",
			raw:  nil,
			want: "",
		},
	}
	for _, c := range cases {
		if got := ayloutil.PreviewURL(c.raw); got != c.want {
			t.Errorf("%s: PreviewURL = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestMediaDuration(t *testing.T) {
	cases := []struct {
		name string
		raw  json.RawMessage
		want int
	}{
		{
			name: "mediabook with length",
			raw:  json.RawMessage(`{"mediabook":{"length":1815,"files":{}}}`),
			want: 1815,
		},
		{
			name: "empty array",
			raw:  json.RawMessage(`[]`),
			want: 0,
		},
		{
			name: "null",
			raw:  nil,
			want: 0,
		},
	}
	for _, c := range cases {
		if got := ayloutil.MediaDuration(c.raw); got != c.want {
			t.Errorf("%s: MediaDuration = %d, want %d", c.name, got, c.want)
		}
	}
}

func TestToScene(t *testing.T) {
	rel := ayloutil.Release{
		ID:           11205761,
		Type:         "scene",
		Title:        "Leather And Bubble Butts",
		Description:  "Scene description here",
		DateReleased: "2025-07-01T12:00:00+00:00",
		Actors: []ayloutil.Actor{
			{ID: 15645, Name: "Ricky Johnson", Gender: "male"},
			{ID: 1234, Name: "Brittany Andrews", Gender: "female"},
		},
		Collections: []ayloutil.Collection{
			{ID: 96, Name: "Brazzers Exxtra", ShortName: "bex"},
		},
		Tags: []ayloutil.Tag{
			{ID: 365, Name: "Muscular Man", Category: "Body Type"},
			{ID: 79, Name: "MILF", Category: ""},
		},
		Stats: ayloutil.Stats{Likes: 390, Views: 25153},
		RawImages: json.RawMessage(`{"poster":{"0":{"xl":{"urls":{"default":"https://cdn.example.com/poster.jpg"}}}}}`),
		RawVideos: json.RawMessage(`{"mediabook":{"length":1815,"files":{"720p":{"format":"720p","urls":{"view":"https://cdn.example.com/trailer.mp4"}}}}}`),
	}

	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	scene := ayloutil.ToScene(config, "https://www.brazzers.com/pornstar/2719/reagan-foxx", rel, now)

	if scene.ID != "11205761" {
		t.Errorf("ID = %q, want 11205761", scene.ID)
	}
	if scene.SiteID != "brazzers" {
		t.Errorf("SiteID = %q", scene.SiteID)
	}
	if scene.Title != "Leather And Bubble Butts" {
		t.Errorf("Title = %q", scene.Title)
	}
	if scene.URL != "https://www.brazzers.com/video/11205761/leather-and-bubble-butts" {
		t.Errorf("URL = %q", scene.URL)
	}
	if scene.Date.Year() != 2025 || scene.Date.Month() != 7 || scene.Date.Day() != 1 {
		t.Errorf("Date = %v", scene.Date)
	}
	if scene.Description != "Scene description here" {
		t.Errorf("Description = %q", scene.Description)
	}
	if scene.Thumbnail != "https://cdn.example.com/poster.jpg" {
		t.Errorf("Thumbnail = %q", scene.Thumbnail)
	}
	if scene.Preview != "https://cdn.example.com/trailer.mp4" {
		t.Errorf("Preview = %q", scene.Preview)
	}
	if len(scene.Performers) != 2 || scene.Performers[0] != "Ricky Johnson" {
		t.Errorf("Performers = %v", scene.Performers)
	}
	if scene.Studio != "Brazzers" {
		t.Errorf("Studio = %q", scene.Studio)
	}
	if len(scene.Tags) != 2 || scene.Tags[0] != "Muscular Man" {
		t.Errorf("Tags = %v", scene.Tags)
	}
	if scene.Series != "Brazzers Exxtra" {
		t.Errorf("Series = %q", scene.Series)
	}
	if scene.Duration != 1815 {
		t.Errorf("Duration = %d, want 1815", scene.Duration)
	}
	if scene.Likes != 390 {
		t.Errorf("Likes = %d", scene.Likes)
	}
	if scene.Views != 25153 {
		t.Errorf("Views = %d", scene.Views)
	}
	if scene.StudioURL != "https://www.brazzers.com/pornstar/2719/reagan-foxx" {
		t.Errorf("StudioURL = %q", scene.StudioURL)
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

func TestFetchToken(t *testing.T) {
	ts := newTestServer(nil, 0)
	defer ts.Close()

	cfg := ayloutil.SiteConfig{SiteID: "brazzers", SiteBase: ts.URL, StudioName: "Brazzers"}
	a := &ayloutil.Scraper{Client: ts.Client(), Config: cfg, APIHost: ts.URL}

	token, err := a.FetchToken(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if token != "test-token" {
		t.Errorf("token = %q, want test-token", token)
	}
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

	cfg := ayloutil.SiteConfig{SiteID: "brazzers", SiteBase: ts.URL, StudioName: "Brazzers"}
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
			RawImages: json.RawMessage(`[]`), RawVideos: json.RawMessage(`[]`),
		},
		{
			ID: 2002, Type: "scene", Title: "Known Scene",
			DateReleased: "2026-01-01T12:00:00+00:00",
			RawImages: json.RawMessage(`[]`), RawVideos: json.RawMessage(`[]`),
		},
	}

	ts := newTestServer(releases, 2)
	defer ts.Close()

	cfg := ayloutil.SiteConfig{SiteID: "brazzers", SiteBase: ts.URL, StudioName: "Brazzers"}
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

func TestListScenesActorFilter(t *testing.T) {
	releases := []ayloutil.Release{
		{
			ID: 3001, Type: "scene", Title: "Actor Scene",
			DateReleased: "2026-01-15T12:00:00+00:00",
			Actors:       []ayloutil.Actor{{ID: 2719, Name: "Reagan Foxx"}},
			RawImages:    json.RawMessage(`[]`), RawVideos: json.RawMessage(`[]`),
		},
	}

	var gotActorID string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			http.SetCookie(w, &http.Cookie{Name: "instance_token", Value: "test-token"})
			_, _ = w.Write([]byte("<html></html>"))
		case "/v2/releases":
			gotActorID = r.URL.Query().Get("actorId")
			resp := ayloutil.ReleasesResponse{
				Meta:   ayloutil.APIMeta{Count: 1, Total: 1},
				Result: releases,
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	cfg := ayloutil.SiteConfig{SiteID: "brazzers", SiteBase: ts.URL, StudioName: "Brazzers"}
	s := &Scraper{aylo: &ayloutil.Scraper{Client: ts.Client(), Config: cfg, APIHost: ts.URL}}

	studioURL := ts.URL + "/pornstar/2719/reagan-foxx"
	ch, err := s.ListScenes(context.Background(), studioURL, scraper.ListOpts{})
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

	if gotActorID != "2719" {
		t.Errorf("API called with actorId=%q, want 2719", gotActorID)
	}
	if len(scenes) != 1 || scenes[0] != "Actor Scene" {
		t.Errorf("scenes = %v", scenes)
	}
}

func TestListScenesSeries(t *testing.T) {
	seriesReleases := []ayloutil.Release{
		{
			ID:           99999,
			Type:         "serie",
			Title:        "Test Series",
			Description:  "Series description",
			DateReleased: "2026-03-01T12:00:00+00:00",
			Actors: []ayloutil.Actor{
				{ID: 1, Name: "Actor A"},
				{ID: 2, Name: "Actor B"},
			},
			Collections: []ayloutil.Collection{{ID: 96, Name: "Brazzers Exxtra"}},
			Tags:        []ayloutil.Tag{{ID: 1, Name: "Drama"}},
			RawImages:   json.RawMessage(`{"poster":{"0":{"xl":{"urls":{"default":"https://cdn.example.com/series.jpg"}}}}}`),
			RawVideos:   json.RawMessage(`[]`),
			Children: []ayloutil.Release{
				{
					ID: 100001, Type: "scene", Title: "Episode 1",
					RawImages: json.RawMessage(`[]`),
					RawVideos: json.RawMessage(`{"mediabook":{"length":1200,"files":{}}}`),
				},
				{
					ID: 100002, Type: "scene", Title: "Episode 2",
					RawImages: json.RawMessage(`[]`),
					RawVideos: json.RawMessage(`{"mediabook":{"length":1400,"files":{}}}`),
				},
			},
		},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			http.SetCookie(w, &http.Cookie{Name: "instance_token", Value: "test-token"})
			_, _ = w.Write([]byte("<html></html>"))
		case "/v2/releases":
			resp := ayloutil.ReleasesResponse{
				Meta:   ayloutil.APIMeta{Count: len(seriesReleases), Total: len(seriesReleases)},
				Result: seriesReleases,
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	cfg := ayloutil.SiteConfig{SiteID: "brazzers", SiteBase: ts.URL, StudioName: "Brazzers"}
	s := &Scraper{aylo: &ayloutil.Scraper{Client: ts.Client(), Config: cfg, APIHost: ts.URL}}

	studioURL := ts.URL + "/series/99999/test-series"
	ch, err := s.ListScenes(context.Background(), studioURL, scraper.ListOpts{})
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
		if r.Scene.Title == "Episode 1" {
			if len(r.Scene.Performers) != 2 {
				t.Errorf("Episode 1 performers = %v, want 2 (inherited from series)", r.Scene.Performers)
			}
			if r.Scene.Series != "Brazzers Exxtra" {
				t.Errorf("Episode 1 series = %q, want Brazzers Exxtra", r.Scene.Series)
			}
			if r.Scene.Thumbnail != "https://cdn.example.com/series.jpg" {
				t.Errorf("Episode 1 thumbnail = %q, want series poster", r.Scene.Thumbnail)
			}
			if r.Scene.Duration != 1200 {
				t.Errorf("Episode 1 duration = %d, want 1200", r.Scene.Duration)
			}
		}
	}

	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2", len(scenes))
	}
	if scenes[0] != "Episode 1" || scenes[1] != "Episode 2" {
		t.Errorf("scenes = %v", scenes)
	}
}
