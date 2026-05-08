package sexlikereal

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wasylq/FSS/scraper"
)

const fixtureList = `{
  "data": [
    {
      "id": 81236,
      "title": "Church Babe Gone Wicked",
      "label": "church-babe-gone-wicked-81236",
      "description": "A great VR scene description.",
      "date": 1746403200,
      "fullVideoLength": 3563,
      "thumbnailUrl": "https://cdn-vr.sexlikereal.com/images/81236/cover.webp",
      "studio": {"id": 224, "name": "SLR Originals", "label": "slr-originals-224"},
      "actors": [
        {"id": 7099, "name": "Molly Little", "label": "molly-little-7099"},
        {"id": 5405, "name": "Nikki Nuttz", "label": "nikki-nuttz-5405"}
      ]
    },
    {
      "id": 80000,
      "title": "Second Scene Title",
      "label": "second-scene-title-80000",
      "description": "Another description.",
      "date": 1746316800,
      "fullVideoLength": 1200,
      "thumbnailUrl": "https://cdn-vr.sexlikereal.com/images/80000/cover.webp",
      "studio": {"id": 100, "name": "Other Studio", "label": "other-studio-100"},
      "actors": [
        {"id": 1234, "name": "Jane Doe", "label": "jane-doe-1234"}
      ]
    }
  ],
  "meta": {
    "pagination": {"page": 1, "perPage": 36, "totalCount": 55000, "totalPages": 1528}
  }
}`

const fixtureDetail81236 = `{
  "data": {
    "categories": [
      {"id": 191, "name": "Blow job", "label": "blow-job-vr"},
      {"id": 801, "name": "3D", "label": "3d-vr-porn"}
    ],
    "price": {"type": "individual", "discounted": false, "amount": 12.99, "gross": 12.99}
  }
}`

const fixtureDetail80000 = `{
  "data": {
    "categories": [
      {"id": 200, "name": "Cowgirl", "label": "cowgirl-vr"}
    ],
    "price": {"type": "bulk", "discounted": false, "amount": 6.99, "gross": 6.99}
  }
}`

const fixtureEmpty = `{"data": [], "meta": {"pagination": {"page": 2, "perPage": 36, "totalCount": 2, "totalPages": 1}}}`

func newTestServer(listPages map[int]string, details map[int]string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v3/scenes" && r.URL.Query().Get("page") != "":
			page := 1
			if p := r.URL.Query().Get("page"); p != "" {
				if _, err := fmt.Sscanf(p, "%d", &page); err != nil {
					page = 1
				}
			}
			if body, ok := listPages[page]; ok {
				_, _ = fmt.Fprint(w, body)
				return
			}
			_, _ = fmt.Fprint(w, fixtureEmpty)
		case len(r.URL.Path) > len("/v3/scenes/"):
			idStr := r.URL.Path[len("/v3/scenes/"):]
			id := 0
			if _, err := fmt.Sscanf(idStr, "%d", &id); err == nil {
				if body, ok := details[id]; ok {
					_, _ = fmt.Fprint(w, body)
					return
				}
			}
			http.NotFound(w, r)
		default:
			_, _ = fmt.Fprint(w, fixtureEmpty)
		}
	}))
}

func collect(ch <-chan scraper.SceneResult) []scraper.SceneResult {
	var results []scraper.SceneResult
	for r := range ch {
		results = append(results, r)
	}
	return results
}

func TestResolveFilter(t *testing.T) {
	tests := []struct {
		url      string
		wantMode filterMode
		wantID   string
	}{
		{"https://www.sexlikereal.com/scenes", filterAll, ""},
		{"https://www.sexlikereal.com", filterAll, ""},
		{"https://www.sexlikereal.com/studios/slr-originals-224", filterStudio, "224"},
		{"https://www.sexlikereal.com/pornstars/molly-little-7099", filterModel, "7099"},
	}
	for _, tt := range tests {
		mode, id := resolveFilter(tt.url)
		if mode != tt.wantMode {
			t.Errorf("resolveFilter(%q) mode = %d, want %d", tt.url, mode, tt.wantMode)
		}
		if id != tt.wantID {
			t.Errorf("resolveFilter(%q) id = %q, want %q", tt.url, id, tt.wantID)
		}
	}
}

