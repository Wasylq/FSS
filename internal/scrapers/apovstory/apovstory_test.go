package apovstory

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
	"github.com/Wasylq/FSS/scraper"
)

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url  string
		want bool
	}{
		{"https://apovstory.com", true},
		{"https://www.apovstory.com/updates/page_1.html", true},
		{"https://apovstory.com/trailers/Making-a-Man-II.html", true},
		{"https://www.manyvids.com/Profile/123/foo", false},
		{"https://example.com", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.want {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.want)
		}
	}
}

func TestParseDuration(t *testing.T) {
	cases := []struct {
		input string
		want  int
	}{
		{"32:06", 32*60 + 6},
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
		month time.Month
		day   int
	}{
		{"September 1, 2023", 2023, 9, 1},
		{"January 15, 2026", 2026, 1, 15},
		{"December 31, 2020", 2020, 12, 31},
	}
	for _, c := range cases {
		d := parseDate(c.input)
		if d.Year() != c.year || d.Month() != c.month || d.Day() != c.day {
			t.Errorf("parseDate(%q) = %v", c.input, d)
		}
	}
}

func TestParseDateInvalid(t *testing.T) {
	d := parseDate("")
	if !d.IsZero() {
		t.Errorf("parseDate empty should be zero, got %v", d)
	}
}

const fixtureListPage = `<!DOCTYPE html><html><body>
<h2><span>Latest Updates</span></h2>
<div class="videoBlock" data-setid="117">
	<div class="videoPic">
<div class="b117_videothumb_123">
	<a href="%s/trailers/Making-a-Man-II.html"></a>
	<img src="/content//contentthumbs/15/99/1599-2x.jpg" alt="" class="video_placeholder" />
	<video width="100%%" height="100%%" loop muted poster="/content//contentthumbs/15/99/1599-2x.jpg">
		<source src='/videothumbs/apov_makingaman2_preview_sm.mp4' type="video/mp4" />
	</video>
</div>
	</div>
	<div class="updateDetails">
		<div class="updateDetails_left">
			<div class="updateDetails_title">
				<a href="%s/trailers/Making-a-Man-II.html" title="Making a Man II">Making a Man II</a>
			</div>
			<div class="updateDetails_models">
				<a href="%s/models/ReaganFoxx.html">Reagan Foxx</a>
				 &amp; <a href="%s/models/ChadAlva.html">Chad Alva</a>
			</div>
		</div><!--//updateDetails_left-->
		<div class="updateDetails_right">
			<div class="updateDetails_rating">
				<i class="fa fa-star-o"></i> 9.0 | 32:06
			</div>
			<div class="updateDetails_date">
				September 1, 2023
			</div>
		</div><!--//updateDetails_right-->
	</div><!--//updateDetails-->
	</div>

<div class="videoBlock" data-setid="116">
	<div class="videoPic">
<div class="b116_videothumb_456">
	<a href="%s/trailers/Another-Scene.html"></a>
	<img src="/content//contentthumbs/15/98/1598-2x.jpg" alt="" class="video_placeholder" />
	<video width="100%%" height="100%%" loop muted>
		<source src='/videothumbs/apov_anotherscene_preview_sm.mp4' type="video/mp4" />
	</video>
</div>
	</div>
	<div class="updateDetails">
		<div class="updateDetails_left">
			<div class="updateDetails_title">
				<a href="%s/trailers/Another-Scene.html" title="Another Scene">Another Scene</a>
			</div>
			<div class="updateDetails_models">
				<a href="%s/models/ActorA.html">Actor A</a>
			</div>
		</div><!--//updateDetails_left-->
		<div class="updateDetails_right">
			<div class="updateDetails_rating">
				<i class="fa fa-star-o"></i> 8.5 | 25:30
			</div>
			<div class="updateDetails_date">
				August 15, 2023
			</div>
		</div><!--//updateDetails_right-->
	</div><!--//updateDetails-->
	</div>

<h2><span>Most Popular Updates</span></h2>
<div class="videoBlock" data-setid="50">
	<div class="videoPic">
<div class="b50_videothumb_789">
	<a href="%s/trailers/Popular-Scene.html"></a>
	<img src="/content//contentthumbs/10/50/1050-2x.jpg" alt="" class="video_placeholder" />
</div>
	</div>
	<div class="updateDetails">
		<div class="updateDetails_left">
			<div class="updateDetails_title">
				<a href="%s/trailers/Popular-Scene.html" title="Popular Scene">Popular Scene</a>
			</div>
			<div class="updateDetails_models">
				<a href="%s/models/PopularActor.html">Popular Actor</a>
			</div>
		</div><!--//updateDetails_left-->
		<div class="updateDetails_right">
			<div class="updateDetails_rating">
				<i class="fa fa-star-o"></i> 10.0 | 40:00
			</div>
			<div class="updateDetails_date">
				June 1, 2022
			</div>
		</div><!--//updateDetails_right-->
	</div><!--//updateDetails-->
	</div>
</body></html>`

