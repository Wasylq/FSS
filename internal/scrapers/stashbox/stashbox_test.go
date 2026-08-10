package stashbox

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Anastylosis/FSS/scraper"
)

func setupTestInstance(ts *httptest.Server) {
	u, _ := url.Parse(ts.URL)
	instancesOnce.Do(func() {}) // mark as done so real config isn't loaded
	instances = []instance{{
		graphqlURL: ts.URL + "/graphql",
		apiKey:     "test-key",
		host:       u.Host,
		baseURL:    ts.URL,
		siteID:     "teststash",
	}}
}

func strPtr(s string) *string { return &s }
func intPtr(i int) *int       { return &i }

func sampleGQLScene(id, title string) gqlScene {
	return gqlScene{
		ID:          id,
		Title:       strPtr(title),
		Details:     strPtr("A test scene."),
		ReleaseDate: strPtr("2025-06-15"),
		Duration:    intPtr(1800),
		Director:    strPtr("Test Director"),
		Code:        strPtr("TEST-001"),
		Studio:      &gqlStudio{ID: "s1", Name: "Test Studio"},
		Tags:        []gqlTag{{Name: "Tag1"}, {Name: "Tag2"}},
		Performers: []gqlAppearance{
			{Performer: gqlPerformer{Name: "Jane Doe"}, As: nil},
			{Performer: gqlPerformer{Name: "Real Name"}, As: strPtr("Stage Name")},
		},
		Images: []gqlImage{
			{URL: "https://img.example.com/scene1.jpg", Width: 1920, Height: 1080},
		},
		URLs: []gqlURL{
			{URL: "https://example.com/scenes/test-001", Site: gqlSite{Name: "Studio"}},
		},
	}
}

func gqlResponseJSON(scenes []gqlScene, count int) string {
	resp := gqlResponse{}
	resp.Data.QueryScenes.Count = count
	resp.Data.QueryScenes.Scenes = scenes
	b, _ := json.Marshal(resp)
	return string(b)
}

// ---- tests ----

func TestDeriveSiteID(t *testing.T) {
	tests := []struct {
		host string
		want string
	}{
		{"stashdb.org", "stashdb"},
		{"www.stashdb.org", "stashdb"},
		{"pmvstash.org", "pmvstash"},
		{"localhost:9999", "localhost"},
		{"stash", "stash"},
	}
	for _, tt := range tests {
		if got := deriveSiteID(tt.host); got != tt.want {
			t.Errorf("deriveSiteID(%q) = %q, want %q", tt.host, got, tt.want)
		}
	}
}

func TestMatchesURL(t *testing.T) {
	ts := httptest.NewServer(nil)
	defer ts.Close()
	setupTestInstance(ts)

	s := New()
	u, _ := url.Parse(ts.URL)

	tests := []struct {
		url  string
		want bool
	}{
		{ts.URL + "/performers/7a0f7c42-7c45-4fce-911d-0bfbf293707d", true},
		{ts.URL + "/studios/eec95cdd-9f58-4fc7-b7d1-e98786453d27", true},
		{ts.URL + "/scenes/abc-123", false},
		{ts.URL + "/performers/not-a-uuid", false},
		{ts.URL + "/", false},
		{"https://other.example.com/performers/7a0f7c42-7c45-4fce-911d-0bfbf293707d", false},
	}
	_ = u
	for _, tt := range tests {
		if got := s.MatchesURL(tt.url); got != tt.want {
			t.Errorf("MatchesURL(%q) = %v, want %v", tt.url, got, tt.want)
		}
	}
}

func TestBuildInput(t *testing.T) {
	t.Run("performer", func(t *testing.T) {
		input := buildInput("performers", "abc-123", 1)
		performers, ok := input["performers"].(map[string]any)
		if !ok {
			t.Fatal("missing performers filter")
		}
		if performers["modifier"] != "INCLUDES" {
			t.Errorf("modifier = %v", performers["modifier"])
		}
		vals := performers["value"].([]string)
		if len(vals) != 1 || vals[0] != "abc-123" {
			t.Errorf("value = %v", vals)
		}
		if input["page"] != 1 || input["per_page"] != 100 {
			t.Errorf("pagination wrong: %v", input)
		}
	})

	t.Run("studio", func(t *testing.T) {
		input := buildInput("studios", "xyz-456", 3)
		if input["parentStudio"] != "xyz-456" {
			t.Errorf("parentStudio = %v", input["parentStudio"])
		}
		if input["page"] != 3 {
			t.Errorf("page = %v", input["page"])
		}
	})
}

