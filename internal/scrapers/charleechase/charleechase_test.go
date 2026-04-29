package charleechase

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

const listingFixture = `<html><body>
<div class="videoarea clear">
    <h3><a href="videos/39046/feet-relaxation-with-lotion">Feet Relaxation With Lotion</a></h3>
    <p class="date">April 3rd 2026</p>
    <div class="videos clear">
        <div class="video_pic"><a href="videos/39046/feet-relaxation-with-lotion"><img src="sd3.php?show=file&path=/videos/38846/thumb_1.jpg" alt=""></a></div>
    </div>
    <div class="video_details clear">
        <h4><a href="videos/39046/feet-relaxation-with-lotion">Section: FootFetish Videos</a></h4>
        <h5>Featuring: Charlee Chase</h5>
        <p>Great foot fetish video.<br>
<span onclick='addtocart(38846,"v")' style="cursor: pointer;">Download this clip for $9.95 </span></p>
        <div class="video_download"><a href="join.html">Download</a></div>
    </div>
</div> </div>

<div class="videoarea clear">
    <h3><a href="videos/38913/squirt-fest">Squirt Fest!</a></h3>
    <p class="date">January 23rd 2026</p>
    <div class="videos clear">
        <div class="video_pic"><a href="videos/38913/squirt-fest"><img src="sd3.php?show=file&path=/videos/38713/thumb_1.jpg" alt=""></a></div>
    </div>
    <div class="video_details clear">
        <h4><a href="videos/38913/squirt-fest">Section: Masturbation/Toys Videos</a></h4>
        <h5>Featuring: Charlee Chase</h5>
        <p>Hot solo video.<br>
<span onclick='addtocart(38713,"v")' style="cursor: pointer;">Download this clip for $9.95 </span></p>
        <div class="video_download"><a href="join.html">Download</a></div>
    </div>
</div> </div>
</body></html>`

const detailFixture = `<html><body>
<div class="customcontent">
<h1 class="customhcolor">Feet Relaxation With Lotion</h1>
<h3 class="customhcolor">Charlee Chase</h3>
<h4 class="customhcolor">bare feet,barefoot,Cougar,Feet,Fetish</h4>
</div>
<div class="date-and-covers">
    <div class="date">April 3rd 2026</div>
    <div class="">video duration <strong>00:08:26</strong></div>
    <div class="">Price: $9.95</div>
    <div class=""><strong>1920 x 1080</strong> FULL HD</div>
</div>
</body></html>`

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url   string
		match bool
	}{
		{"https://charleechaselive.com/sd3.php?show=recent_video_updates", true},
		{"https://charleechaselive.com/videos", true},
		{"https://charleechaselive.com/videos/", true},
		{"https://charleechaselive.com/videos/page/3", true},
		{"https://www.charleechaselive.com/videos", true},
		{"https://charleechaselive.com/", true},
		{"https://charleechaselive.com", true},
		{"https://charleechaselive.com/videos/39046/feet-relaxation", false},
		{"https://example.com/videos", false},
		{"", false},
	}
	for _, c := range cases {
		got := s.MatchesURL(c.url)
		if got != c.match {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.match)
		}
	}
}