const fixtureModelPage = `<!DOCTYPE html><html><body>
<h1>Reagan Foxx</h1>
<div class="videoBlock" data-setid="117">
	<div class="videoPic">
<div class="b117_videothumb_123">
	<a href="%s/trailers/Making-a-Man-II.html"></a>
	<img src="/content//contentthumbs/15/99/1599-2x.jpg" alt="" class="video_placeholder" />
	<video width="100%%" height="100%%" loop muted poster="/content//contentthumbs/15/99/1599-2x.jpg">
		<source src='/videothumbs/apov_makingaman2_preview_sm.mp4' type="video/mp4" />
	</video>
</div>
	</div>
	<div class="updateDetails">
		<div class="updateDetails_left">
			<div class="updateDetails_title">
				<a href="%s/trailers/Making-a-Man-II.html" title="Making a Man II">Making a Man II</a>
			</div>
			<div class="updateDetails_models">
				<a href="%s/models/ReaganFoxx.html">Reagan Foxx</a>
			</div>
		</div><!--//updateDetails_left-->
		<div class="updateDetails_right">
			<div class="updateDetails_rating">
				<i class="fa fa-star-o"></i> 9.0 | 32:06
			</div>
			<div class="updateDetails_date">
				September 1, 2023
			</div>
		</div><!--//updateDetails_right-->
	</div><!--//updateDetails-->
	</div>

<div class="videoBlock" data-setid="116">
	<div class="videoPic">
<div class="b116_videothumb_456">
	<a href="%s/trailers/Another-Scene.html"></a>
	<img src="/content//contentthumbs/15/98/1598-2x.jpg" alt="" class="video_placeholder" />
</div>
	</div>
	<div class="updateDetails">
		<div class="updateDetails_left">
			<div class="updateDetails_title">
				<a href="%s/trailers/Another-Scene.html" title="Another Scene">Another Scene</a>
			</div>
			<div class="updateDetails_models">
				<a href="%s/models/ReaganFoxx.html">Reagan Foxx</a>
			</div>
		</div><!--//updateDetails_left-->
		<div class="updateDetails_right">
			<div class="updateDetails_rating">
				<i class="fa fa-star-o"></i> 8.5 | 25:30
			</div>
			<div class="updateDetails_date">
				August 15, 2023
			</div>
		</div><!--//updateDetails_right-->
	</div><!--//updateDetails-->
	</div>
</body></html>`

const fixtureDetailPage = `<!DOCTYPE html><html><head>
<meta property="og:description" content="A compelling scene description with details about the plot."/>
</head><body>
<li><span>CATEGORIES:</span>
<a href="https://apovstory.com/categories/Blowjob_1_d.html">Blowjob</a>,
<a href="https://apovstory.com/categories/milf_1_d.html">MILF</a>,
<a href="https://apovstory.com/categories/Taboo_1_d.html">Taboo</a>
</li>
</body></html>`

func TestFetchPage(t *testing.T) {
	var ts *httptest.Server
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, fixtureListPage, ts.URL, ts.URL, ts.URL, ts.URL, ts.URL, ts.URL, ts.URL, ts.URL, ts.URL, ts.URL)
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client(), siteBase: ts.URL}
	entries, err := s.fetchPage(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2 (should exclude Popular section)", len(entries))
	}

	e := entries[0]
	if e.id != "117" {
		t.Errorf("ID = %q, want 117", e.id)
	}
	if e.title != "Making a Man II" {
		t.Errorf("Title = %q", e.title)
	}
	if !strings.Contains(e.url, "Making-a-Man-II.html") {
		t.Errorf("URL = %q", e.url)
	}
	if !strings.Contains(e.thumbnail, "1599-2x.jpg") {
		t.Errorf("Thumbnail = %q", e.thumbnail)
	}
	if !strings.Contains(e.preview, "apov_makingaman2_preview_sm.mp4") {
		t.Errorf("Preview = %q", e.preview)
	}
	if len(e.performers) != 2 || e.performers[0] != "Reagan Foxx" || e.performers[1] != "Chad Alva" {
		t.Errorf("Performers = %v", e.performers)
	}
	if e.duration != 32*60+6 {
		t.Errorf("Duration = %d, want %d", e.duration, 32*60+6)
	}
	if e.date.Year() != 2023 || e.date.Month() != 9 || e.date.Day() != 1 {
		t.Errorf("Date = %v", e.date)
	}
	if e.rating != 9.0 {
		t.Errorf("Rating = %f", e.rating)
	}
}

