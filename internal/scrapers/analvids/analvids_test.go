package analvids

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wasylq/FSS/parseutil"
	"github.com/Wasylq/FSS/scraper"
)

const fixtureListing = `<html><body>
<div class="card-scene" data-content="100"><div class="card-scene__view"><div class="card-scene__labels"><div class="label label--y mb-5">4k</div></div><div class="card-scene__time"><div class="label label--time">1 h 5 min</div><div class="label label--time">2025-06-15</div></div><a href="/watch/100/first_scene_slug" data-preview="https://cdn.example.com/preview.mp4"><img data-src="https://cdn.example.com/thumb1.jpg" alt="First Scene Title"></a></div><div class="card-scene__text"><a href="/watch/100/first_scene_slug" title="First Scene Title">First Scene Title</a></div></div>
<div class="card-scene" data-content="99"><div class="card-scene__view"><div class="card-scene__time"><div class="label label--time">35 min</div><div class="label label--time">2025-06-14</div></div><a href="/watch/99/second_scene" data-preview=""><img data-src="https://cdn.example.com/thumb2.jpg" alt="Second Scene"></a></div><div class="card-scene__text"><a href="/watch/99/second_scene" title="Second Scene">Second Scene</a></div></div>
</body></html>`

const fixtureDetail = `<html><body>
<h1 class="watch__title"><a href="https://www.analvids.com/model/1/alice" class="text-primary">Alice</a> &amp; <a href="https://www.analvids.com/model/2/bob" class="text-primary">Bob</a>, First Scene Title<span class="watch__featuring_models d-block mt-10 mt-lg-5">featuring&nbsp;<a href="https://www.analvids.com/model/3/carol" class="text-primary">Carol</a></span></h1>
<i class="bi bi-calendar3"></i> 2025-06-15
<i class="bi bi-clock"></i> 65:30
<span class="fw-bold">Studio:</span> <a href="/studios/test-studio">Test Studio</a>
<div class="genres-list">
<a href="/genre/anal">anal</a>
<a href="/genre/dp">dp</a>
</div>
</body></html>`

const fixtureEmpty = `<html><body></body></html>`

func TestParseListing(t *testing.T) {
	entries := parseListing(fixtureListing)
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}

	e := entries[0]
	if e.id != "100" {
		t.Errorf("id = %q, want 100", e.id)
	}
	if e.title != "First Scene Title" {
		t.Errorf("title = %q", e.title)
	}
	if e.url != "/watch/100/first_scene_slug" {
		t.Errorf("url = %q", e.url)
	}
	if e.thumb != "https://cdn.example.com/thumb1.jpg" {
		t.Errorf("thumb = %q", e.thumb)
	}
	if e.dur != 3900 {
		t.Errorf("dur = %d, want 3900 (1h5min)", e.dur)
	}
	if e.date != "2025-06-15" {
		t.Errorf("date = %q, want 2025-06-15", e.date)
	}

	e2 := entries[1]
	if e2.dur != 2100 {
		t.Errorf("dur = %d, want 2100 (35 min)", e2.dur)
	}
}

