package julesjordan

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
		{"https://www.julesjordan.com/trial/", true},
		{"https://julesjordan.com/trial/categories/movies.html", true},
		{"https://www.julesjordan.com/trial/models/kendra-lust.html", true},
		{"https://www.julesjordan.com/trial/dvds/dvds.html", true},
		{"https://example.com/julesjordan", false},
		{"", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.want {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.want)
		}
	}
}

func TestParseListingCards(t *testing.T) {
	body := []byte(`
<div class="jj-content-card">
<a href="/scenes/Scene-One_vids.html" class="jj-card-thumb">
<img id="set-target-100" class="jj-thumb-img stdimage" src="/thumbs/100.jpg" /></a>
<div class="jj-card-body">
<h2 class="jj-card-title">Scene One Title</h2>
<div class="jj-card-meta">Starring: <span class="update_models">
<a href="/models/alice.html">Alice</a>, <a href="/models/bob.html">Bob</a></span></div>
<div class="jj-card-date">Released: April 28, 2026</div>
</div></div>

<div class="jj-content-card">
<a href="/scenes/Scene-Two_vids.html" class="jj-card-thumb">
<img id="set-target-200" class="jj-thumb-img" src="/thumbs/200.jpg" /></a>
<div class="jj-card-body">
<h2 class="jj-card-title">Scene Two Title</h2>
<div class="jj-card-meta">Starring: <span class="update_models">
<a href="/models/charlie.html">Charlie</a></span></div>
<div class="jj-card-date">Released: April 20, 2026</div>
</div></div>
`)
	items := parseListingCards(body, "https://test.local")
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}

	s := items[0]
	if s.slug != "Scene-One" {
		t.Errorf("slug = %q, want Scene-One", s.slug)
	}
	if s.title != "Scene One Title" {
		t.Errorf("title = %q", s.title)
	}
	if len(s.performers) != 2 || s.performers[0] != "Alice" {
		t.Errorf("performers = %v", s.performers)
	}
	wantDate := time.Date(2026, 4, 28, 0, 0, 0, 0, time.UTC)
	if !s.date.Equal(wantDate) {
		t.Errorf("date = %v, want %v", s.date, wantDate)
	}
	if s.thumb != "/thumbs/100.jpg" {
		t.Errorf("thumb = %q", s.thumb)
	}
	if s.url != "https://test.local/scenes/Scene-One_vids.html" {
		t.Errorf("url = %q", s.url)
	}
}

func TestParseDetailPage(t *testing.T) {
	body := []byte(`
<h1 class="scene-title">Detail Scene Title</h1>
<div class="scene-meta">
<div class="meta-item"><div class="lbl">Starring</div>
<div class="val"><span class="update_models">
<a href="/models/alice.html">Alice</a>, <a href="/models/bob.html">Bob</a></span></div></div>
<div class="meta-item"><div class="lbl">Released</div>
<div class="val">April 28, 2026</div></div>
</div>
<div class="scene-desc">A great scene description with &amp; special chars.</div>
<div class="scene-cats"><span class="cats-lbl">Categories</span>
<a href="category.php?id=11" class="cat-tag">Anal</a><a href="category.php?id=29" class="cat-tag">Blowjobs</a></div>
`)
	d := parseDetailPage(body)
	if d.title != "Detail Scene Title" {
		t.Errorf("title = %q", d.title)
	}
	if d.description != "A great scene description with & special chars." {
		t.Errorf("description = %q", d.description)
	}
	if len(d.tags) != 2 || d.tags[0] != "Anal" {
		t.Errorf("tags = %v", d.tags)
	}
	if len(d.performers) != 2 || d.performers[0] != "Alice" {
		t.Errorf("performers = %v", d.performers)
	}
	wantDate := time.Date(2026, 4, 28, 0, 0, 0, 0, time.UTC)
	if !d.date.Equal(wantDate) {
		t.Errorf("date = %v, want %v", d.date, wantDate)
	}
}

func TestParseDetailPageEmpty(t *testing.T) {
	d := parseDetailPage([]byte(`<div>nothing here</div>`))
	if d.title != "" || d.description != "" || len(d.tags) != 0 {
		t.Errorf("expected empty detail, got %+v", d)
	}
}

func TestParseDVDListingPage(t *testing.T) {
	body := []byte(`
<a href="/dvds/DVD-One.html" class="dvd-listing-card">
<div class="dvd-listing-thumb"><img src="/thumbs/dvd1.jpg" /></div>
<div class="dvd-listing-bar">
<span class="dvd-listing-name">DVD One Title</span>
<span class="dvd-listing-date">11/13/2025</span>
</div></a>
<a href="/dvds/DVD-Two.html" class="dvd-listing-card">
<div class="dvd-listing-thumb"><img src="/thumbs/dvd2.jpg" /></div>
<div class="dvd-listing-bar">
<span class="dvd-listing-name">DVD Two Title</span>
<span class="dvd-listing-date">10/20/2025</span>
</div></a>
`)
	entries := parseDVDListingPage(body)
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
	if entries[0].url != "/dvds/DVD-One.html" || entries[0].name != "DVD One Title" {
		t.Errorf("entry[0] = %+v", entries[0])
	}
	if entries[1].name != "DVD Two Title" {
		t.Errorf("entry[1].name = %q", entries[1].name)
	}
}

func TestExtractDVDSceneURLs(t *testing.T) {
	body := []byte(`
<a href="/scenes/Scene-A_vids.html" class="dvd-watch-btn">Watch</a>
<a href="/scenes/Scene-A_vids.html"><img src="thumb.jpg" /></a>
<a href="/scenes/Scene-B_vids.html" class="dvd-watch-btn">Watch</a>
`)
	urls := extractDVDSceneURLs(body)
	if len(urls) != 2 {
		t.Fatalf("got %d URLs, want 2 (deduped)", len(urls))
	}
	if urls[0] != "/scenes/Scene-A_vids.html" {
		t.Errorf("url[0] = %q", urls[0])
	}
	if urls[1] != "/scenes/Scene-B_vids.html" {
		t.Errorf("url[1] = %q", urls[1])
	}
}