func TestFetchDetail(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(fixtureDetailPage))
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client(), siteBase: ts.URL}
	entry := listEntry{
		id:         "117",
		title:      "Making a Man II",
		url:        ts.URL + "/trailers/Making-a-Man-II.html",
		thumbnail:  ts.URL + "/content//contentthumbs/15/99/1599-2x.jpg",
		preview:    ts.URL + "/videothumbs/apov_makingaman2_preview_sm.mp4",
		performers: []string{"Reagan Foxx", "Chad Alva"},
		duration:   32*60 + 6,
		date:       time.Date(2023, 9, 1, 0, 0, 0, 0, time.UTC),
	}

	scene, err := s.fetchDetail(context.Background(), ts.URL, entry)
	if err != nil {
		t.Fatal(err)
	}

	if scene.ID != "117" {
		t.Errorf("ID = %q", scene.ID)
	}
	if scene.Title != "Making a Man II" {
		t.Errorf("Title = %q", scene.Title)
	}
	if scene.SiteID != "apovstory" {
		t.Errorf("SiteID = %q", scene.SiteID)
	}
	if scene.Studio != "A POV Story" {
		t.Errorf("Studio = %q", scene.Studio)
	}
	if scene.Description != "A compelling scene description with details about the plot." {
		t.Errorf("Description = %q", scene.Description)
	}
	if len(scene.Tags) != 3 || scene.Tags[0] != "Blowjob" || scene.Tags[1] != "MILF" || scene.Tags[2] != "Taboo" {
		t.Errorf("Tags = %v", scene.Tags)
	}
	if len(scene.Performers) != 2 {
		t.Errorf("Performers = %v", scene.Performers)
	}
	if scene.Duration != 32*60+6 {
		t.Errorf("Duration = %d", scene.Duration)
	}
}

func TestListScenes(t *testing.T) {
	var ts *httptest.Server
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "page_1"):
			_, _ = fmt.Fprintf(w, fixtureListPage, ts.URL, ts.URL, ts.URL, ts.URL, ts.URL, ts.URL, ts.URL, ts.URL, ts.URL, ts.URL)
		case strings.Contains(r.URL.Path, "page_2"):
			_, _ = w.Write([]byte(`<html><body><h2><span>Latest Updates</span></h2><h2><span>Most Popular Updates</span></h2></body></html>`))
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

	scenes := testutil.CollectScenes(t, ch)

	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2", len(scenes))
	}
}

func TestListScenesModel(t *testing.T) {
	var ts *httptest.Server
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/models/"):
			_, _ = fmt.Fprintf(w, fixtureModelPage, ts.URL, ts.URL, ts.URL, ts.URL, ts.URL, ts.URL)
		case strings.Contains(r.URL.Path, "trailers/"):
			_, _ = w.Write([]byte(fixtureDetailPage))
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client(), siteBase: ts.URL}
	modelURL := ts.URL + "/models/ReaganFoxx.html"
	ch, err := s.ListScenes(context.Background(), modelURL, scraper.ListOpts{Workers: 1})
	if err != nil {
		t.Fatal(err)
	}

	scenes := testutil.CollectScenes(t, ch)
	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2", len(scenes))
	}
	if scenes[0].StudioURL != modelURL && scenes[1].StudioURL != modelURL {
		t.Error("expected StudioURL to be the model URL")
	}
}

func TestListScenesModelKnownIDs(t *testing.T) {
	var ts *httptest.Server
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/models/"):
			_, _ = fmt.Fprintf(w, fixtureModelPage, ts.URL, ts.URL, ts.URL, ts.URL, ts.URL, ts.URL)
		case strings.Contains(r.URL.Path, "trailers/"):
			_, _ = w.Write([]byte(fixtureDetailPage))
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client(), siteBase: ts.URL}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/models/ReaganFoxx.html", scraper.ListOpts{
		Workers:  1,
		KnownIDs: map[string]bool{"116": true},
	})
	if err != nil {
		t.Fatal(err)
	}

	scenes := testutil.CollectScenes(t, ch)
	if len(scenes) != 1 {
		t.Fatalf("got %d scenes, want 1 (skip known ID 116)", len(scenes))
	}
}

func TestListScenesKnownIDs(t *testing.T) {
	var ts *httptest.Server
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "page_1"):
			_, _ = fmt.Fprintf(w, fixtureListPage, ts.URL, ts.URL, ts.URL, ts.URL, ts.URL, ts.URL, ts.URL, ts.URL, ts.URL, ts.URL)
		default:
			_, _ = w.Write([]byte(fixtureDetailPage))
		}
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client(), siteBase: ts.URL}
	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{
		Workers:  1,
		KnownIDs: map[string]bool{"116": true},
	})
	if err != nil {
		t.Fatal(err)
	}

	scenes, stoppedEarly := testutil.CollectScenesWithStop(t, ch)

	if !stoppedEarly {
		t.Error("expected StoppedEarly signal")
	}
	if len(scenes) != 1 || scenes[0].ID != "117" {
		t.Errorf("got scenes %v, want [117]", scenes)
	}
}