func TestParseDetail(t *testing.T) {
	d := parseDetail(fixtureDetail)
	if len(d.performers) != 3 || d.performers[0] != "Alice" || d.performers[1] != "Bob" || d.performers[2] != "Carol" {
		t.Errorf("performers = %v, want [Alice Bob Carol]", d.performers)
	}
	if d.date != "2025-06-15" {
		t.Errorf("date = %q, want 2025-06-15", d.date)
	}
	if d.duration != 3930 {
		t.Errorf("duration = %d, want 3930 (65:30)", d.duration)
	}
	if d.studio != "Test Studio" {
		t.Errorf("studio = %q", d.studio)
	}
	if len(d.tags) != 2 || d.tags[0] != "anal" || d.tags[1] != "dp" {
		t.Errorf("tags = %v", d.tags)
	}
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		in   string
		want int
	}{
		{"55 min", 3300},
		{"1 h 8 min", 4080},
		{"2 h 30 min", 9000},
		{"1 h", 3600},
	}
	for _, tt := range tests {
		if got := parseDuration(tt.in); got != tt.want {
			t.Errorf("parseDuration(%q) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestParseDurationColon(t *testing.T) {
	tests := []struct {
		in   string
		want int
	}{
		{"65:30", 3930},
		{"1:05:30", 3930},
		{"30:00", 1800},
	}
	for _, tt := range tests {
		if got := parseutil.ParseDurationColon(tt.in); got != tt.want {
			t.Errorf("parseutil.ParseDurationColon(%q) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func newTestServer(listingPages map[int]string, details map[string]string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/new-videos" || r.URL.Path == "/new-videos/":
			if body, ok := listingPages[1]; ok {
				_, _ = fmt.Fprint(w, body)
				return
			}
		case len(r.URL.Path) > len("/new-videos/"):
			page := r.URL.Path[len("/new-videos/"):]
			var pn int
			if _, err := fmt.Sscanf(page, "%d", &pn); err == nil {
				if body, ok := listingPages[pn]; ok {
					_, _ = fmt.Fprint(w, body)
					return
				}
			}
		}
		if body, ok := details[r.URL.Path]; ok {
			_, _ = fmt.Fprint(w, body)
			return
		}
		_, _ = fmt.Fprint(w, fixtureEmpty)
	}))
}

func collect(ch <-chan scraper.SceneResult) []scraper.SceneResult {
	var results []scraper.SceneResult
	for r := range ch {
		results = append(results, r)
	}
	return results
}

func TestListScenes(t *testing.T) {
	ts := newTestServer(
		map[int]string{1: fixtureListing},
		map[string]string{
			"/watch/100/first_scene_slug": fixtureDetail,
			"/watch/99/second_scene":      fixtureDetail,
		},
	)
	defer ts.Close()

	s := &Scraper{client: ts.Client(), base: ts.URL}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/new-videos", scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}

	results := collect(ch)
	var scenes int
	for _, r := range results {
		if r.Kind == scraper.KindScene {
			scenes++
			if r.Scene.ID == "100" {
				if len(r.Scene.Performers) != 3 {
					t.Errorf("performers = %v", r.Scene.Performers)
				}
				if r.Scene.Studio != "Test Studio" {
					t.Errorf("studio = %q", r.Scene.Studio)
				}
				if r.Scene.Duration != 3930 {
					t.Errorf("duration = %d, want 3930 (detail page value)", r.Scene.Duration)
				}
			}
		}
	}
	if scenes != 2 {
		t.Errorf("got %d scenes, want 2", scenes)
	}
}

func TestKnownIDsStopEarly(t *testing.T) {
	ts := newTestServer(
		map[int]string{1: fixtureListing},
		map[string]string{
			"/watch/100/first_scene_slug": fixtureDetail,
		},
	)
	defer ts.Close()

	s := &Scraper{client: ts.Client(), base: ts.URL}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/new-videos", scraper.ListOpts{
		KnownIDs: map[string]bool{"99": true},
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

func TestResolveMode(t *testing.T) {
	s := New()

	tests := []struct {
		url        string
		wantPath   string
		wantSorted bool
	}{
		{"https://www.analvids.com/new-videos", s.base + "/new-videos", true},
		{"https://www.analvids.com", s.base + "/new-videos", true},
		{"https://www.analvids.com/studios/giorgio-grandi", s.base + "/studios/giorgio-grandi", false},
		{"https://www.analvids.com/model/3329/anna_de_ville", s.base + "/model/3329/anna_de_ville", false},
	}
	for _, tt := range tests {
		path, sorted := s.resolveMode(tt.url)
		if path != tt.wantPath {
			t.Errorf("resolveMode(%q) path = %q, want %q", tt.url, path, tt.wantPath)
		}
		if sorted != tt.wantSorted {
			t.Errorf("resolveMode(%q) sorted = %v, want %v", tt.url, sorted, tt.wantSorted)
		}
	}
}

func TestMatchesURL(t *testing.T) {
	s := New()
	tests := []struct {
		url  string
		want bool
	}{
		{"https://www.analvids.com/new-videos", true},
		{"https://analvids.com/studios/test", true},
		{"https://www.analvids.com/model/1/test", true},
		{"https://example.com", false},
	}
	for _, tt := range tests {
		if got := s.MatchesURL(tt.url); got != tt.want {
			t.Errorf("MatchesURL(%q) = %v, want %v", tt.url, got, tt.want)
		}
	}
}