func TestToScene(t *testing.T) {
	inst := instance{
		baseURL: "https://stashdb.org",
		siteID:  "stashdb",
	}
	gs := sampleGQLScene("scene-uuid-1", "Test Scene Title")
	scene := toScene("https://stashdb.org/studios/s1", inst, gs)

	if scene.ID != "scene-uuid-1" {
		t.Errorf("ID = %q", scene.ID)
	}
	if scene.SiteID != "stashdb" {
		t.Errorf("SiteID = %q", scene.SiteID)
	}
	if scene.Title != "Test Scene Title" {
		t.Errorf("Title = %q", scene.Title)
	}
	if scene.Description != "A test scene." {
		t.Errorf("Description = %q", scene.Description)
	}
	wantDate := time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC)
	if !scene.Date.Equal(wantDate) {
		t.Errorf("Date = %v", scene.Date)
	}
	if scene.Duration != 1800 {
		t.Errorf("Duration = %d", scene.Duration)
	}
	if scene.Director != "Test Director" {
		t.Errorf("Director = %q", scene.Director)
	}
	if scene.Studio != "Test Studio" {
		t.Errorf("Studio = %q", scene.Studio)
	}
	if len(scene.Performers) != 2 || scene.Performers[0] != "Jane Doe" || scene.Performers[1] != "Stage Name" {
		t.Errorf("Performers = %v", scene.Performers)
	}
	if len(scene.Tags) != 2 || scene.Tags[0] != "Tag1" {
		t.Errorf("Tags = %v", scene.Tags)
	}
	if scene.Thumbnail != "https://img.example.com/scene1.jpg" {
		t.Errorf("Thumbnail = %q", scene.Thumbnail)
	}
	if scene.URL != "https://stashdb.org/scenes/scene-uuid-1" {
		t.Errorf("URL = %q", scene.URL)
	}
}

func TestToSceneNoOptionalFields(t *testing.T) {
	inst := instance{baseURL: "https://stashdb.org", siteID: "stashdb"}
	gs := gqlScene{ID: "bare-uuid"}
	scene := toScene("https://stashdb.org/studios/s1", inst, gs)

	if scene.ID != "bare-uuid" {
		t.Errorf("ID = %q", scene.ID)
	}
	if scene.Title != "" {
		t.Errorf("Title should be empty, got %q", scene.Title)
	}
	if !scene.Date.IsZero() {
		t.Errorf("Date should be zero, got %v", scene.Date)
	}
}

func TestToSceneBestImage(t *testing.T) {
	inst := instance{baseURL: "https://stashdb.org", siteID: "stashdb"}
	gs := gqlScene{
		ID: "img-uuid",
		Images: []gqlImage{
			{URL: "https://img.example.com/small.jpg", Width: 640, Height: 480},
			{URL: "https://img.example.com/large.jpg", Width: 3840, Height: 2160},
			{URL: "https://img.example.com/medium.jpg", Width: 1920, Height: 1080},
		},
	}
	scene := toScene("https://stashdb.org/studios/s1", inst, gs)
	if scene.Thumbnail != "https://img.example.com/large.jpg" {
		t.Errorf("should pick widest image, got %q", scene.Thumbnail)
	}
}

func TestListScenes(t *testing.T) {
	scenes := []gqlScene{
		sampleGQLScene("uuid-1", "Scene 1"),
		sampleGQLScene("uuid-2", "Scene 2"),
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("ApiKey") != "test-key" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		_, _ = fmt.Fprint(w, gqlResponseJSON(scenes, 2))
	}))
	defer ts.Close()
	setupTestInstance(ts)

	s := New()
	studioURL := ts.URL + "/studios/eec95cdd-9f58-4fc7-b7d1-e98786453d27"
	ch, err := s.ListScenes(context.Background(), studioURL, scraper.ListOpts{Delay: time.Millisecond})
	if err != nil {
		t.Fatal(err)
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

func TestListScenesKnownIDs(t *testing.T) {
	scenes := []gqlScene{
		sampleGQLScene("new-uuid", "New Scene"),
		sampleGQLScene("known-uuid", "Known Scene"),
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, gqlResponseJSON(scenes, 2))
	}))
	defer ts.Close()
	setupTestInstance(ts)

	s := New()
	studioURL := ts.URL + "/performers/7a0f7c42-7c45-4fce-911d-0bfbf293707d"
	ch, err := s.ListScenes(context.Background(), studioURL, scraper.ListOpts{
		Delay:    time.Millisecond,
		KnownIDs: map[string]bool{"known-uuid": true},
	})
	if err != nil {
		t.Fatal(err)
	}

	var scenes2, stopped int
	for r := range ch {
		switch r.Kind {
		case scraper.KindScene:
			scenes2++
		case scraper.KindStoppedEarly:
			stopped++
		case scraper.KindError:
			t.Errorf("unexpected error: %v", r.Err)
		}
	}
	if scenes2 != 1 {
		t.Errorf("got %d scenes, want 1", scenes2)
	}
	if stopped != 1 {
		t.Errorf("got %d stopped, want 1", stopped)
	}
}

