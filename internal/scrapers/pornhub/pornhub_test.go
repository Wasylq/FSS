package pornhub

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wasylq/FSS/scraper"
)

// ---- fixtures ----

type testItem struct {
	vkey       string
	title      string
	thumbURL   string
	durStr     string
	studioSlug string
	studioName string
}

func videoListHTML(items []testItem) []byte {
	var sb strings.Builder
	sb.WriteString("<html><body><ul>")
	for _, item := range items {
		fmt.Fprintf(&sb, `
<li class="pcVideoListItem js-pop videoblock videoBox" id="v%s"
    data-video-id="%s" data-video-vkey="%s">
    <div class="wrap">
        <div class="phimage">
            <a href="/view_video.php?viewkey=%s" title="%s" class="fade">
                <img src="%s" alt="%s" loading="lazy">
                <div class="marker-overlays js-noFade">
                    <var class="duration">%s</var>
                </div>
            </a>
        </div>
        <div class="thumbnail-info-wrapper clearfix">
            <div class="thumbnail-info">
                <div class="videoUploaderBlock clearfix">
                    <div class="usernameWrap">
                        <a href="/pornstar/%s">%s</a>
                    </div>
                </div>
                <var class="added">2 years ago</var>
            </div>
        </div>
    </div>
</li>`, item.vkey, item.vkey, item.vkey,
			item.vkey, item.title,
			item.thumbURL, item.title,
			item.durStr,
			item.studioSlug, item.studioName)
	}
	sb.WriteString("</ul></body></html>")
	return []byte(sb.String())
}

func testItem1() testItem {
	return testItem{
		vkey:       "aabbcc112233",
		title:      "Scene One",
		thumbURL:   "https://ei.phncdn.com/videos/202305/16/431685661/original/(m=eafTGgaaaa)11.jpg",
		durStr:     "20:47",
		studioSlug: "dee-williams",
		studioName: "Dee Williams",
	}
}

func testItem2() testItem {
	return testItem{
		vkey:       "ddeeff445566",
		title:      "Scene Two &amp; More",
		thumbURL:   "https://ei.phncdn.com/videos/202301/10/123456789/original/(m=eafTGgaaaa)11.jpg",
		durStr:     "10:30",
		studioSlug: "dee-williams",
		studioName: "Dee Williams",
	}
}

func testItem3() testItem {
	return testItem{
		vkey:       "112233aabbcc",
		title:      "Scene Three",
		thumbURL:   "https://ei.phncdn.com/videos/202212/01/987654321/original/(m=eafTGgaaaa)11.jpg",
		durStr:     "1:05:00",
		studioSlug: "dee-williams",
		studioName: "Dee Williams",
	}
}

// ---- TestMatchesURL ----

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url   string
		match bool
	}{
		{"https://www.pornhub.com/pornstar/dee-williams", true},
		{"https://pornhub.com/pornstar/bettie-bondage", true},
		{"https://www.pornhub.com/channels/mylf", true},
		{"https://www.pornhub.com/pornstar/dee-williams/videos", true},
		{"https://www.pornhub.com/view_video.php?viewkey=abc123", false},
		{"https://www.manyvids.com/pornstar/someone", false},
		{"", false},
	}
	for _, c := range cases {
		got := s.MatchesURL(c.url)
		if got != c.match {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.match)
		}
	}
}

// ---- TestBuildPageURL ----

func TestBuildPageURL(t *testing.T) {
	cases := []struct {
		input   string
		page    int
		want    string
		wantErr bool
	}{
		{
			"https://www.pornhub.com/pornstar/dee-williams",
			1,
			"https://www.pornhub.com/pornstar/dee-williams/videos?page=1",
			false,
		},
		{
			"https://www.pornhub.com/channels/mylf",
			3,
			"https://www.pornhub.com/channels/mylf/videos?page=3",
			false,
		},
		{
			"https://www.pornhub.com/pornstar/dee-williams/videos",
			2,
			"https://www.pornhub.com/pornstar/dee-williams/videos?page=2",
			false,
		},
		{
			"https://www.manyvids.com/Profile/123",
			1,
			"",
			true,
		},
	}
	for _, c := range cases {
		got, err := buildPageURL(c.input, c.page)
		if (err != nil) != c.wantErr {
			t.Errorf("buildPageURL(%q, %d) error = %v, wantErr %v", c.input, c.page, err, c.wantErr)
			continue
		}
		if got != c.want {
			t.Errorf("buildPageURL(%q, %d) = %q, want %q", c.input, c.page, got, c.want)
		}
	}
}

// ---- TestParseDuration ----

func TestParseDuration(t *testing.T) {
	cases := []struct {
		input string
		want  int
	}{
		{"20:47", 1247},
		{"10:30", 630},
		{"1:05:00", 3900},
		{"00:45", 45},
		{"", 0},
	}
	for _, c := range cases {
		got := parseDuration(c.input)
		if got != c.want {
			t.Errorf("parseDuration(%q) = %d, want %d", c.input, got, c.want)
		}
	}
}

// ---- TestParseItem ----

func TestParseItem(t *testing.T) {
	item := testItem1()
	body := videoListHTML([]testItem{item})
	items := parseItems(body)
	if len(items) != 1 {
		t.Fatalf("parseItems returned %d items, want 1", len(items))
	}
	got := items[0]

	if got.vkey != "aabbcc112233" {
		t.Errorf("vkey = %q, want %q", got.vkey, "aabbcc112233")
	}
	if got.title != "Scene One" {
		t.Errorf("title = %q, want %q", got.title, "Scene One")
	}
	if got.thumbnail != item.thumbURL {
		t.Errorf("thumbnail = %q, want %q", got.thumbnail, item.thumbURL)
	}
	if got.duration != 1247 {
		t.Errorf("duration = %d, want 1247 (20:47)", got.duration)
	}
	wantDate := time.Date(2023, 5, 16, 0, 0, 0, 0, time.UTC)
	if !got.date.Equal(wantDate) {
		t.Errorf("date = %v, want %v", got.date, wantDate)
	}
	if got.studio != "Dee Williams" {
		t.Errorf("studio = %q, want %q", got.studio, "Dee Williams")
	}
}

