package mydirtyhobby

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wasylq/FSS/scraper"
)

const testStudioURL = "https://www.mydirtyhobby.com/profil/2517040-Dirty-Tina/videos"

// ---- MatchesURL ----

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url   string
		match bool
	}{
		{"https://www.mydirtyhobby.com/profil/2517040-Dirty-Tina/videos", true},
		{"https://www.mydirtyhobby.com/profil/2517040-Dirty-Tina", true},
		{"http://mydirtyhobby.com/profil/123-some-user", true},
		{"https://www.manyvids.com/Profile/123", false},
		{"https://www.mydirtyhobby.com/videos/top", false},
		{"", false},
	}
	for _, c := range cases {
		got := s.MatchesURL(c.url)
		if got != c.match {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.match)
		}
	}
}

// ---- profileParams ----

func TestProfileParams(t *testing.T) {
	cases := []struct {
		url      string
		wantUID  int
		wantNick string
		wantErr  bool
	}{
		{"https://www.mydirtyhobby.com/profil/2517040-Dirty-Tina/videos", 2517040, "Dirty-Tina", false},
		{"https://www.mydirtyhobby.com/profil/999-some-user", 999, "some-user", false},
		{"https://www.manyvids.com/Profile/123", 0, "", true},
	}
	for _, c := range cases {
		uid, nick, err := profileParams(c.url)
		if (err != nil) != c.wantErr {
			t.Errorf("profileParams(%q) error = %v, wantErr %v", c.url, err, c.wantErr)
			continue
		}
		if uid != c.wantUID {
			t.Errorf("profileParams(%q) uid = %d, want %d", c.url, uid, c.wantUID)
		}
		if nick != c.wantNick {
			t.Errorf("profileParams(%q) nick = %q, want %q", c.url, nick, c.wantNick)
		}
	}
}

// ---- parseDuration ----

func TestParseDuration(t *testing.T) {
	cases := []struct {
		input string
		want  int
	}{
		{"10:47", 647},
		{"1:02:03", 3723},
		{"0:30", 30},
		{"", 0},
	}
	for _, c := range cases {
		got := parseDuration(c.input)
		if got != c.want {
			t.Errorf("parseDuration(%q) = %d, want %d", c.input, got, c.want)
		}
	}
}

// ---- toScene ----

func TestToScene(t *testing.T) {
	item := mdhItem{
		UID:                 2517040,
		UVID:                12649291,
		Nick:                "Dirty-Tina",
		Title:               "Public in the sauna",
		Description:         "Some description",
		Thumbnail:           "https://cdn.example.com/thumb.jpg",
		Price:               "971",
		HasDiscount:         false,
		Duration:            "10:47",
		LatestPictureChange: "2025-12-06T11:12:06+01:00",
	}
	now := parseDate("2026-01-01T00:00:00Z")
	scene := toScene(testStudioURL, "https://www.mydirtyhobby.com", 2517040, "Dirty-Tina", item, now)

	if scene.ID != "12649291" {
		t.Errorf("ID = %q, want %q", scene.ID, "12649291")
	}
	if scene.SiteID != "mydirtyhobby" {
		t.Errorf("SiteID = %q, want %q", scene.SiteID, "mydirtyhobby")
	}
	if scene.Title != "Public in the sauna" {
		t.Errorf("Title = %q, want %q", scene.Title, "Public in the sauna")
	}
	if scene.Duration != 647 {
		t.Errorf("Duration = %d, want 647", scene.Duration)
	}
	if len(scene.PriceHistory) != 1 {
		t.Fatalf("PriceHistory len = %d, want 1", len(scene.PriceHistory))
	}
	if scene.PriceHistory[0].Regular != 9.71 {
		t.Errorf("Regular = %f, want 9.71", scene.PriceHistory[0].Regular)
	}
	wantURL := "https://www.mydirtyhobby.com/profil/2517040-Dirty-Tina/videos/12649291"
	if scene.URL != wantURL {
		t.Errorf("URL = %q, want %q", scene.URL, wantURL)
	}
}

// ---- ListScenes (httptest) ----

func makeResponse(items []mdhItem, total, page, totalPages int) []byte {
	resp := listResponse{
		Items:      items,
		Total:      total,
		Page:       page,
		TotalPages: totalPages,
	}
	b, _ := json.Marshal(resp)
	return b
}