func TestListScenesPagination(t *testing.T) {
	page1 := make([]gqlScene, perPage)
	for i := range page1 {
		page1[i] = gqlScene{ID: fmt.Sprintf("p1-%03d", i), Title: strPtr("Title")}
	}
	page2 := []gqlScene{{ID: "p2-000", Title: strPtr("Title")}}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req gqlRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		input := req.Variables["input"].(map[string]any)
		page := int(input["page"].(float64))
		if page == 2 {
			_, _ = fmt.Fprint(w, gqlResponseJSON(page2, 101))
		} else {
			_, _ = fmt.Fprint(w, gqlResponseJSON(page1, 101))
		}
	}))
	defer ts.Close()
	setupTestInstance(ts)

	s := New()
	studioURL := ts.URL + "/studios/eec95cdd-9f58-4fc7-b7d1-e98786453d27"
	ch, err := s.ListScenes(context.Background(), studioURL, scraper.ListOpts{Delay: time.Millisecond})
	if err != nil {
		t.Fatal(err)
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
	if count != 101 {
		t.Errorf("got %d scenes, want 101", count)
	}
}

func TestListScenesNoConfig(t *testing.T) {
	instancesOnce.Do(func() {})
	instances = nil

	s := New()
	_, err := s.ListScenes(context.Background(), "https://stashdb.org/studios/abc", scraper.ListOpts{})
	if err == nil {
		t.Fatal("expected error for unconfigured host")
	}
}

// --- golden fixture ----------------------------------------------------------
//
// The other tests build gqlScene values in Go, so encode and decode share the
// struct tag and a renamed tag round-trips unnoticed. This one decodes a
// queryScenes response captured verbatim from stashdb.org and runs it through
// toScene, so a tag break fails here.
//
// The fixture carries no credential — the API key travels in the ApiKey request
// header, never in the response — and that is asserted below rather than assumed.
//
// Notable shapes: almost every scalar is a *pointer* (nullable in the schema), so
// a wrong tag yields a silent nil rather than a type error; performers are nested
// two deep (`performers[].performer.name`) with a separate `as` alias that is null
// here; and `release_date` is a date string while `duration` is an int.
func TestGoldenQueryScenes(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("testdata", "query_scenes.json"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(bytes.ToLower(body), []byte("apikey")) {
		t.Fatal("fixture appears to contain a credential — it must not be committed")
	}

	var resp gqlResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("decoding captured payload: %v", err)
	}
	if len(resp.Errors) != 0 {
		t.Fatalf("captured payload carries GraphQL errors: %+v", resp.Errors)
	}
	if resp.Data.QueryScenes.Count == 0 {
		t.Error("count is 0 (data.queryScenes.count) — pagination depends on it")
	}
	if len(resp.Data.QueryScenes.Scenes) != 2 {
		t.Fatalf("decoded %d scenes, want 2", len(resp.Data.QueryScenes.Scenes))
	}

	inst := instance{baseURL: "https://stashdb.org", siteID: "stashdb"}
	sc := toScene("https://stashdb.org/studios/x", inst, resp.Data.QueryScenes.Scenes[0])

	if sc.ID != "019f7e7a-f725-7628-8930-8dc79cd82b1d" {
		t.Errorf("ID = %q (scenes[].id)", sc.ID)
	}
	if sc.Title != "START-608" {
		t.Errorf("Title = %q (scenes[].title, a *string)", sc.Title)
	}
	if sc.Date.Format("2006-01-02") != "2026-08-06" {
		t.Errorf("Date = %v (scenes[].release_date), want 2026-08-06", sc.Date)
	}
	if sc.Duration != 8916 {
		t.Errorf("Duration = %d (scenes[].duration), want 8916", sc.Duration)
	}
	if sc.Director != "Chinpo Dazai" {
		t.Errorf("Director = %q (scenes[].director)", sc.Director)
	}
	if sc.Studio != "SOD Create" {
		t.Errorf("Studio = %q (scenes[].studio.name)", sc.Studio)
	}
	if sc.Description == "" {
		t.Error("Description is empty (scenes[].details)")
	}
	// performers[].performer.name — nested two deep.
	if len(sc.Performers) != 1 || sc.Performers[0] != "Rei Kamiki" {
		t.Errorf("Performers = %v (scenes[].performers[].performer.name)", sc.Performers)
	}
	if len(sc.Tags) == 0 {
		t.Error("Tags is empty (scenes[].tags[].name)")
	}
	if sc.Thumbnail == "" {
		t.Error("Thumbnail is empty (scenes[].images[].url)")
	}
}
