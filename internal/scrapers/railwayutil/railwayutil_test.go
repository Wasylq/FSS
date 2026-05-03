package railwayutil

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
	"github.com/Wasylq/FSS/scraper"
)

var testCfg = SiteConfig{
	ID:       "testsite",
	SiteCode: "TST",
	Studio:   "Test Studio",
	SiteBase: "https://testsite.com",
	Patterns: []string{"testsite.com/#/models"},
	MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?testsite\.com`),
}

func TestExtractPerformer(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"Abby Adams 1", "Abby Adams"},
		{"Allie Nicole all", "Allie Nicole"},
		{"Amber Jayne 12", "Amber Jayne"},
		{"Solo Name", "Solo Name"},
		{"Name", "Name"},
	}
	for _, tt := range tests {
		if got := ExtractPerformer(tt.name); got != tt.want {
			t.Errorf("ExtractPerformer(%q) = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		s    string
		want int
	}{
		{"00:10:13", 613},
		{"10:13", 613},
		{"05:00", 300},
		{"", 0},
	}
	for _, tt := range tests {
		if got := ParseDuration(tt.s); got != tt.want {
			t.Errorf("ParseDuration(%q) = %d, want %d", tt.s, got, tt.want)
		}
	}
}

func TestParseFilter(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"https://testsite.com/#/models", ""},
		{"https://testsite.com/#/models/Abby%20Adams", "abby adams"},
		{"https://testsite.com/#/models/Allie Nicole", "allie nicole"},
		{"https://testsite.com/", ""},
	}
	for _, tt := range tests {
		if got := ParseFilter(tt.url); got != tt.want {
			t.Errorf("ParseFilter(%q) = %q, want %q", tt.url, got, tt.want)
		}
	}
}

func newTestServer(videos []APIVideo) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(videos)
	}))
}

func TestRun(t *testing.T) {
	videos := []APIVideo{
		{ID: "aaa", Name: "Abby Adams 1", Site: "TST", Duration: "10:13"},
		{ID: "bbb", Name: "Abby Adams 2", Site: "TST", Duration: "08:30"},
		{ID: "ccc", Name: "Allie Nicole 1", Site: "TST", Duration: "05:00"},
	}

	srv := newTestServer(videos)
	defer srv.Close()

	orig := APIBase
	t.Cleanup(func() { APIBase = orig })
	APIBase = srv.URL

	s := New(testCfg)
	ch, err := s.ListScenes(context.Background(), "https://testsite.com/#/models", scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}

	scenes := testutil.CollectScenes(t, ch)
	if len(scenes) != 3 {
		t.Fatalf("got %d scenes, want 3", len(scenes))
	}

	if scenes[0].Title != "Abby Adams 1" {
		t.Errorf("scenes[0].Title = %q, want %q", scenes[0].Title, "Abby Adams 1")
	}
	if scenes[0].Duration != 613 {
		t.Errorf("scenes[0].Duration = %d, want 613", scenes[0].Duration)
	}
	if len(scenes[0].Performers) != 1 || scenes[0].Performers[0] != "Abby Adams" {
		t.Errorf("scenes[0].Performers = %v, want [Abby Adams]", scenes[0].Performers)
	}
	if scenes[0].SiteID != "testsite" {
		t.Errorf("scenes[0].SiteID = %q, want %q", scenes[0].SiteID, "testsite")
	}
}

func TestRunWithFilter(t *testing.T) {
	videos := []APIVideo{
		{ID: "aaa", Name: "Abby Adams 1", Site: "TST", Duration: "10:13"},
		{ID: "bbb", Name: "Abby Adams 2", Site: "TST", Duration: "08:30"},
		{ID: "ccc", Name: "Allie Nicole 1", Site: "TST", Duration: "05:00"},
	}

	srv := newTestServer(videos)
	defer srv.Close()

	orig := APIBase
	t.Cleanup(func() { APIBase = orig })
	APIBase = srv.URL

	s := New(testCfg)
	ch, err := s.ListScenes(context.Background(), "https://testsite.com/#/models/Abby%20Adams", scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}

	scenes := testutil.CollectScenes(t, ch)
	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2", len(scenes))
	}
}

func TestKnownIDs(t *testing.T) {
	videos := []APIVideo{
		{ID: "aaa", Name: "Abby Adams 1", Site: "TST", Duration: "10:13"},
		{ID: "bbb", Name: "Abby Adams 2", Site: "TST", Duration: "08:30"},
		{ID: "ccc", Name: "Allie Nicole 1", Site: "TST", Duration: "05:00"},
	}

	srv := newTestServer(videos)
	defer srv.Close()

	orig := APIBase
	t.Cleanup(func() { APIBase = orig })
	APIBase = srv.URL

	s := New(testCfg)
	ch, err := s.ListScenes(context.Background(), "https://testsite.com/#/models", scraper.ListOpts{
		KnownIDs: map[string]bool{"bbb": true},
	})
	if err != nil {
		t.Fatal(err)
	}

	scenes, stopped := testutil.CollectScenesWithStop(t, ch)
	if len(scenes) != 1 {
		t.Fatalf("got %d scenes, want 1", len(scenes))
	}
	if !stopped {
		t.Error("expected StoppedEarly")
	}
}
