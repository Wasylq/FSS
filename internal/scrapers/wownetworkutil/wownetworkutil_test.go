package wownetworkutil

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/parseutil"
	"github.com/Wasylq/FSS/scraper"
)

var _ scraper.StudioScraper = (*Scraper)(nil)

func TestParseDuration(t *testing.T) {
	tests := []struct {
		in   string
		want int
	}{
		{"PT27M32S", 27*60 + 32},
		{"PT50M53S", 50*60 + 53},
		{"PT1H5M30S", 3600 + 5*60 + 30},
		{"PT59M43S", 59*60 + 43},
		{"PT0M0S", 0},
		{"", 0},
		{"null", 0},
	}
	for _, tt := range tests {
		got := parseutil.ParseDurationISO(tt.in)
		if got != tt.want {
			t.Errorf("ParseDurationISO(%q) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestExtractSlug(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"https://wowgirls.com/tour/trailer/trailers/who-let-the-dogs-out", "who-let-the-dogs-out"},
		{"https://wowgirls.com/tour/trailer/whatsnew/awakening-lust", "awakening-lust"},
		{"https://ultrafilms.com/tour/trailer/trailers/new-wave", "new-wave"},
	}
	for _, tt := range tests {
		got := extractSlug(tt.in)
		if got != tt.want {
			t.Errorf("extractSlug(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestParseDescriptionPerformers(t *testing.T) {
	tests := []struct {
		name string
		desc string
		want []string
	}{
		{
			"with performers",
			`New Wave featuring <a class="model-tag-label" href="/join" data-property="model" data-value="67">Anjelica</a><span class="model-tag-spacer"></span><a class="model-tag-label" href="/join" data-property="model" data-value="23">Sofilie</a> — watch the trailer`,
			[]string{"Anjelica", "Sofilie"},
		},
		{
			"no performers",
			`Awakening Lust — watch the trailer on WowGirls`,
			nil,
		},
		{
			"single performer",
			`Solo featuring <a class="model-tag-label" href="/join" data-property="model" data-value="1">Alice</a> — watch`,
			[]string{"Alice"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseDescriptionPerformers(tt.desc)
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("performers[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestParseListingPerformers(t *testing.T) {
	html := []byte(`
<div class="trailer clickable mt-0" data-href="/tour/trailer/whatsnew/awakening-lust">
	<div class="trailer-header">
		<div class="title">Awakening Lust</div>
		<div class="models-list meta-list-outer">
			<div class="models meta-list">
				<div class="model meta-list-item">Leona Mia</div>
				<div class="model meta-list-item">Kitty Wild</div>
			</div>
		</div>
	</div>
</div>
</div>
</div>
<div class="trailer clickable mt-0" data-href="/tour/trailer/whatsnew/sultry-eyes">
	<div class="trailer-header">
		<div class="title">Sultry Eyes</div>
		<div class="models-list meta-list-outer">
			<div class="models meta-list">
				<div class="model meta-list-item">Alissa Foxy</div>
			</div>
		</div>
	</div>
</div>
</div>
</div>`)

	result := make(map[string][]string)
	parseListingPerformers(html, result)

	if perfs, ok := result["awakening-lust"]; !ok {
		t.Error("missing awakening-lust")
	} else if len(perfs) != 2 || perfs[0] != "Leona Mia" || perfs[1] != "Kitty Wild" {
		t.Errorf("awakening-lust performers = %v", perfs)
	}

	if perfs, ok := result["sultry-eyes"]; !ok {
		t.Error("missing sultry-eyes")
	} else if len(perfs) != 1 || perfs[0] != "Alissa Foxy" {
		t.Errorf("sultry-eyes performers = %v", perfs)
	}
}

func TestMatchesURL(t *testing.T) {
	cfg := SiteConfig{SiteID: "wowgirls", Domain: "wowgirls.com", StudioName: "WowGirls", AltDomains: []string{"wowporn.com"}}
	s := New(cfg)

	tests := []struct {
		url  string
		want bool
	}{
		{"https://wowgirls.com", true},
		{"https://www.wowgirls.com", true},
		{"https://wowgirls.com/tour/whats-new", true},
		{"https://wowporn.com", true},
		{"https://www.wowporn.com/something", true},
		{"https://other.com", false},
	}
	for _, tt := range tests {
		if got := s.MatchesURL(tt.url); got != tt.want {
			t.Errorf("MatchesURL(%q) = %v, want %v", tt.url, got, tt.want)
		}
	}
}

func TestPatterns(t *testing.T) {
	cfg := SiteConfig{SiteID: "wowgirls", Domain: "wowgirls.com", StudioName: "WowGirls"}
	s := New(cfg)
	patterns := s.Patterns()
	if len(patterns) != 3 {
		t.Fatalf("got %d patterns, want 3", len(patterns))
	}
}

const testSitemap = `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
    <url><loc>https://example.com/</loc></url>
    <url><loc>https://example.com/tour/whats-new</loc></url>
    <url><loc>https://example.com/tour/trailer/trailers/scene-one</loc></url>
    <url><loc>https://example.com/tour/trailer/trailers/scene-two</loc></url>
    <url><loc>https://example.com/tour/trailer/whatsnew/scene-three</loc></url>
</urlset>`

func testJSONLD(title, date, duration string) string {
	return fmt.Sprintf(`{"@context":"https://schema.org","@type":"VideoObject","name":"%s","description":"%s — watch the trailer","thumbnailUrl":"https://cdn.example.com/thumb.jpg","duration":"%s","isFamilyFriendly":false,"uploadDate":"%s"}`,
		title, title, duration, date)
}

func testJSONLDWithPerformers(title, date, duration string, performers []string) string {
	var parts []string
	for _, p := range performers {
		parts = append(parts, fmt.Sprintf(`<a class=\"model-tag-label\" href=\"/join\" data-property=\"model\" data-value=\"1\">%s</a>`, p))
	}
	desc := fmt.Sprintf(`%s featuring %s — watch the trailer`, title, strings.Join(parts, `<span class=\"model-tag-spacer\"></span>`))
	return fmt.Sprintf(`{"@context":"https://schema.org","@type":"VideoObject","name":"%s","description":"%s","thumbnailUrl":"https://cdn.example.com/thumb.jpg","duration":"%s","isFamilyFriendly":false,"uploadDate":"%s"}`,
		title, desc, duration, date)
}

func detailPage(jsonLD string) string {
	return fmt.Sprintf(`<html><head><script type="application/ld+json">%s</script></head><body></body></html>`, jsonLD)
}

func TestEndToEnd(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/sitemap.xml":
			w.Header().Set("Content-Type", "application/xml")
			_, _ = fmt.Fprint(w, strings.ReplaceAll(testSitemap, "https://example.com", "http://"+r.Host))
		case "/tour/whats-new":
			_, _ = fmt.Fprint(w, `<html><body>
<div class="trailer clickable mt-0" data-href="/tour/trailer/whatsnew/scene-three">
	<div class="trailer-header">
		<div class="models-list meta-list-outer">
			<div class="models meta-list">
				<div class="model meta-list-item">Alice</div>
			</div>
		</div>
	</div>
</div>
</div>
</div>
</body></html>`)
		case "/tour/trailer/trailers/scene-one":
			_, _ = fmt.Fprint(w, detailPage(testJSONLD("Scene One", "2020-01-15", "PT30M0S")))
		case "/tour/trailer/trailers/scene-two":
			_, _ = fmt.Fprint(w, detailPage(testJSONLDWithPerformers("Scene Two", "2021-06-01", "PT45M10S", []string{"Bob", "Carol"})))
		case "/tour/trailer/whatsnew/scene-three":
			_, _ = fmt.Fprint(w, detailPage(testJSONLD("Scene Three", "2025-03-01", "PT59M43S")))
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	domain := strings.TrimPrefix(ts.URL, "http://")
	cfg := SiteConfig{SiteID: "test", Domain: domain, StudioName: "Test Studio"}
	s := New(cfg)

	ctx := context.Background()
	ch, err := s.ListScenes(ctx, ts.URL, scraper.ListOpts{Delay: time.Millisecond})
	if err != nil {
		t.Fatalf("ListScenes: %v", err)
	}

	var scenes []models.Scene
	for res := range ch {
		switch res.Kind {
		case scraper.KindScene:
			scenes = append(scenes, res.Scene)
		case scraper.KindError:
			t.Fatalf("unexpected error: %v", res.Err)
		}
	}

	if len(scenes) != 3 {
		t.Fatalf("got %d scenes, want 3", len(scenes))
	}

	s1 := scenes[0]
	if s1.ID != "scene-one" {
		t.Errorf("scene[0].ID = %q", s1.ID)
	}
	if s1.Title != "Scene One" {
		t.Errorf("scene[0].Title = %q", s1.Title)
	}
	if s1.Duration != 30*60 {
		t.Errorf("scene[0].Duration = %d, want %d", s1.Duration, 30*60)
	}
	if s1.Date.Format("2006-01-02") != "2020-01-15" {
		t.Errorf("scene[0].Date = %v", s1.Date)
	}

	s2 := scenes[1]
	if len(s2.Performers) != 2 || s2.Performers[0] != "Bob" || s2.Performers[1] != "Carol" {
		t.Errorf("scene[1].Performers = %v (from JSON-LD description)", s2.Performers)
	}

	s3 := scenes[2]
	if len(s3.Performers) != 1 || s3.Performers[0] != "Alice" {
		t.Errorf("scene[2].Performers = %v (from listing page)", s3.Performers)
	}
	if s3.Duration != 59*60+43 {
		t.Errorf("scene[2].Duration = %d, want %d", s3.Duration, 59*60+43)
	}
}

func TestKnownIDsEarlyStop(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/sitemap.xml":
			w.Header().Set("Content-Type", "application/xml")
			_, _ = fmt.Fprint(w, strings.ReplaceAll(testSitemap, "https://example.com", "http://"+r.Host))
		case "/tour/whats-new":
			_, _ = fmt.Fprint(w, `<html><body></body></html>`)
		case "/tour/trailer/trailers/scene-one":
			_, _ = fmt.Fprint(w, detailPage(testJSONLD("Scene One", "2020-01-15", "PT30M0S")))
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	domain := strings.TrimPrefix(ts.URL, "http://")
	cfg := SiteConfig{SiteID: "test", Domain: domain, StudioName: "Test"}
	s := New(cfg)

	ctx := context.Background()
	ch, err := s.ListScenes(ctx, ts.URL, scraper.ListOpts{
		Delay:    time.Millisecond,
		KnownIDs: map[string]bool{"scene-two": true},
	})
	if err != nil {
		t.Fatalf("ListScenes: %v", err)
	}

	sceneCount := 0
	stoppedEarly := false
	for res := range ch {
		switch res.Kind {
		case scraper.KindScene:
			sceneCount++
		case scraper.KindStoppedEarly:
			stoppedEarly = true
		case scraper.KindError:
			t.Fatalf("unexpected error: %v", res.Err)
		}
	}
	if sceneCount != 1 {
		t.Errorf("got %d scenes, want 1", sceneCount)
	}
	if !stoppedEarly {
		t.Error("expected StoppedEarly")
	}
}

func TestSceneValidation(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/sitemap.xml":
			sm := `<?xml version="1.0" encoding="UTF-8"?><urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"><url><loc>` + "http://" + r.Host + `/tour/trailer/trailers/test-scene</loc></url></urlset>`
			w.Header().Set("Content-Type", "application/xml")
			_, _ = fmt.Fprint(w, sm)
		case "/tour/whats-new":
			_, _ = fmt.Fprint(w, `<html><body></body></html>`)
		case "/tour/trailer/trailers/test-scene":
			_, _ = fmt.Fprint(w, detailPage(testJSONLDWithPerformers("Test Scene", "2025-01-01", "PT30M0S", []string{"Performer"})))
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	domain := strings.TrimPrefix(ts.URL, "http://")
	cfg := SiteConfig{SiteID: "test", Domain: domain, StudioName: "Test"}
	s := New(cfg)

	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{Delay: time.Millisecond})
	if err != nil {
		t.Fatalf("ListScenes: %v", err)
	}

	for res := range ch {
		switch res.Kind {
		case scraper.KindScene:
			testutil.ValidateScene(t, res.Scene)
		case scraper.KindError:
			t.Fatalf("unexpected error: %v", res.Err)
		}
	}
}
