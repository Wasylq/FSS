package nubiles

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wasylq/FSS/scraper"
)

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url  string
		want bool
	}{
		{"https://nubiles-porn.com", true},
		{"https://www.nubiles-porn.com", true},
		{"https://nubiles-porn.com/model/profile/4854/cherie-deville", true},
		{"https://nubiles-porn.com/video/category/3/milf", true},
		{"https://nubiles.net", true},
		{"https://momsteachsex.com", true},
		{"https://stepsiblingscaught.com", true},
		{"https://myfamilypies.com", true},
		{"https://princesscum.com", true},
		{"https://badteenspunished.com", true},
		{"https://nubileset.com", true},
		{"https://petitehdporn.com", true},
		{"https://cumswappingsis.com", true},
		{"https://familyswap.xxx", true},
		{"https://caughtmycoach.com", true},
		{"https://detentiongirls.com", true},
		{"https://realitysis.com", true},
		{"https://shesbreedingmaterial.com", true},
		{"https://youngermommy.com", true},
		{"https://petiteballerinasfucked.com", true},
		{"https://nubiles-casting.com", true},
		{"https://www.brazzers.com", false},
		{"https://example.com", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.want {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.want)
		}
	}
}

func TestParseFilter(t *testing.T) {
	cases := []struct {
		url      string
		wantMode filterMode
		wantID   string
	}{
		{"https://nubiles-porn.com", filterAll, ""},
		{"https://momsteachsex.com", filterAll, ""},
		{"https://nubiles-porn.com/model/profile/4854/cherie-deville", filterModel, "4854"},
		{"https://nubiles-porn.com/video/category/3/milf", filterCategory, "3"},
	}
	for _, c := range cases {
		f := parseFilter(c.url)
		if f.mode != c.wantMode {
			t.Errorf("parseFilter(%q).mode = %d, want %d", c.url, f.mode, c.wantMode)
		}
		if f.id != c.wantID {
			t.Errorf("parseFilter(%q).id = %q, want %q", c.url, f.id, c.wantID)
		}
	}
}

func TestListingURL(t *testing.T) {
	base := "https://nubiles-porn.com"
	cases := []struct {
		f      filter
		offset int
		want   string
	}{
		{filter{mode: filterAll}, 0, "https://nubiles-porn.com/video/gallery"},
		{filter{mode: filterAll}, 12, "https://nubiles-porn.com/video/gallery/12"},
		{filter{mode: filterModel, id: "4854", slug: "cherie-deville"}, 0, "https://nubiles-porn.com/video/model/4854/cherie-deville"},
		{filter{mode: filterModel, id: "4854", slug: "cherie-deville"}, 12, "https://nubiles-porn.com/video/model/4854/cherie-deville/12"},
		{filter{mode: filterCategory, id: "3", slug: "milf"}, 0, "https://nubiles-porn.com/video/category/3/milf"},
		{filter{mode: filterCategory, id: "3", slug: "milf"}, 24, "https://nubiles-porn.com/video/category/3/milf/24"},
	}
	for _, c := range cases {
		got := listingURL(base, c.f, c.offset)
		if got != c.want {
			t.Errorf("listingURL(%v, %d) = %q, want %q", c.f, c.offset, got, c.want)
		}
	}
}

func TestBestSrcset(t *testing.T) {
	srcset := "https://img.example.com/cover320.jpg 320w,https://img.example.com/cover640.jpg 640w,https://img.example.com/cover960.jpg 960w,"
	got := bestSrcset(srcset)
	if got != "https://img.example.com/cover960.jpg" {
		t.Errorf("bestSrcset = %q, want 960w URL", got)
	}
}

