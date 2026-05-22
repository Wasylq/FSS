package nookies

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wasylq/FSS/scraper"
)

const testListingPage = `<!DOCTYPE html><html><body>
<div class="video-card">
    <div class="video-card-img img-hover-zoom">
        <a href="/video/3439/desiree-edens-milk-cookies" class="v-thumb" data-src="BASEURL/tour-content/gilfaf/preview.mp4">
            <img src="BASEURL/tour-content/gilfaf/main.jpg" alt="Desiree Eden&#39;s Milk &amp; Cookies" class="main-video-img ">
            <div class="video-logo">
                <a href="/site/gilfaf"><img src="BASEURL/tour/images/overlays/gilfaf.png" /></a>
            </div>
        </a>
    </div>
    <div class="video-card-text">
        <h4 class="title">
            <a href="/video/3439/desiree-edens-milk-cookies" title="">
                Desiree Eden's Milk &amp; Cook...
            </a>
        </h4>
        <div class="meta">
            <div>
                <a href="/model/desiree-eden" class="tag-btn">Desiree Eden</a>
            </div>
            <span class="date">2026-05-22</span>
        </div>
    </div>
</div><!-- End video-card-->
<div class="video-card">
    <div class="video-card-img img-hover-zoom">
        <a href="/video/3438/mia-river-makes-him-rain-cum" class="v-thumb">
            <img src="BASEURL/tour-content/mylked/main.jpg" alt="Mia River Makes Him Rain Cum" class="main-video-img ">
            <div class="video-logo">
                <a href="/site/mylked"><img src="BASEURL/tour/images/overlays/mylked.png" /></a>
            </div>
        </a>
    </div>
    <div class="video-card-text">
        <h4 class="title">
            <a href="/video/3438/mia-river-makes-him-rain-cum" title="">
                Mia River Makes Him Rain Cum
            </a>
        </h4>
        <div class="meta">
            <div>
                <a href="/model/mia-river" class="tag-btn">Mia River</a>
            </div>
            <span class="date">2026-05-21</span>
        </div>
    </div>
</div><!-- End video-card-->
<ul class="pagination">
<li><a href="?page=1">«</a></li>
<li class="active"><span>1</span></li>
<li><a href="?page=2">2</a></li>
<li><a href="?page=3">3</a></li>
<li><a href="?page=2">»</a></li>
</ul>
</body></html>`

const testDetailPage1 = `<!DOCTYPE html><html><head>
<meta name="description" content="Desiree Eden detail description." />
</head><body>
<h1 style="padding-bottom: 5px; padding-top: 0px;">Desiree Eden&#39;s Milk &amp; Cookies, and GILF Nookie</h1>
<h3>Update Details</h3>
<p>
    <i class="fa-regular fa-calendar"></i> Release Date: May 22, 2026
    <br />
    <i class="fa-solid fa-video"></i>

    12:34

    &nbsp;/&nbsp; <i class="fa-regular fa-images"></i> 48 Photos
</p>
<div class="video-details-block">
    <h3>Description:</h3>
    <p>Desiree Eden detail description.</p>
</div>
<div class="video-details-block video-details-tags-list">
    <h3>Tags:</h3>
    <a class="pill-link" href="/tag/gilf">GILF</a>
    <a class="pill-link" href="/tag/big-tits"> big tits</a>
    <a class="pill-link" href="/tag/handjob"> handjob</a>
</div>
<h1 class="head_line">Nookies Sites</h1>
</body></html>`

const testDetailPage2 = `<!DOCTYPE html><html><head>
<meta name="description" content="Second scene description." />
</head><body>
<h1 style="padding-bottom: 5px; padding-top: 0px;">Mia River Makes Him Rain Cum</h1>
<h3>Update Details</h3>
<p>
    <i class="fa-regular fa-calendar"></i> Release Date: May 21, 2026
    <br />
    <i class="fa-solid fa-video"></i>

    07:52
</p>
<div class="video-details-block">
    <h3>Description:</h3>
    <p>Second scene description.</p>
</div>
<div class="video-details-block video-details-tags-list">
    <h3>Tags:</h3>
    <a class="pill-link" href="/tag/petite">Petite</a>
    <a class="pill-link" href="/tag/cumshot"> cumshot</a>
</div>
</body></html>`

