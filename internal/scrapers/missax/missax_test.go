package missax

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wasylq/FSS/scraper"
)

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url  string
		want bool
	}{
		{"https://www.missax.com", true},
		{"https://missax.com/tour/categories/movies_1_d.html", true},
		{"https://www.missax.com/tour/trailers/Some-Scene.html", true},
		{"https://www.manyvids.com/Profile/123/foo", false},
		{"https://example.com", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.want {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.want)
		}
	}
}

const fixtureListPage = `<!DOCTYPE html><html><body>
<div class="photo-thumb video-thumb">
	<div class="photo-thumb_body">
		<a href="%s/tour/trailers/Test-Scene.html" title="Test Scene">
			<img class="mainThumb thumbs stdimage" src0_1x="https://cdn.missax.com/tour/content/contentthumbs/10/01/1001-1x.jpg?expires=99&token=abc" />
		</a>
	</div>
	<div class="thumb-descr">
		<p class="thumb-title">Test Scene</p>
		<p class="model-name">
			<a href="/tour/models/ActorA.html">Actor A</a>, <a href="/tour/models/ActorB.html">Actor B</a>
		</p>
	</div>
</div><!-- end photo-thumb -->
<div class="photo-thumb video-thumb">
	<div class="photo-thumb_body">
		<a href="%s/tour/trailers/Another-Scene.html" title="Another Scene">
			<img class="mainThumb thumbs stdimage" src0_1x="https://cdn.missax.com/tour/content/contentthumbs/10/02/1002-1x.jpg?expires=99&token=def" />
		</a>
	</div>
	<div class="thumb-descr">
		<p class="thumb-title">Another Scene</p>
		<p class="model-name">
			<a href="/tour/models/ActorC.html">Actor C</a>
		</p>
	</div>
</div><!-- end photo-thumb -->
</body></html>`

const fixtureDetailPage = `<!DOCTYPE html><html><head>
<TITLE>Test Scene</TITLE>
<meta property="og:image" content="https://www.missax.com/tour/content/contentthumbs/1001.jpg" />
</head><body>
<div class="wrap-block">
	<video src="/trailers/missa_testscene_12345.mp4" poster="/img.jpg" width="1190" height="675"></video>
</div>
<p class="dvd-scenes__data">
	Runtime: 25:30 | Added: 03/15/2026 | Featuring: <a href="/tour/models/ActorA.html">Actor A</a>, <a href="/tour/models/ActorB.html">Actor B</a>
</p>
<p class="dvd-scenes__data">
	Categories: <a href="/tour/categories/Blowjob_1_d.html">Blowjob</a>, <a href="/tour/categories/Taboo_1_d.html">Taboo</a>, <a href="/tour/categories/Natural_1_d.html">Natural</a>
</p>
<p class="dvd-scenes__title">
	Video Description:
</p>
<p class="text text--marg">
	First paragraph of the description.
<P>
Second paragraph with more details.
</p>
</body></html>`

func TestFetchPage(t *testing.T) {
	var ts *httptest.Server
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, fixtureListPage, ts.URL, ts.URL)
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client(), siteBase: ts.URL}
	entries, err := s.fetchPage(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}

	e := entries[0]
	if e.id != "1001" {
		t.Errorf("ID = %q, want 1001", e.id)
	}
	if e.title != "Test Scene" {
		t.Errorf("Title = %q", e.title)
	}
	if !strings.Contains(e.url, "Test-Scene.html") {
		t.Errorf("URL = %q", e.url)
	}
	if !strings.Contains(e.thumbnail, "1001-1x.jpg") {
		t.Errorf("Thumbnail = %q", e.thumbnail)
	}
	if len(e.performers) != 2 || e.performers[0] != "Actor A" || e.performers[1] != "Actor B" {
		t.Errorf("Performers = %v", e.performers)
	}
}

