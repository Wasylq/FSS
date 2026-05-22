package bang

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/Wasylq/FSS/scraper"
)

const testPage = `<!DOCTYPE html>
<html>
<head>
<link rel="next" href="https://www.bang.com/originals/3366/bang-real-teens?by=date&page=2">
</head>
<body>
<p class="text-xxs md:text-xs text-foreground/70 mx-4" id="resultsCount">
	88 results
</p>
<div class="grid gap-2 grid-cols-2">
<div class="video_container flex flex-col flex-none w-full mb-2 align-top max-w-full">
    <a href="/video/aZS-pWVKkEBWCCzi/sakura-lins-bushy-pussy-gets-publicly-plowed"
       data-controller="videopreview" data-videopreview-id-value="6994bea5654a904056082ce2" data-videopreview-duration-value="3203">
        <div class="relative">
            <img data-videopreview-target="image"
                 src="https://i.bang.com/screenshots/60398/movie/1/2079055.jpg?w=240&h=150">
        </div>
        <span class="block text-xs lg:text-sm text-foreground font-semibold truncate mt-1 text-left">Sakura Lin&#039;s Bushy Pussy Gets Publicly Plowed</span>
    </a>
    <div class="grid grid-cols-1 text-card-foreground relative">
        <div class="text-xs">
            <svg class="h-3 h-3 fill-muted-foreground" viewBox="0 0 576 512"><path d="M288 32"/></svg>
                34K
            <span class="hidden xs:inline-block truncate">
                <span class="mx-1 lg:mx-2">•</span> May 12, 2026
            </span>
        </div>
        <div class="flex items-center flex-row flex-nowrap text-[13px] mt-1 truncate">
            <a class="scrollup text-ring font-medium capitalize hover:underline" href="/pornstar/aeD8Ssnu6lSQDi14/sakura-lin">Sakura Lin</a>
        </div>
    </div>
</div>
<div class="video_container flex flex-col flex-none w-full mb-2 align-top max-w-full">
    <a href="/video/aX02upuNVNzPC9wc/idaho-teen-aubry-babcock"
       data-controller="videopreview" data-videopreview-id-value="697d36ba9b8d54dccf0bdc1c" data-videopreview-duration-value="3068">
        <div class="relative">
            <img data-videopreview-target="image"
                 src="https://i.bang.com/screenshots/60126/movie/1/2075001.jpg?w=240&h=150">
        </div>
        <span class="block text-xs lg:text-sm text-foreground font-semibold truncate mt-1 text-left">Idaho Teen Aubry Babcock &amp; Friends</span>
    </a>
    <div class="grid grid-cols-1 text-card-foreground relative">
        <div class="text-xs">
            <svg class="h-3 h-3 fill-muted-foreground" viewBox="0 0 576 512"><path d="M288 32"/></svg>
                1.5K
            <span class="hidden xs:inline-block truncate">
                <span class="mx-1 lg:mx-2">•</span> Apr 28, 2026
            </span>
        </div>
        <div class="flex items-center flex-row flex-nowrap text-[13px] mt-1 truncate">
            <a class="scrollup text-ring font-medium capitalize hover:underline" href="/pornstar/abc123/aubry-babcock">Aubry Babcock</a>
            <a class="scrollup text-ring font-medium capitalize hover:underline" href="/pornstar/def456/alex-jett">Alex Jett</a>
        </div>
    </div>
</div>
</div>
</body></html>`

const testPageLast = `<!DOCTYPE html>
<html><head></head>
<body>
<p id="resultsCount">2 results</p>
<div class="grid">
<div class="video_container flex flex-col flex-none w-full mb-2 align-top max-w-full">
    <a href="/video/abc123/last-scene"
       data-controller="videopreview" data-videopreview-id-value="aaaaaaaaaaaaaaaaaaaaaaaa" data-videopreview-duration-value="1800">
        <div class="relative">
            <img data-videopreview-target="image" src="https://i.bang.com/screenshots/1/movie/1/1.jpg?w=240&h=150">
        </div>
        <span class="block text-xs lg:text-sm text-foreground font-semibold truncate mt-1 text-left">Last Scene</span>
    </a>
    <div class="grid grid-cols-1 text-card-foreground relative">
        <div class="text-xs">
            <svg class="h-3 h-3 fill-muted-foreground" viewBox="0 0 576 512"><path d="M288 32"/></svg>
                500
            <span class="hidden xs:inline-block truncate">
                <span class="mx-1 lg:mx-2">•</span> Jan 1, 2025
            </span>
        </div>
        <div class="flex items-center flex-row flex-nowrap text-[13px] mt-1 truncate">
            <a class="scrollup text-ring font-medium capitalize hover:underline" href="/pornstar/xyz/jane-doe">Jane Doe</a>
        </div>
    </div>
</div>
</div>
</body></html>`