func testListingWithBase(base string) string {
	return fmt.Sprintf(`<!DOCTYPE html><html><body>
<div class="video-card">
    <div class="video-card-img img-hover-zoom">
        <a href="/video/3439/desiree-edens-milk-cookies" class="v-thumb">
            <img src="%s/tour-content/gilfaf/main.jpg" alt="Desiree Eden&#39;s Milk &amp; Cookies" class="main-video-img ">
            <div class="video-logo">
                <a href="/site/gilfaf"><img src="%s/tour/images/overlays/gilfaf.png" /></a>
            </div>
        </a>
    </div>
    <div class="video-card-text">
        <h4 class="title">
            <a href="/video/3439/desiree-edens-milk-cookies" title="">Desiree Eden's Milk &amp; Cook...</a>
        </h4>
        <div class="meta">
            <div><a href="/model/desiree-eden" class="tag-btn">Desiree Eden</a></div>
            <span class="date">2026-05-22</span>
        </div>
    </div>
</div><!-- End video-card-->
<div class="video-card">
    <div class="video-card-img img-hover-zoom">
        <a href="/video/3438/mia-river-makes-him-rain-cum" class="v-thumb">
            <img src="%s/tour-content/mylked/main.jpg" alt="Mia River Makes Him Rain Cum" class="main-video-img ">
            <div class="video-logo">
                <a href="/site/mylked"><img src="%s/tour/images/overlays/mylked.png" /></a>
            </div>
        </a>
    </div>
    <div class="video-card-text">
        <h4 class="title">
            <a href="/video/3438/mia-river-makes-him-rain-cum" title="">Mia River Makes Him Rain Cum</a>
        </h4>
        <div class="meta">
            <div><a href="/model/mia-river" class="tag-btn">Mia River</a></div>
            <span class="date">2026-05-21</span>
        </div>
    </div>
</div><!-- End video-card-->
<ul class="pagination">
<li><a href="?page=1">«</a></li>
<li class="active"><span>1</span></li>
<li><a href="?page=2">2</a></li>
<li><a href="?page=3">3</a></li>
<li><a href="?page=2">»</a></li>
</ul>
</body></html>`, base, base, base, base)
}

func TestParseListingPage(t *testing.T) {
	items := parseListingPage([]byte(testListingPage))
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	it := items[0]
	if it.id != "3439" {
		t.Errorf("id = %q, want 3439", it.id)
	}
	if it.url != "/video/3439/desiree-edens-milk-cookies" {
		t.Errorf("url = %q", it.url)
	}
	if it.title != "Desiree Eden's Milk & Cookies" {
		t.Errorf("title = %q", it.title)
	}
	if it.subSite != "gilfaf" {
		t.Errorf("subSite = %q, want gilfaf", it.subSite)
	}
	want := time.Date(2026, 5, 22, 0, 0, 0, 0, time.UTC)
	if !it.date.Equal(want) {
		t.Errorf("date = %v, want %v", it.date, want)
	}
	if len(it.performers) != 1 || it.performers[0] != "Desiree Eden" {
		t.Errorf("performers = %v", it.performers)
	}

	it2 := items[1]
	if it2.id != "3438" {
		t.Errorf("id = %q, want 3438", it2.id)
	}
	if it2.title != "Mia River Makes Him Rain Cum" {
		t.Errorf("title = %q", it2.title)
	}
	if it2.subSite != "mylked" {
		t.Errorf("subSite = %q, want mylked", it2.subSite)
	}
}