func TestParseItemHTMLEntities(t *testing.T) {
	item := testItem2()
	body := videoListHTML([]testItem{item})
	items := parseItems(body)
	if len(items) != 1 {
		t.Fatalf("parseItems returned %d items, want 1", len(items))
	}
	if items[0].title != "Scene Two & More" {
		t.Errorf("title = %q, want %q", items[0].title, "Scene Two & More")
	}
}

func TestParseItemHourDuration(t *testing.T) {
	item := testItem3()
	body := videoListHTML([]testItem{item})
	items := parseItems(body)
	if len(items) != 1 {
		t.Fatalf("parseItems returned %d items, want 1", len(items))
	}
	if items[0].duration != 3900 {
		t.Errorf("duration = %d, want 3900 (1:05:00)", items[0].duration)
	}
}

// ---- TestToScene ----

func TestToScene(t *testing.T) {
	item := phItem{
		vkey:      "aabbcc112233",
		title:     "Scene One",
		thumbnail: "https://ei.phncdn.com/videos/202305/16/431685661/original/(m=eafTGgaaaa)11.jpg",
		duration:  1247,
		date:      time.Date(2023, 5, 16, 0, 0, 0, 0, time.UTC),
		studio:    "Dee Williams",
	}
	now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
	scene := toScene("https://www.pornhub.com/pornstar/dee-williams", item, now)

	if scene.ID != "aabbcc112233" {
		t.Errorf("ID = %q", scene.ID)
	}
	if scene.SiteID != "pornhub" {
		t.Errorf("SiteID = %q", scene.SiteID)
	}
	if scene.URL != "https://www.pornhub.com/view_video.php?viewkey=aabbcc112233" {
		t.Errorf("URL = %q", scene.URL)
	}
	if scene.Duration != 1247 {
		t.Errorf("Duration = %d, want 1247", scene.Duration)
	}
	if len(scene.PriceHistory) != 1 || !scene.PriceHistory[0].IsFree {
		t.Errorf("PriceHistory = %v, want one free snapshot", scene.PriceHistory)
	}
}

// ---- TestListScenes (pornstar) ----

func TestListScenes(t *testing.T) {
	page1 := []testItem{testItem1(), testItem2()}
	page2 := []testItem{testItem3()}

	page := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page++
		switch page {
		case 1:
			_, _ = w.Write(videoListHTML(page1))
		case 2:
			_, _ = w.Write(videoListHTML(page2))
		default:
			_, _ = w.Write(videoListHTML(nil))
		}
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client()}
	studioURL := ts.URL + "/pornstar/dee-williams"
	ch, err := s.ListScenes(context.Background(), studioURL, scraper.ListOpts{})
	if err != nil {
		t.Fatalf("ListScenes error: %v", err)
	}

	got := map[string]string{}
	for r := range ch {
		if r.Err != nil {
			t.Errorf("unexpected error: %v", r.Err)
			continue
		}
		got[r.Scene.ID] = r.Scene.Title
	}

	if len(got) != 3 {
		t.Fatalf("got %d scenes, want 3: %v", len(got), got)
	}
	want := map[string]string{
		"aabbcc112233": "Scene One",
		"ddeeff445566": "Scene Two & More",
		"112233aabbcc": "Scene Three",
	}
	for id, title := range want {
		if got[id] != title {
			t.Errorf("scene %s title = %q, want %q", id, got[id], title)
		}
	}
}

// ---- TestListScenesChannel ----

func TestListScenesChannel(t *testing.T) {
	items := []testItem{testItem1(), testItem2()}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.RawQuery, "page=2") {
			_, _ = w.Write(videoListHTML(nil))
		} else {
			_, _ = w.Write(videoListHTML(items))
		}
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client()}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/channels/mylf", scraper.ListOpts{})
	if err != nil {
		t.Fatalf("ListScenes error: %v", err)
	}

	var count int
	for r := range ch {
		if r.Err != nil {
			t.Errorf("unexpected error: %v", r.Err)
		} else {
			count++
		}
	}
	if count != 2 {
		t.Errorf("got %d scenes, want 2", count)
	}
}

// ---- TestListScenesKnownIDs ----

func TestListScenesKnownIDs(t *testing.T) {
	items := []testItem{testItem1(), testItem2(), testItem3()}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(videoListHTML(items))
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client()}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/pornstar/dee-williams", scraper.ListOpts{
		KnownIDs: map[string]bool{"ddeeff445566": true},
	})
	if err != nil {
		t.Fatalf("ListScenes error: %v", err)
	}

	var scenes []scraper.SceneResult
	sawStoppedEarly := false
	for r := range ch {
		if r.StoppedEarly {
			sawStoppedEarly = true
			continue
		}
		if r.Err == nil {
			scenes = append(scenes, r)
		}
	}
	if len(scenes) != 1 {
		t.Errorf("got %d scenes, want 1 (early stop at known ID)", len(scenes))
	}
	if !sawStoppedEarly {
		t.Error("expected StoppedEarly signal, got none")
	}
	if len(scenes) > 0 && scenes[0].Scene.ID != "aabbcc112233" {
		t.Errorf("scene ID = %q, want %q", scenes[0].Scene.ID, "aabbcc112233")
	}
}