func TestParseListItems(t *testing.T) {
	items := parseListItems([]byte(listingFixture))
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}

	item := items[0]
	if item.id != "39046" {
		t.Errorf("id = %q, want %q", item.id, "39046")
	}
	if item.title != "Feet Relaxation With Lotion" {
		t.Errorf("title = %q", item.title)
	}
	if item.href != "videos/39046/feet-relaxation-with-lotion" {
		t.Errorf("href = %q", item.href)
	}
	wantDate := time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC)
	if !item.date.Equal(wantDate) {
		t.Errorf("date = %v, want %v", item.date, wantDate)
	}
	if !strings.Contains(item.thumbnail, "thumb_1.jpg") {
		t.Errorf("thumbnail = %q, want thumb_1.jpg", item.thumbnail)
	}
	if item.category != "FootFetish Videos" {
		t.Errorf("category = %q", item.category)
	}
	if len(item.performers) != 1 || item.performers[0] != "Charlee Chase" {
		t.Errorf("performers = %v", item.performers)
	}
	if item.description != "Great foot fetish video." {
		t.Errorf("description = %q", item.description)
	}
	if item.price != 9.95 {
		t.Errorf("price = %f, want 9.95", item.price)
	}

	item2 := items[1]
	if item2.id != "38913" {
		t.Errorf("item2 id = %q, want %q", item2.id, "38913")
	}
	if item2.title != "Squirt Fest!" {
		t.Errorf("item2 title = %q", item2.title)
	}
}

func TestParseDetail(t *testing.T) {
	d := parseDetail([]byte(detailFixture))
	if d.duration != 506 {
		t.Errorf("duration = %d, want 506 (00:08:26)", d.duration)
	}
	wantTags := []string{"bare feet", "barefoot", "Cougar", "Feet", "Fetish"}
	if len(d.tags) != len(wantTags) {
		t.Fatalf("tags = %v, want %v", d.tags, wantTags)
	}
	for i, tag := range wantTags {
		if d.tags[i] != tag {
			t.Errorf("tags[%d] = %q, want %q", i, d.tags[i], tag)
		}
	}
}

func TestParseDate(t *testing.T) {
	cases := []struct {
		input string
		want  time.Time
	}{
		{"April 3rd 2026", time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC)},
		{"January 23rd 2026", time.Date(2026, 1, 23, 0, 0, 0, 0, time.UTC)},
		{"November 15th 2024", time.Date(2024, 11, 15, 0, 0, 0, 0, time.UTC)},
		{"August 23rd 2024", time.Date(2024, 8, 23, 0, 0, 0, 0, time.UTC)},
		{"May 17th 2024", time.Date(2024, 5, 17, 0, 0, 0, 0, time.UTC)},
		{"", time.Time{}},
	}
	for _, c := range cases {
		got := parseDate(c.input)
		if !got.Equal(c.want) {
			t.Errorf("parseDate(%q) = %v, want %v", c.input, got, c.want)
		}
	}
}

func TestParseDuration(t *testing.T) {
	cases := []struct {
		input string
		want  int
	}{
		{"00:08:26", 506},
		{"01:30:00", 5400},
		{"00:00:45", 45},
		{"", 0},
	}
	for _, c := range cases {
		got := parseDuration(c.input)
		if got != c.want {
			t.Errorf("parseDuration(%q) = %d, want %d", c.input, got, c.want)
		}
	}
}

func TestToScene(t *testing.T) {
	item := listItem{
		id:          "39046",
		href:        "videos/39046/feet-relaxation-with-lotion",
		title:       "Feet Relaxation With Lotion",
		date:        time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC),
		thumbnail:   "sd3.php?show=file&path=/videos/38846/thumb_1.jpg",
		category:    "FootFetish Videos",
		performers:  []string{"Charlee Chase"},
		description: "Great video.",
		price:       9.95,
	}
	detail := &detailData{
		tags:     []string{"bare feet", "Cougar"},
		duration: 506,
	}
	now := time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC)
	sc := toScene("https://charleechaselive.com/videos", item, detail, now)

	if sc.ID != "39046" {
		t.Errorf("ID = %q", sc.ID)
	}
	if sc.SiteID != "charleechase" {
		t.Errorf("SiteID = %q", sc.SiteID)
	}
	if sc.URL != "https://charleechaselive.com/videos/39046/feet-relaxation-with-lotion" {
		t.Errorf("URL = %q", sc.URL)
	}
	if sc.Duration != 506 {
		t.Errorf("Duration = %d", sc.Duration)
	}
	if len(sc.Tags) != 2 {
		t.Errorf("Tags = %v", sc.Tags)
	}
	if len(sc.PriceHistory) != 1 || sc.PriceHistory[0].Regular != 9.95 {
		t.Errorf("PriceHistory = %v", sc.PriceHistory)
	}
	if sc.Studio != "Charlee Chase" {
		t.Errorf("Studio = %q", sc.Studio)
	}
}

