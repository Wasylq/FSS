package ladyfyre

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wasylq/FSS/scraper"
)

func TestScraperInterface(t *testing.T) {
	var _ scraper.StudioScraper = New()
}

func TestMatchesURL(t *testing.T) {
	s := New()
	tests := []struct {
		url  string
		want bool
	}{
		{"https://www.ladyfyre.com/tour/categories/movies.html", true},
		{"https://ladyfyre.com/tour/updates/Some-Scene.html", true},
		{"http://www.ladyfyre.com/tour/models/LadyOliviaFyre.html", true},
		{"https://www.ladyfyre.com/", true},
		{"https://www.example.com/", false},
		{"https://www.houseofyre.com/", false},
	}
	for _, tt := range tests {
		if got := s.MatchesURL(tt.url); got != tt.want {
			t.Errorf("MatchesURL(%q) = %v, want %v", tt.url, got, tt.want)
		}
	}
}

const listingHTML = `<html><body>
<li class="active"><a href="https://www.ladyfyre.com/tour/categories/movies_1_d.html">1</a></li>
<li><a href="https://www.ladyfyre.com/tour/categories/movies_2_d.html">2</a></li>
<li><a href="https://www.ladyfyre.com/tour/categories/movies_3_d.html">3</a></li>
<div class="updateItem">
	<a href="https://www.ladyfyre.com/tour/updates/Ridding-Yourself-of-Masculine-Mannerisms.html">
		<img class="stdimage " src="https://cdn.example.com/tour/content/LadyFyreRidding/1.jpg" />
	</a>
	<div class="updateDetails">
	<div class="buy_button">Buy $8.99</div>
		<h4>
			<a href="https://www.ladyfyre.com/tour/updates/Ridding-Yourself-of-Masculine-Mannerisms.html">
				Ridding Yourself of Masculine Mannerisms
			</a>
		</h4>
		<p>
	<span class="tour_update_models">
			<a href="https://www.ladyfyre.com/tour/models/LadyOliviaFyre.html">Lady Fyre</a>
	</span>
 <span>05/16/2026</span></p>
	</div>
</div>
<div class="updateItem">
	<a href="https://www.ladyfyre.com/tour/updates/Therapist-Part-3.html">
		<img class="stdimage " src="https://cdn.example.com/tour/content/Therapist3/1.jpg" />
	</a>
	<div class="updateDetails">
	<div class="buy_button">Buy $12.99</div>
		<h4>
			<a href="https://www.ladyfyre.com/tour/updates/Therapist-Part-3.html">
				Therapist Part 3
			</a>
		</h4>
		<p>
	<span class="tour_update_models">
			<a href="https://www.ladyfyre.com/tour/models/LadyOliviaFyre.html">Lady Fyre</a>, <a href="https://www.ladyfyre.com/tour/models/AnotherModel.html">Another Model</a>
	</span>
 <span>05/09/2026</span></p>
	</div>
</div>
</body></html>`

func TestParseListingEntries(t *testing.T) {
	entries := parseListingEntries([]byte(listingHTML))
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	e := entries[0]
	if e.slug != "Ridding-Yourself-of-Masculine-Mannerisms" {
		t.Errorf("slug = %q", e.slug)
	}
	if e.title != "Ridding Yourself of Masculine Mannerisms" {
		t.Errorf("title = %q", e.title)
	}
	if len(e.performers) != 1 || e.performers[0] != "Lady Fyre" {
		t.Errorf("performers = %v", e.performers)
	}
	if e.date != "05/16/2026" {
		t.Errorf("date = %q", e.date)
	}
	if e.thumbnail != "https://cdn.example.com/tour/content/LadyFyreRidding/1.jpg" {
		t.Errorf("thumbnail = %q", e.thumbnail)
	}
	if e.price != 8.99 {
		t.Errorf("price = %f", e.price)
	}

	e2 := entries[1]
	if e2.slug != "Therapist-Part-3" {
		t.Errorf("entry 2 slug = %q", e2.slug)
	}
	if len(e2.performers) != 2 {
		t.Errorf("entry 2 performers = %v", e2.performers)
	}
	if e2.price != 12.99 {
		t.Errorf("entry 2 price = %f", e2.price)
	}
}

func TestParseMaxPage(t *testing.T) {
	if got := parseMaxPage([]byte(listingHTML)); got != 3 {
		t.Errorf("parseMaxPage = %d, want 3", got)
	}
}

func TestHasNextPage(t *testing.T) {
	if !hasNextPage([]byte(listingHTML), 1) {
		t.Error("expected hasNextPage(1) = true")
	}
	if hasNextPage([]byte(listingHTML), 3) {
		t.Error("expected hasNextPage(3) = false")
	}
}