func TestParseDetailPage(t *testing.T) {
	d := parseDetailPage([]byte(testDetailPage1))

	if d.title != "Desiree Eden's Milk & Cookies, and GILF Nookie" {
		t.Errorf("title = %q", d.title)
	}
	if d.duration != 754 {
		t.Errorf("duration = %d, want 754", d.duration)
	}
	if d.description != "Desiree Eden detail description." {
		t.Errorf("description = %q", d.description)
	}
	if len(d.tags) != 3 {
		t.Errorf("tags = %v, want 3", d.tags)
	}
}

func TestEstimateTotal(t *testing.T) {
	body := []byte(testListingPage)
	total := estimateTotal(body, 24)
	if total != 72 {
		t.Errorf("estimateTotal = %d, want 72 (3 pages * 24)", total)
	}
}

func TestHasNextPage(t *testing.T) {
	if !hasNextPage([]byte(testListingPage)) {
		t.Error("expected next page")
	}
	if hasNextPage([]byte(`<html><body>no pagination</body></html>`)) {
		t.Error("expected no next page")
	}
}

func TestPagination(t *testing.T) {
	page1Called := false
	page2Called := false
	var ts *httptest.Server
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/videos":
			if r.URL.Query().Get("page") == "2" {
				page2Called = true
				w.WriteHeader(200)
				return
			}
			page1Called = true
			_, _ = fmt.Fprint(w, testListingWithBase(ts.URL))
		case "/video/3439/desiree-edens-milk-cookies":
			_, _ = fmt.Fprint(w, testDetailPage1)
		case "/video/3438/mia-river-makes-him-rain-cum":
			_, _ = fmt.Fprint(w, testDetailPage2)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client(), base: ts.URL}

	ch, err := s.ListScenes(context.Background(), ts.URL+"/videos", scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}

	var scenes []string
	for r := range ch {
		switch r.Kind {
		case scraper.KindScene:
			scenes = append(scenes, r.Scene.ID)
		case scraper.KindError:
			t.Errorf("unexpected error: %v", r.Err)
		}
	}

	if !page1Called || !page2Called {
		t.Errorf("page1=%v page2=%v, both should be true", page1Called, page2Called)
	}
	if len(scenes) != 2 {
		t.Errorf("got %d scenes, want 2", len(scenes))
	}
	if len(scenes) >= 1 && scenes[0] != "3439" {
		t.Errorf("scene[0] = %q", scenes[0])
	}
}

func TestDetailEnrichment(t *testing.T) {
	var ts *httptest.Server
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/videos":
			if r.URL.Query().Get("page") != "" {
				w.WriteHeader(200)
				return
			}
			_, _ = fmt.Fprint(w, testListingWithBase(ts.URL))
		case "/video/3439/desiree-edens-milk-cookies":
			_, _ = fmt.Fprint(w, testDetailPage1)
		case "/video/3438/mia-river-makes-him-rain-cum":
			_, _ = fmt.Fprint(w, testDetailPage2)
		default:
			w.WriteHeader(200)
		}
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client(), base: ts.URL}

	ch, err := s.ListScenes(context.Background(), ts.URL+"/videos", scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}

	var got []scraper.SceneResult
	for r := range ch {
		if r.Kind == scraper.KindScene {
			got = append(got, r)
		}
	}

	if len(got) < 1 {
		t.Fatal("expected at least 1 scene")
	}

	scene := got[0].Scene
	if scene.Title != "Desiree Eden's Milk & Cookies, and GILF Nookie" {
		t.Errorf("title = %q, want full title from detail", scene.Title)
	}
	if scene.Duration != 754 {
		t.Errorf("duration = %d, want 754", scene.Duration)
	}
	if scene.Description != "Desiree Eden detail description." {
		t.Errorf("description = %q", scene.Description)
	}
	if len(scene.Tags) != 3 {
		t.Errorf("tags = %v, want 3", scene.Tags)
	}
	if scene.SiteID != "gilfaf" {
		t.Errorf("siteID = %q, want gilfaf", scene.SiteID)
	}
	if len(scene.Performers) != 1 || scene.Performers[0] != "Desiree Eden" {
		t.Errorf("performers = %v", scene.Performers)
	}
}