func TestListScenes(t *testing.T) {
	page1 := []mdhItem{
		{UID: 1, UVID: 101, Nick: "TestUser", Title: "Scene A", Price: "500", Duration: "5:00", LatestPictureChange: "2025-01-01T00:00:00Z"},
		{UID: 1, UVID: 102, Nick: "TestUser", Title: "Scene B", Price: "600", Duration: "6:00", LatestPictureChange: "2025-01-02T00:00:00Z"},
	}
	page2 := []mdhItem{
		{UID: 1, UVID: 103, Nick: "TestUser", Title: "Scene C", Price: "700", Duration: "7:00", LatestPictureChange: "2025-01-03T00:00:00Z"},
	}

	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		var req listRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", 400)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if req.Page == 1 {
			_, _ = w.Write(makeResponse(page1, 3, 1, 2))
		} else {
			_, _ = w.Write(makeResponse(page2, 3, 2, 2))
		}
	}))
	defer srv.Close()

	s := &Scraper{
		client:      srv.Client(),
		siteBase:    srv.URL,
		contentBase: srv.URL,
		pageSize:    20,
	}

	studioURL := srv.URL + "/profil/1-TestUser/videos"
	ch, err := s.ListScenes(context.Background(), studioURL, scraper.ListOpts{})
	if err != nil {
		t.Fatalf("ListScenes error: %v", err)
	}

	var scenes []scraper.SceneResult
	for r := range ch {
		scenes = append(scenes, r)
	}

	// First result is Total hint.
	if scenes[0].Total != 3 {
		t.Errorf("Total hint = %d, want 3", scenes[0].Total)
	}
	scenesOnly := scenes[1:]
	if len(scenesOnly) != 3 {
		t.Errorf("got %d scenes, want 3", len(scenesOnly))
	}
	if scenesOnly[0].Scene.ID != "101" {
		t.Errorf("first scene ID = %q, want %q", scenesOnly[0].Scene.ID, "101")
	}
	if callCount != 2 {
		t.Errorf("page requests = %d, want 2", callCount)
	}
}

func TestListScenesKnownIDs(t *testing.T) {
	items := []mdhItem{
		{UID: 1, UVID: 201, Nick: "U", Title: "New", Price: "100", Duration: "1:00", LatestPictureChange: "2025-06-01T00:00:00Z"},
		{UID: 1, UVID: 202, Nick: "U", Title: "Known", Price: "100", Duration: "1:00", LatestPictureChange: "2025-05-01T00:00:00Z"},
		{UID: 1, UVID: 203, Nick: "U", Title: "Also known", Price: "100", Duration: "1:00", LatestPictureChange: "2025-04-01T00:00:00Z"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(makeResponse(items, 3, 1, 1))
	}))
	defer srv.Close()

	s := &Scraper{
		client:      srv.Client(),
		siteBase:    srv.URL,
		contentBase: srv.URL,
		pageSize:    20,
	}

	studioURL := srv.URL + "/profil/1-U/videos"
	ch, err := s.ListScenes(context.Background(), studioURL, scraper.ListOpts{
		KnownIDs: map[string]bool{"202": true},
	})
	if err != nil {
		t.Fatalf("ListScenes error: %v", err)
	}

	var results []scraper.SceneResult
	for r := range ch {
		results = append(results, r)
	}

	// Only Total hint + scene 201 should appear; 202 triggers early stop.
	scenesOnly := make([]scraper.SceneResult, 0)
	sawStoppedEarly := false
	for _, r := range results {
		if r.Kind == scraper.KindStoppedEarly {
			sawStoppedEarly = true
			continue
		}
		if r.Total == 0 {
			scenesOnly = append(scenesOnly, r)
		}
	}
	if len(scenesOnly) != 1 {
		t.Errorf("got %d scenes, want 1 (early stop at known ID)", len(scenesOnly))
	}
	if !sawStoppedEarly {
		t.Error("expected StoppedEarly signal, got none")
	}
	if len(scenesOnly) > 0 && scenesOnly[0].Scene.ID != "201" {
		t.Errorf("scene ID = %q, want %q", scenesOnly[0].Scene.ID, "201")
	}
}