func TestMatchesURL(t *testing.T) {
	s := New()
	tests := []struct {
		url  string
		want bool
	}{
		{"https://www.sexlikereal.com/scenes", true},
		{"https://sexlikereal.com/studios/slr-originals-224", true},
		{"https://www.sexlikereal.com", true},
		{"https://example.com", false},
	}
	for _, tt := range tests {
		if got := s.MatchesURL(tt.url); got != tt.want {
			t.Errorf("MatchesURL(%q) = %v, want %v", tt.url, got, tt.want)
		}
	}
}

func TestListScenes(t *testing.T) {
	ts := newTestServer(
		map[int]string{1: fixtureList},
		map[int]string{81236: fixtureDetail81236, 80000: fixtureDetail80000},
	)
	defer ts.Close()

	s := &Scraper{client: ts.Client(), apiBaseURL: ts.URL}
	ch, err := s.ListScenes(context.Background(), "https://www.sexlikereal.com/scenes", scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}

	results := collect(ch)
	var scenes int
	for _, r := range results {
		switch r.Kind {
		case scraper.KindScene:
			scenes++
			if r.Scene.ID == "81236" {
				if r.Scene.Title != "Church Babe Gone Wicked" {
					t.Errorf("title = %q", r.Scene.Title)
				}
				if r.Scene.Studio != "SLR Originals" {
					t.Errorf("studio = %q", r.Scene.Studio)
				}
				if len(r.Scene.Performers) != 2 {
					t.Errorf("performers = %v", r.Scene.Performers)
				}
				if r.Scene.Duration != 3563 {
					t.Errorf("duration = %d, want 3563", r.Scene.Duration)
				}
				if len(r.Scene.Tags) != 2 || r.Scene.Tags[0] != "Blow job" {
					t.Errorf("tags = %v", r.Scene.Tags)
				}
				if r.Scene.URL != "https://www.sexlikereal.com/scenes/church-babe-gone-wicked-81236" {
					t.Errorf("url = %q", r.Scene.URL)
				}
				if r.Scene.LowestPrice != 12.99 {
					t.Errorf("lowestPrice = %f, want 12.99", r.Scene.LowestPrice)
				}
			}
		case scraper.KindError:
			t.Errorf("unexpected error: %v", r.Err)
		}
	}
	if scenes != 2 {
		t.Errorf("got %d scenes, want 2", scenes)
	}
}

func TestKnownIDsStopEarly(t *testing.T) {
	ts := newTestServer(
		map[int]string{1: fixtureList},
		map[int]string{81236: fixtureDetail81236},
	)
	defer ts.Close()

	s := &Scraper{client: ts.Client(), apiBaseURL: ts.URL}
	ch, err := s.ListScenes(context.Background(), "https://www.sexlikereal.com/scenes", scraper.ListOpts{
		KnownIDs: map[string]bool{"80000": true},
	})
	if err != nil {
		t.Fatal(err)
	}

	results := collect(ch)
	var scenes, stopped int
	for _, r := range results {
		switch r.Kind {
		case scraper.KindScene:
			scenes++
		case scraper.KindStoppedEarly:
			stopped++
		}
	}
	if scenes != 1 {
		t.Errorf("got %d scenes, want 1", scenes)
	}
	if stopped != 1 {
		t.Errorf("got %d stoppedEarly, want 1", stopped)
	}
}

func TestStudioFilter(t *testing.T) {
	var capturedStudios string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v3/scenes" {
			capturedStudios = r.URL.Query().Get("studios")
		}
		_, _ = fmt.Fprint(w, fixtureEmpty)
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client(), apiBaseURL: ts.URL}
	ch, _ := s.ListScenes(context.Background(), "https://www.sexlikereal.com/studios/slr-originals-224", scraper.ListOpts{})
	collect(ch)

	if capturedStudios != "224" {
		t.Errorf("studios param = %q, want 224", capturedStudios)
	}
}

func TestModelFilter(t *testing.T) {
	var capturedModels string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v3/scenes" {
			capturedModels = r.URL.Query().Get("models")
		}
		_, _ = fmt.Fprint(w, fixtureEmpty)
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client(), apiBaseURL: ts.URL}
	ch, _ := s.ListScenes(context.Background(), "https://www.sexlikereal.com/pornstars/molly-little-7099", scraper.ListOpts{})
	collect(ch)

	if capturedModels != "7099" {
		t.Errorf("models param = %q, want 7099", capturedModels)
	}
}