func TestParsing(t *testing.T) {
	items := parseItems([]byte(testPage))
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	it := items[0]
	if it.id != "6994bea5654a904056082ce2" {
		t.Errorf("id = %q, want 6994bea5654a904056082ce2", it.id)
	}
	if it.duration != 3203 {
		t.Errorf("duration = %d, want 3203", it.duration)
	}
	if it.title != "Sakura Lin's Bushy Pussy Gets Publicly Plowed" {
		t.Errorf("title = %q", it.title)
	}
	if it.urlPath != "/video/aZS-pWVKkEBWCCzi/sakura-lins-bushy-pussy-gets-publicly-plowed" {
		t.Errorf("urlPath = %q", it.urlPath)
	}
	if it.thumbnail != "https://i.bang.com/screenshots/60398/movie/1/2079055.jpg?w=240&h=150" {
		t.Errorf("thumbnail = %q", it.thumbnail)
	}
	want := time.Date(2026, 5, 12, 0, 0, 0, 0, time.UTC)
	if !it.date.Equal(want) {
		t.Errorf("date = %v, want %v", it.date, want)
	}
	if it.views != 34000 {
		t.Errorf("views = %d, want 34000", it.views)
	}
	if len(it.performers) != 1 || it.performers[0] != "Sakura Lin" {
		t.Errorf("performers = %v", it.performers)
	}

	it2 := items[1]
	if it2.id != "697d36ba9b8d54dccf0bdc1c" {
		t.Errorf("item2 id = %q", it2.id)
	}
	if it2.title != "Idaho Teen Aubry Babcock & Friends" {
		t.Errorf("item2 title = %q", it2.title)
	}
	if it2.views != 1500 {
		t.Errorf("item2 views = %d, want 1500", it2.views)
	}
	if len(it2.performers) != 2 {
		t.Errorf("item2 performers = %v, want 2", it2.performers)
	}
	if len(it2.performers) == 2 && it2.performers[1] != "Alex Jett" {
		t.Errorf("item2 performers[1] = %q", it2.performers[1])
	}
}

func TestTotalAndNextPage(t *testing.T) {
	body := []byte(testPage)
	if m := totalRe.FindSubmatch(body); m == nil {
		t.Error("totalRe did not match")
	} else if string(m[1]) != "88" {
		t.Errorf("total = %q, want 88", string(m[1]))
	}
	if !nextRe.Match(body) {
		t.Error("nextRe should match page with link rel=next")
	}
	if nextRe.Match([]byte(testPageLast)) {
		t.Error("nextRe should NOT match last page")
	}
}

func TestPagination(t *testing.T) {
	page1Called := false
	page2Called := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("page") {
		case "1":
			page1Called = true
			_, _ = fmt.Fprint(w, testPage)
		case "2":
			page2Called = true
			_, _ = fmt.Fprint(w, testPageLast)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client()}
	studioURL := ts.URL + "/originals/3366/bang-real-teens"
	ch, err := s.ListScenes(context.Background(), studioURL, scraper.ListOpts{})
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
	if len(scenes) != 3 {
		t.Errorf("got %d scenes, want 3", len(scenes))
	}
}

func TestEarlyStop(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, testPage)
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client()}
	studioURL := ts.URL + "/originals/3366/bang-real-teens"
	known := map[string]bool{"697d36ba9b8d54dccf0bdc1c": true}
	ch, err := s.ListScenes(context.Background(), studioURL, scraper.ListOpts{KnownIDs: known})
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
		t.Errorf("got %d scenes, want 1 (before known ID)", len(scenes))
	}
}