func TestEarlyStop(t *testing.T) {
	var ts *httptest.Server
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/videos":
			if r.URL.Query().Get("page") != "" {
				w.WriteHeader(200)
				return
			}
			_, _ = fmt.Fprint(w, testListingWithBase(ts.URL))
		case "/video/3439/desiree-edens-milk-cookies":
			_, _ = fmt.Fprint(w, testDetailPage1)
		case "/video/3438/mia-river-makes-him-rain-cum":
			_, _ = fmt.Fprint(w, testDetailPage2)
		default:
			w.WriteHeader(200)
		}
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client(), base: ts.URL}

	known := map[string]bool{"3438": true}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/videos", scraper.ListOpts{KnownIDs: known})
	if err != nil {
		t.Fatal(err)
	}

	var scenes []string
	stoppedEarly := false
	for r := range ch {
		switch r.Kind {
		case scraper.KindScene:
			scenes = append(scenes, r.Scene.ID)
		case scraper.KindStoppedEarly:
			stoppedEarly = true
		}
	}

	if !stoppedEarly {
		t.Error("expected StoppedEarly")
	}
	if len(scenes) != 1 {
		t.Errorf("got %d scenes, want 1", len(scenes))
	}
}

func TestMatchesURL(t *testing.T) {
	s := New()
	tests := []struct {
		url  string
		want bool
	}{
		{"https://nookies.com/", true},
		{"https://www.nookies.com/videos", true},
		{"https://nookies.com/site/clubtug", true},
		{"https://nookies.com/model/jessica-ryan", true},
		{"https://nookies.com/tag/handjob", true},
		{"https://example.com/", false},
	}
	for _, tc := range tests {
		if got := s.MatchesURL(tc.url); got != tc.want {
			t.Errorf("MatchesURL(%q) = %v, want %v", tc.url, got, tc.want)
		}
	}
}

func TestSiteMode(t *testing.T) {
	var ts *httptest.Server
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/site/clubtug":
			if r.URL.Query().Get("page") != "" {
				w.WriteHeader(200)
				return
			}
			_, _ = fmt.Fprint(w, testListingWithBase(ts.URL))
		case "/video/3439/desiree-edens-milk-cookies":
			_, _ = fmt.Fprint(w, testDetailPage1)
		case "/video/3438/mia-river-makes-him-rain-cum":
			_, _ = fmt.Fprint(w, testDetailPage2)
		default:
			w.WriteHeader(200)
		}
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client(), base: ts.URL}

	ch, err := s.ListScenes(context.Background(), ts.URL+"/site/clubtug", scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}

	var count int
	for r := range ch {
		if r.Kind == scraper.KindScene {
			count++
		}
	}

	if count != 2 {
		t.Errorf("got %d scenes, want 2", count)
	}
}

func TestModelMode(t *testing.T) {
	var ts *httptest.Server
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/model/jessica-ryan":
			_, _ = fmt.Fprint(w, testListingWithBase(ts.URL))
		case "/video/3439/desiree-edens-milk-cookies":
			_, _ = fmt.Fprint(w, testDetailPage1)
		case "/video/3438/mia-river-makes-him-rain-cum":
			_, _ = fmt.Fprint(w, testDetailPage2)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client(), base: ts.URL}

	ch, err := s.ListScenes(context.Background(), ts.URL+"/model/jessica-ryan", scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}

	var count int
	for r := range ch {
		if r.Kind == scraper.KindScene {
			count++
		}
	}

	if count != 2 {
		t.Errorf("got %d scenes, want 2", count)
	}
}