func TestListScenes(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/videos/page/1":
			_, _ = fmt.Fprint(w, listingFixture)
		case "/videos/39046/feet-relaxation-with-lotion":
			_, _ = fmt.Fprint(w, detailFixture)
		case "/videos/38913/squirt-fest":
			_, _ = fmt.Fprint(w, detailFixture)
		case "/videos/page/2":
			_, _ = fmt.Fprint(w, "<html><body></body></html>")
		default:
			_, _ = fmt.Fprint(w, "<html><body></body></html>")
		}
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client(), base: ts.URL}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/videos", scraper.ListOpts{})
	if err != nil {
		t.Fatalf("ListScenes error: %v", err)
	}

	var scenes []scraper.SceneResult
	for r := range ch {
		if r.Err != nil {
			t.Errorf("unexpected error: %v", r.Err)
			continue
		}
		scenes = append(scenes, r)
	}

	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2", len(scenes))
	}
	if scenes[0].Scene.ID != "39046" {
		t.Errorf("scene[0].ID = %q, want %q", scenes[0].Scene.ID, "39046")
	}
	if scenes[1].Scene.ID != "38913" {
		t.Errorf("scene[1].ID = %q, want %q", scenes[1].Scene.ID, "38913")
	}
}

func TestListScenesKnownIDs(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/videos/page/1":
			_, _ = fmt.Fprint(w, listingFixture)
		default:
			_, _ = fmt.Fprint(w, "<html><body></body></html>")
		}
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client(), base: ts.URL}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/videos", scraper.ListOpts{
		KnownIDs: map[string]bool{"38913": true},
	})
	if err != nil {
		t.Fatalf("ListScenes error: %v", err)
	}

	var scenes []scraper.SceneResult
	sawStoppedEarly := false
	for r := range ch {
		switch r.Kind {
		case scraper.KindStoppedEarly:
			sawStoppedEarly = true
		case scraper.KindScene:
			scenes = append(scenes, r)
		case scraper.KindError:
			t.Errorf("unexpected error: %v", r.Err)
		}
	}

	if len(scenes) != 1 {
		t.Errorf("got %d scenes, want 1 (early stop at known ID)", len(scenes))
	}
	if !sawStoppedEarly {
		t.Error("expected StoppedEarly signal, got none")
	}
}

func TestMultiplePerformers(t *testing.T) {
	listing := `<html><body>
<div class="videoarea clear">
    <h3><a href="videos/37757/compilation">Compilation!</a></h3>
    <p class="date">August 23rd 2024</p>
    <div class="videos clear">
        <div class="video_pic"><a href="videos/37757/compilation"><img src="thumb_1.jpg" alt=""></a></div>
    </div>
    <div class="video_details clear">
        <h4><a href="videos/37757/compilation">Section: Boy/Girl Videos</a></h4>
        <h5>Featuring: Brandon, Dakota, Charlee Chase</h5>
        <p>Great compilation.<br>
<span onclick='addtocart(37558,"v")'>Download this clip for $9.95 </span></p>
        <div class="video_download"><a href="join.html">Download</a></div>
    </div>
</div> </div>
</body></html>`

	items := parseListItems([]byte(listing))
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	want := []string{"Brandon", "Dakota", "Charlee Chase"}
	if len(items[0].performers) != 3 {
		t.Fatalf("performers = %v, want %v", items[0].performers, want)
	}
	for i, p := range want {
		if items[0].performers[i] != p {
			t.Errorf("performers[%d] = %q, want %q", i, items[0].performers[i], p)
		}
	}
}