func TestMatchesURL(t *testing.T) {
	s := New()
	tests := []struct {
		url  string
		want bool
	}{
		{"https://www.bang.com/originals/3366/bang-real-teens", true},
		{"https://www.bang.com/original/4308/bang-confessions", true},
		{"https://www.bang.com/studio/299/bang-originals", true},
		{"https://www.bang.com/studio/412/lead-porn/movies", true},
		{"https://www.bang.com/pornstar/aeD8Ssnu6lSQDi14/sakura-lin", true},
		{"https://www.bang.com/videos?in=bang!%20real%20teens", true},
		{"https://www.bang.com/videos?from=bang!%20europe", true},
		{"https://bang.com/originals/5000/bang-surprise", true},
		{"https://www.bang.com/video/aZS-pWVKkEBWCCzi/some-scene", false},
		{"https://example.com/originals/1234/test", false},
		{"https://www.pornhub.com/channels/bang", false},
	}
	for _, tc := range tests {
		if got := s.MatchesURL(tc.url); got != tc.want {
			t.Errorf("MatchesURL(%q) = %v, want %v", tc.url, got, tc.want)
		}
	}
}

func TestBuildPageURL(t *testing.T) {
	tests := []struct {
		studioURL string
		mode      urlMode
		page      int
		wantPath  string
		wantQuery string
	}{
		{
			"https://www.bang.com/originals/3366/bang-real-teens",
			modeOriginals, 2,
			"/originals/3366/bang-real-teens", "by=date&page=2",
		},
		{
			"https://www.bang.com/studio/412/lead-porn/movies",
			modeStudio, 1,
			"/studio/412/lead-porn", "by=date&page=1",
		},
		{
			"https://www.bang.com/pornstar/aeD8Ssnu6lSQDi14/sakura-lin",
			modePornstar, 3,
			"/pornstar/aeD8Ssnu6lSQDi14/sakura-lin", "by=date&page=3",
		},
	}
	for _, tc := range tests {
		got := buildPageURL(tc.studioURL, tc.mode, tc.page)
		u := mustParseURL(got)
		if u.Path != tc.wantPath {
			t.Errorf("buildPageURL(%q) path = %q, want %q", tc.studioURL, u.Path, tc.wantPath)
		}
		if u.RawQuery != tc.wantQuery {
			t.Errorf("buildPageURL(%q) query = %q, want %q", tc.studioURL, u.RawQuery, tc.wantQuery)
		}
	}
}

func TestDetectMode(t *testing.T) {
	tests := []struct {
		url     string
		want    urlMode
		wantErr bool
	}{
		{"https://www.bang.com/originals/3366/bang-real-teens", modeOriginals, false},
		{"https://www.bang.com/original/4308/bang-confessions", modeOriginals, false},
		{"https://www.bang.com/studio/299/bang-originals", modeStudio, false},
		{"https://www.bang.com/pornstar/abc/name", modePornstar, false},
		{"https://www.bang.com/videos?in=bang!+real+teens", modeVideos, false},
		{"https://www.bang.com/videos?from=bang!+europe", modeVideos, false},
		{"https://www.bang.com/originals", 0, true},
		{"https://www.bang.com/videos", 0, true},
	}
	for _, tc := range tests {
		got, err := detectMode(tc.url)
		if (err != nil) != tc.wantErr {
			t.Errorf("detectMode(%q) err = %v, wantErr = %v", tc.url, err, tc.wantErr)
			continue
		}
		if err == nil && got != tc.want {
			t.Errorf("detectMode(%q) = %v, want %v", tc.url, got, tc.want)
		}
	}
}

func TestParseViews(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"34K", 34000},
		{"1.5K", 1500},
		{"500", 500},
		{"0", 0},
	}
	for _, tc := range tests {
		if got := parseViews(tc.input); got != tc.want {
			t.Errorf("parseViews(%q) = %d, want %d", tc.input, got, tc.want)
		}
	}
}

func mustParseURL(raw string) *url.URL {
	u, err := url.Parse(raw)
	if err != nil {
		panic(err)
	}
	return u
}
