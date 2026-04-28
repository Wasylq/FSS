package naughtyamerica

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wasylq/FSS/scraper"
)

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url  string
		want bool
	}{
		{"https://www.naughtyamerica.com", true},
		{"https://naughtyamerica.com", true},
		{"https://www.naughtyamerica.com/scene/some-scene-123", true},
		{"https://www.naughtyamericavr.com", true},
		{"https://www.myfriendshotmom.com", true},
		{"https://www.mysistershotfriend.com", true},
		{"https://www.tonightsgirlfriend.com", true},
		{"https://www.thundercock.com", true},
		{"https://thundercock.com", true},
		{"https://www.brazzers.com", false},
		{"https://example.com", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.want {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.want)
		}
	}
}

func TestToScene(t *testing.T) {
	s := scene{
		ID:            100,
		Title:         "Test Scene",
		Length:        1234,
		PublishedDate: "2026-03-15 07:00:00",
		SceneURL:      "https://www.naughtyamerica.com/scene/test-scene-100",
		Synopsis:      "A test scene",
		Tags:          []string{"Tag1", "Tag2"},
		Performers:    map[string][]string{"female": {"Alice"}, "male": {"Bob"}},
		SiteName:      "My Friend's Hot Mom",
		Trailers:      map[string]string{"trailer_720": "https://videos.naughtycdn.com/mfhm/trailers/mfhmalicebobtrailer_720.mp4"},
		RawPromoVideo: json.RawMessage(`{"aff_16mp4":"https://videos.naughtycdn.com/nonsecure/public/promo/mfhm/alicebob/mfhmalicebob_aff_16.mp4"}`),
	}

	sc := toScene("https://www.naughtyamerica.com", s, fixedTime())

	if sc.ID != "100" {
		t.Errorf("ID = %q, want %q", sc.ID, "100")
	}
	if sc.Title != "Test Scene" {
		t.Errorf("Title = %q", sc.Title)
	}
	if sc.Duration != 1234 {
		t.Errorf("Duration = %d, want 1234", sc.Duration)
	}
	if sc.Studio != "My Friend's Hot Mom" {
		t.Errorf("Studio = %q", sc.Studio)
	}
	if sc.Date.Format("2006-01-02") != "2026-03-15" {
		t.Errorf("Date = %v", sc.Date)
	}
	if len(sc.Performers) != 2 {
		t.Errorf("Performers = %v", sc.Performers)
	}
	if sc.Thumbnail == "" {
		t.Error("expected non-empty Thumbnail")
	}
	if sc.Preview == "" {
		t.Error("expected non-empty Preview")
	}
}

func TestToSceneVR(t *testing.T) {
	s := scene{
		ID:            200,
		Title:         "VR Scene",
		PublishedDate: "2026-01-01 07:00:00",
		Degrees:       180,
		Tags:          []string{"Big Tits"},
	}

	sc := toScene("https://www.naughtyamerica.com", s, fixedTime())

	found := false
	for _, tag := range sc.Tags {
		if tag == "VR" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected VR tag in %v", sc.Tags)
	}
}