func TestFetchDetail(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(fixtureDetailPage))
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client(), siteBase: ts.URL}
	entry := listEntry{
		id:         "1001",
		title:      "Test Scene",
		url:        ts.URL + "/tour/trailers/Test-Scene.html",
		thumbnail:  "https://cdn.missax.com/tour/content/contentthumbs/10/01/1001-1x.jpg",
		performers: []string{"Actor A", "Actor B"},
	}

	scene, err := s.fetchDetail(context.Background(), ts.URL, entry)
	if err != nil {
		t.Fatal(err)
	}

	if scene.ID != "1001" {
		t.Errorf("ID = %q", scene.ID)
	}
	if scene.Title != "Test Scene" {
		t.Errorf("Title = %q", scene.Title)
	}
	if scene.Duration != 25*60+30 {
		t.Errorf("Duration = %d, want %d", scene.Duration, 25*60+30)
	}
	if scene.Date.Year() != 2026 || scene.Date.Month() != 3 || scene.Date.Day() != 15 {
		t.Errorf("Date = %v", scene.Date)
	}
	if len(scene.Tags) != 3 || scene.Tags[0] != "Blowjob" {
		t.Errorf("Tags = %v", scene.Tags)
	}
	wantDesc := "First paragraph of the description.\n\nSecond paragraph with more details."
	if scene.Description != wantDesc {
		t.Errorf("Description = %q, want %q", scene.Description, wantDesc)
	}
	if scene.Thumbnail != "https://www.missax.com/tour/content/contentthumbs/1001.jpg" {
		t.Errorf("Thumbnail = %q", scene.Thumbnail)
	}
	if !strings.Contains(scene.Preview, "missa_testscene") {
		t.Errorf("Preview = %q", scene.Preview)
	}
	if len(scene.Performers) != 2 {
		t.Errorf("Performers = %v", scene.Performers)
	}
	if scene.Studio != "MissaX" {
		t.Errorf("Studio = %q", scene.Studio)
	}
	if scene.SiteID != "missax" {
		t.Errorf("SiteID = %q", scene.SiteID)
	}
}

func TestListScenes(t *testing.T) {
	var ts *httptest.Server
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "movies_1_d"):
			_, _ = fmt.Fprintf(w, fixtureListPage, ts.URL, ts.URL)
		case strings.Contains(r.URL.Path, "movies_2_d"):
			_, _ = w.Write([]byte(`<html><body></body></html>`))
		case strings.Contains(r.URL.Path, "trailers/"):
			_, _ = w.Write([]byte(fixtureDetailPage))
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client(), siteBase: ts.URL}
	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{Workers: 1})
	if err != nil {
		t.Fatal(err)
	}

	var scenes []string
	for r := range ch {
		if r.Kind == scraper.KindTotal || r.Kind == scraper.KindStoppedEarly {
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
}

func TestListScenesKnownIDs(t *testing.T) {
	var ts *httptest.Server
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "movies_1_d"):
			_, _ = fmt.Fprintf(w, fixtureListPage, ts.URL, ts.URL)
		default:
			_, _ = w.Write([]byte(fixtureDetailPage))
		}
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client(), siteBase: ts.URL}
	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{
		Workers:  1,
		KnownIDs: map[string]bool{"1002": true},
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
		if r.Kind == scraper.KindStoppedEarly {
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
	if len(scenes) != 1 || scenes[0] != "1001" {
		t.Errorf("got scenes %v, want [1001]", scenes)
	}
}

func TestParseDuration(t *testing.T) {
	cases := []struct {
		input string
		want  int
	}{
		{"25:30", 25*60 + 30},
		{"1:05:30", 1*3600 + 5*60 + 30},
		{"0:45", 45},
	}
	for _, c := range cases {
		if got := parseDuration(c.input); got != c.want {
			t.Errorf("parseDuration(%q) = %d, want %d", c.input, got, c.want)
		}
	}
}

func TestParseDate(t *testing.T) {
	cases := []struct {
		input string
		year  int
		month int
		day   int
	}{
		{"03/15/2026", 2026, 3, 15},
		{"12/01/2020", 2020, 12, 1},
	}
	for _, c := range cases {
		d := parseDate(c.input)
		if d.Year() != c.year || int(d.Month()) != c.month || d.Day() != c.day {
			t.Errorf("parseDate(%q) = %v", c.input, d)
		}
	}
}

func TestSlugFromURL(t *testing.T) {
	cases := []struct {
		url  string
		want string
	}{
		{"https://www.missax.com/tour/trailers/Test-Scene.html", "Test-Scene"},
		{"https://www.missax.com/tour/trailers/Another-One.html", "Another-One"},
	}
	for _, c := range cases {
		if got := slugFromURL(c.url); got != c.want {
			t.Errorf("slugFromURL(%q) = %q, want %q", c.url, got, c.want)
		}
	}
}