const detailHTML = `<html><head>
<meta property="og:description" content="A great scene description here"/>
</head><body>
<span class="update_tags">
Tags:
  <a href="https://www.ladyfyre.com/tour/categories/femdom.html">Femdom</a>
  <a href="https://www.ladyfyre.com/tour/categories/SissyTraining.html">Sissy Training</a>
</span>
</body></html>`

func TestParseDetail(t *testing.T) {
	entry := listEntry{
		slug:       "Test-Scene",
		url:        "https://www.ladyfyre.com/tour/updates/Test-Scene.html",
		title:      "Test Scene",
		performers: []string{"Lady Fyre"},
		date:       "05/16/2026",
		thumbnail:  "https://cdn.example.com/thumb.jpg",
		price:      8.99,
	}

	scene := parseDetail([]byte(detailHTML), entry)

	if scene.Title != "Test Scene" {
		t.Errorf("title = %q", scene.Title)
	}
	if scene.Description != "A great scene description here" {
		t.Errorf("description = %q", scene.Description)
	}
	if len(scene.Tags) != 2 || scene.Tags[0] != "Femdom" {
		t.Errorf("tags = %v", scene.Tags)
	}
	if scene.Date.Format("2006-01-02") != "2026-05-16" {
		t.Errorf("date = %v", scene.Date)
	}
	if scene.PriceHistory == nil || scene.PriceHistory[0].Regular != 8.99 {
		t.Errorf("price = %v", scene.PriceHistory)
	}
}

func TestListScenes(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		switch {
		case strings.Contains(r.URL.Path, "movies_1_d") || strings.Contains(r.URL.Path, "movies.html"):
			_, _ = fmt.Fprint(w, strings.ReplaceAll(listingHTML, "https://www.ladyfyre.com", ""))
		case strings.Contains(r.URL.Path, "movies_2_d"):
			_, _ = fmt.Fprint(w, `<html><body>
<div class="updateItem">
	<a href="/tour/updates/Last-Scene.html">
		<img class="stdimage " src="https://cdn.example.com/tour/content/Last/1.jpg" />
	</a>
	<div class="updateDetails">
		<h4><a href="/tour/updates/Last-Scene.html">Last Scene</a></h4>
		<p><span class="tour_update_models"><a href="/tour/models/Model.html">Model</a></span>
 <span>01/01/2020</span></p>
	</div>
</div>
</body></html>`)
		case strings.Contains(r.URL.Path, "movies_3_d"):
			_, _ = fmt.Fprint(w, "<html></html>")
		case strings.Contains(r.URL.Path, "/updates/"):
			_, _ = fmt.Fprint(w, detailHTML)
		default:
			_, _ = fmt.Fprint(w, "<html></html>")
		}
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client(), baseOverride: ts.URL}

	ch, err := s.ListScenes(context.Background(), ts.URL+"/tour/", scraper.ListOpts{Workers: 1})
	if err != nil {
		t.Fatal(err)
	}

	var scenes int
	for r := range ch {
		switch r.Kind {
		case scraper.KindScene:
			scenes++
		case scraper.KindError:
			t.Errorf("unexpected error: %v", r.Err)
		}
	}
	if scenes != 3 {
		t.Errorf("got %d scenes, want 3", scenes)
	}
}

func TestKnownIDsStopsEarly(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		switch {
		case strings.Contains(r.URL.Path, "movies_"):
			_, _ = fmt.Fprint(w, strings.ReplaceAll(listingHTML, "https://www.ladyfyre.com", ""))
		case strings.Contains(r.URL.Path, "/updates/"):
			_, _ = fmt.Fprint(w, detailHTML)
		default:
			_, _ = fmt.Fprint(w, "<html></html>")
		}
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client(), baseOverride: ts.URL}

	ch, err := s.ListScenes(context.Background(), ts.URL+"/tour/", scraper.ListOpts{
		Workers:  1,
		KnownIDs: map[string]bool{"Therapist-Part-3": true},
	})
	if err != nil {
		t.Fatal(err)
	}

	var scenes int
	var stoppedEarly bool
	for r := range ch {
		switch r.Kind {
		case scraper.KindScene:
			scenes++
		case scraper.KindStoppedEarly:
			stoppedEarly = true
		case scraper.KindError:
			t.Errorf("unexpected error: %v", r.Err)
		}
	}
	if scenes != 1 {
		t.Errorf("got %d scenes, want 1", scenes)
	}
	if !stoppedEarly {
		t.Error("expected StoppedEarly signal")
	}
}