const listingCardTpl = `<div class="jj-content-card">
<a href="%s/scenes/scene-%d_vids.html" class="jj-card-thumb">
<img id="set-target-%d" class="jj-thumb-img" src="/thumbs/%d.jpg" /></a>
<div class="jj-card-body">
<h2 class="jj-card-title">Scene %d</h2>
<div class="jj-card-meta">Starring: <span class="update_models">
<a href="/models/test.html">Test Model</a></span></div>
<div class="jj-card-date">Released: January 15, 2026</div>
</div></div>`

const detailTpl = `
<h1 class="scene-title">Detail Title</h1>
<div class="scene-meta">
<div class="meta-item"><div class="lbl">Starring</div>
<div class="val"><span class="update_models"><a href="/models/test.html">Test Model</a></span></div></div>
<div class="meta-item"><div class="lbl">Released</div>
<div class="val">January 15, 2026</div></div></div>
<div class="scene-desc">Test description.</div>
<div class="scene-cats"><a class="cat-tag">TestTag</a></div>
`

func buildListingPage(base string, ids []int) []byte {
	var sb string
	for _, id := range ids {
		sb += fmt.Sprintf(listingCardTpl, base, id, id, id, id)
	}
	return []byte(sb)
}

func newTestServer(pages [][]int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")

		switch {
		case r.URL.Path == "/models/test-model.html":
			_, _ = w.Write(buildListingPage("", pages[0]))

		case strings.HasPrefix(r.URL.Path, "/scenes/"):
			_, _ = fmt.Fprint(w, detailTpl)

		case r.URL.Path == "/dvds/dvds.html":
			_, _ = fmt.Fprint(w, `<a href="/dvds/test-dvd.html" class="dvd-listing-card">
<div class="dvd-listing-bar"><span class="dvd-listing-name">Test DVD</span></div></a>`)

		case r.URL.Path == "/dvds/test-dvd.html":
			_, _ = fmt.Fprint(w, `<a href="/scenes/dvd-scene-1_vids.html">Watch</a>
<a href="/scenes/dvd-scene-2_vids.html">Watch</a>`)

		default:
			pageNum := 0
			_, _ = fmt.Sscanf(r.URL.Path, "/categories/movies_%d_d.html", &pageNum)
			if pageNum == 0 {
				pageNum = 1
			}
			idx := pageNum - 1
			if idx >= 0 && idx < len(pages) {
				_, _ = w.Write(buildListingPage("", pages[idx]))
			} else {
				_, _ = fmt.Fprint(w, `<div>empty</div>`)
			}
		}
	}))
}

func TestListScenes(t *testing.T) {
	ts := newTestServer([][]int{{100, 200}})
	defer ts.Close()

	s := &Scraper{client: ts.Client(), base: ts.URL}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/categories/movies.html", scraper.ListOpts{
		Delay: time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}

	results := testutil.CollectScenes(t, ch)
	if len(results) != 2 {
		t.Fatalf("got %d scenes, want 2", len(results))
	}
	if results[0].Description != "Test description." {
		t.Errorf("description = %q", results[0].Description)
	}
	if len(results[0].Tags) != 1 || results[0].Tags[0] != "TestTag" {
		t.Errorf("tags = %v", results[0].Tags)
	}
}

func TestListScenesKnownIDs(t *testing.T) {
	ts := newTestServer([][]int{{1, 2, 3, 4}})
	defer ts.Close()

	s := &Scraper{client: ts.Client(), base: ts.URL}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/categories/movies.html", scraper.ListOpts{
		KnownIDs: map[string]bool{"scene-3": true},
		Delay:    time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}

	results, stoppedEarly := testutil.CollectScenesWithStop(t, ch)
	if !stoppedEarly {
		t.Error("expected StoppedEarly signal")
	}
	if len(results) != 2 {
		t.Fatalf("got %d scenes, want 2", len(results))
	}
}

func TestListScenesPagination(t *testing.T) {
	page1 := make([]int, 32)
	for i := range page1 {
		page1[i] = i + 1
	}
	page2 := []int{33, 34, 35}

	ts := newTestServer([][]int{page1, page2})
	defer ts.Close()

	s := &Scraper{client: ts.Client(), base: ts.URL}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/categories/movies.html", scraper.ListOpts{
		Delay: time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}

	results := testutil.CollectScenes(t, ch)
	if len(results) != 35 {
		t.Fatalf("got %d scenes, want 35", len(results))
	}
}

func TestListScenesModelPage(t *testing.T) {
	ts := newTestServer([][]int{{10, 20, 30}})
	defer ts.Close()

	s := &Scraper{client: ts.Client(), base: ts.URL}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/models/test-model.html", scraper.ListOpts{
		Delay: time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}

	results := testutil.CollectScenes(t, ch)
	if len(results) != 3 {
		t.Fatalf("got %d scenes, want 3", len(results))
	}
}

func TestListScenesDVDMode(t *testing.T) {
	ts := newTestServer([][]int{{100}})
	defer ts.Close()

	s := &Scraper{client: ts.Client(), base: ts.URL}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/dvds/dvds.html", scraper.ListOpts{
		Delay: time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}

	results := testutil.CollectScenes(t, ch)
	if len(results) != 2 {
		t.Fatalf("got %d scenes, want 2", len(results))
	}
	if results[0].Series != "Test DVD" {
		t.Errorf("series = %q, want Test DVD", results[0].Series)
	}
}