func TestThumbnailURL(t *testing.T) {
	cases := []struct {
		name     string
		trailers map[string]string
		promos   map[string]string
		want     string
	}{
		{
			name:   "from promo",
			promos: map[string]string{"aff_16mp4": "https://videos.naughtycdn.com/nonsecure/public/promo/ofm/deedom/ofmdeedom_aff_16.mp4"},
			want:   "https://images4.naughtycdn.com/cms/nacmscontent/v1/scenes/ofm/deedom/scene/horizontal/1279x852c.jpg",
		},
		{
			name:     "from trailer non-VR",
			trailers: map[string]string{"trailer_720": "https://videos.naughtycdn.com/nathck/trailers/nathckivybrocktrailer_720.mp4"},
			want:     "https://images4.naughtycdn.com/cms/nacmscontent/v1/scenes/nathck/ivybrock/scene/horizontal/1279x852c.jpg",
		},
		{
			name:     "from trailer VR",
			trailers: map[string]string{"vrdesktophd": "https://videos.naughtycdn.com/nonsecure/psex/trailers/vr/psexaudreyjuan/psexaudreyjuanteaser_vrdesktophd.mp4"},
			want:     "https://images4.naughtycdn.com/cms/nacmscontent/v1/scenes/psex/audreyjuan/scene/horizontal/1279x852c.jpg",
		},
		{
			name: "empty",
			want: "",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := thumbnailURL(c.trailers, c.promos)
			if got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestPreviewURL(t *testing.T) {
	trailers := map[string]string{
		"smartphonevr30": "https://videos.naughtycdn.com/a.mp4",
		"trailer_720":    "https://videos.naughtycdn.com/b.mp4",
	}
	got := previewURL(trailers)
	if got != "https://videos.naughtycdn.com/b.mp4" {
		t.Errorf("got %q, want trailer_720 URL", got)
	}
}

func newTestServer(scenes []scene, total, lastPage int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := apiResponse{
			CurrentPage: 1,
			LastPage:    lastPage,
			Total:       total,
			PerPage:     100,
			Data:        scenes,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
}

func TestListScenes(t *testing.T) {
	scenes := []scene{
		{
			ID: 500, Title: "NA Scene", Length: 1800,
			PublishedDate: "2026-04-01 07:00:00",
			SceneURL:      "https://www.naughtyamerica.com/scene/na-scene-500",
			SiteName:      "My Friend's Hot Mom",
			Performers:    map[string][]string{"female": {"Jane"}},
			Tags:          []string{"MILF"},
		},
	}

	ts := newTestServer(scenes, 1, 1)
	defer ts.Close()

	s := &Scraper{client: ts.Client(), apiURL: ts.URL}

	ch, err := s.ListScenes(context.Background(), "https://www.naughtyamerica.com", scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}

	var titles []string
	for r := range ch {
		if r.Kind == scraper.KindTotal || r.Kind == scraper.KindStoppedEarly {
			continue
		}
		if r.Err != nil {
			t.Errorf("unexpected error: %v", r.Err)
			continue
		}
		titles = append(titles, r.Scene.Title)
	}

	if len(titles) != 1 || titles[0] != "NA Scene" {
		t.Errorf("titles = %v", titles)
	}
}

func TestListScenesKnownIDs(t *testing.T) {
	scenes := []scene{
		{ID: 600, Title: "New", PublishedDate: "2026-04-01 07:00:00",
			Performers: map[string][]string{}},
		{ID: 601, Title: "Known", PublishedDate: "2026-03-01 07:00:00",
			Performers: map[string][]string{}},
	}

	ts := newTestServer(scenes, 2, 1)
	defer ts.Close()

	s := &Scraper{client: ts.Client(), apiURL: ts.URL}

	ch, err := s.ListScenes(context.Background(), "https://www.naughtyamerica.com", scraper.ListOpts{
		KnownIDs: map[string]bool{"601": true},
	})
	if err != nil {
		t.Fatal(err)
	}

	var ids []string
	var stoppedEarly bool
	for r := range ch {
		if r.Total > 0 {
			continue
		}
		if r.Kind == scraper.KindStoppedEarly {
			stoppedEarly = true
			continue
		}
		if r.Err != nil {
			t.Errorf("unexpected error: %v", r.Err)
			continue
		}
		ids = append(ids, r.Scene.ID)
	}

	if !stoppedEarly {
		t.Error("expected StoppedEarly signal")
	}
	if len(ids) != 1 || ids[0] != "600" {
		t.Errorf("got ids %v, want [600]", ids)
	}
}

func fixedTime() (t time.Time) {
	t, _ = time.Parse(time.RFC3339, "2026-04-24T12:00:00Z")
	return
}