const listingHTML = `<html><body>
<div class="page-item-dropdown"><button>1 of 3</button></div>
<figure class=" ">
    <div class="img-wrapper">
        <a href="/video/watch/12345/test-scene-s1e1">
            <picture>
                <img data-srcset="https://images.example.com/cover320.jpg 320w,https://images.example.com/cover960.jpg 960w," class="lazyload content-grid-image" alt="Test Scene">
            </picture>
            <div data-preview-src="https://videos.example.com/preview.mp4"></div>
        </a>
    </div>
    <figcaption>
        <div class="caption-header">
            <span class="title"><a href="/video/watch/12345/test-scene-s1e1">Test Scene - S1:E1</a></span>
        </div>
        <div class="models ">
            <a class="model" href="/model/profile/100/alice">Alice</a>
            <a class="model" href="/model/profile/200/bob">Bob</a>
        </div>
        <a href="https://momsteachsex.com" class="site-link">MomsTeachSex</a>
        &ndash; <span class="date">Apr 20, 2026</span>
    </figcaption>
</figure>
<figure class=" ">
    <div class="img-wrapper">
        <a href="/video/watch/12344/second-scene-s2e3">
            <picture>
                <img data-srcset="https://images.example.com/cover2_960.jpg 960w," class="lazyload content-grid-image" alt="Second Scene">
            </picture>
        </a>
    </div>
    <figcaption>
        <div class="caption-header">
            <span class="title"><a href="/video/watch/12344/second-scene-s2e3">Second Scene - S2:E3</a></span>
        </div>
        <div class="models ">
            <a class="model" href="/model/profile/300/carol">Carol</a>
        </div>
        <a class="site-link">Nubiles-Porn</a>
        &ndash; <span class="date">Apr 18, 2026</span>
    </figcaption>
</figure>
<div class="page-item-dropdown"><button>1 of 3</button></div>
</body></html>`

const detailHTML = `<html><head>
<meta name="description" content="A detailed description of the scene.">
<meta name="keywords" content="Big Boobs,Blowjob,Brunette,Milf">
<meta property="og:image" content="https://images.example.com/og_cover960.jpg">
</head><body></body></html>`

func TestFetchListing(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, listingHTML)
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client()}
	entries, totalPages, err := s.fetchListing(context.Background(), ts.URL+"/video/gallery")
	if err != nil {
		t.Fatal(err)
	}

	if totalPages != 3 {
		t.Errorf("totalPages = %d, want 3", totalPages)
	}
	if len(entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(entries))
	}

	e := entries[0]
	if e.id != "12345" {
		t.Errorf("id = %q, want 12345", e.id)
	}
	if e.title != "Test Scene - S1:E1" {
		t.Errorf("title = %q", e.title)
	}
	if e.thumbnail != "https://images.example.com/cover960.jpg" {
		t.Errorf("thumbnail = %q", e.thumbnail)
	}
	if e.preview != "https://videos.example.com/preview.mp4" {
		t.Errorf("preview = %q", e.preview)
	}
	if len(e.performers) != 2 || e.performers[0] != "Alice" {
		t.Errorf("performers = %v", e.performers)
	}
	if e.subSite != "MomsTeachSex" {
		t.Errorf("subSite = %q", e.subSite)
	}
	if e.date != "Apr 20, 2026" {
		t.Errorf("date = %q", e.date)
	}
}

func TestFetchDetail(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, detailHTML)
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client()}
	entry := listEntry{
		id: "12345", title: "Test Scene - S1:E1",
		url:        "/video/watch/12345/test-scene-s1e1",
		thumbnail:  "https://images.example.com/cover960.jpg",
		performers: []string{"Alice"}, subSite: "MomsTeachSex",
		date: "Apr 20, 2026",
	}

	scene, err := s.fetchDetail(context.Background(), ts.URL, entry)
	if err != nil {
		t.Fatal(err)
	}

	if scene.Description != "A detailed description of the scene." {
		t.Errorf("Description = %q", scene.Description)
	}
	if len(scene.Tags) != 4 || scene.Tags[0] != "Big Boobs" {
		t.Errorf("Tags = %v", scene.Tags)
	}
	if scene.Thumbnail != "https://images.example.com/og_cover960.jpg" {
		t.Errorf("Thumbnail = %q (should be og:image)", scene.Thumbnail)
	}
	if scene.Date.Format("2006-01-02") != "2026-04-20" {
		t.Errorf("Date = %v", scene.Date)
	}
	if scene.Studio != "MomsTeachSex" {
		t.Errorf("Studio = %q", scene.Studio)
	}
}

func TestListScenes(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/video/gallery":
			_, _ = fmt.Fprint(w, listingHTML)
		default:
			_, _ = fmt.Fprint(w, detailHTML)
		}
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client()}
	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{Workers: 2})
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

	if len(titles) != 2 {
		t.Errorf("got %d scenes, want 2: %v", len(titles), titles)
	}
}

func TestListScenesKnownIDs(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/video/gallery":
			_, _ = fmt.Fprint(w, listingHTML)
		default:
			_, _ = fmt.Fprint(w, detailHTML)
		}
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client()}
	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{
		Workers:  1,
		KnownIDs: map[string]bool{"12344": true},
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
	if len(ids) != 1 || ids[0] != "12345" {
		t.Errorf("got ids %v, want [12345]", ids)
	}
}
