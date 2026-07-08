package jayspov

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/scraper"
)

// card is a real captured scene-update tile (trimmed for length).
func card(id, vid, title, performer, date string) string {
	return fmt.Sprintf(`<div class="grid-item" id="ascene_%[1]s"><article class="scene-update silent-teaser"
	style="position: relative;"><div class="scene-img-wrapper"><div class="still-screen"><img class="img-full-fluid mx-auto"
		data-srcset="https://imgs1cdn.adultempire.com/bn/1000/%[2]s-custom-default.jpg 1000w"
		src="https://imgs1cdn.adultempire.com/bn/25/%[2]s-custom-default.jpg"
		data-src="https://imgs1cdn.adultempire.com/bn/600/%[2]s-custom-default.jpg"
		alt="Image of %[3]s" title="%[3]s" /></div></div><a class="scene-update-details text-white"
		href="/%[1]s/jays-pov-slug-streaming-scene-video.html"
		data-Category="Home" data-Label="Scene Update - Title" ><span><h5>
			%[4]s
		</h5></span><span class="date">
		%[5]s
	</span></a></article></div>`, id, vid, title, performer, date)
}

// banner is a promo tile with no scene link — must be skipped.
const banner = `<div class="grid-item" id="ascene_999999"><article class="scene-update">` +
	`<img alt="Image of Welcome to the official Jays POV Membership website!" title="Welcome" />` +
	`<a class="scene-update-details" href="/join">Join Now</a></article></div>`

const pagination = `<ul class="pagination"><li class="page-item active"><a href="?">1</a></li>` +
	`<li class="page-item"><a href="?page=2" title="Next">Next</a></li></ul>`

const detailPage = `<!DOCTYPE html><html><head><title>unrelated template title</title></head><body>
<div class="release-date"><span class="font-weight-bold mr-2">Released:</span>Jun 24, 2026 </div>
<div class="studio"><span class="font-weight-bold mr-2">Studio:</span><span> Jay&#39;s POV </span></div>
<div class="series"><span class="font-weight-bold mr-2">Series:</span><a href="/streaming-video-by-scene.html?series=63303" >Daddy Roleplay</a></div>
<div class="length"><span class="font-weight-bold mr-2">Length:</span>36 min </div>
<div class="tags"><span class="font-weight-bold mr-2">Tags:</span><a href="/join" >Family Roleplaying</a>, <a href="/join" >POV</a>, <a href="/join" >Creampie</a></div>
</body></html>`

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	page1 := "<html><body>" +
		banner +
		card("1791872", "5106201", "Petite Bubble Butt Cock Slut Chastity Doll Rides Step Daddy&#39;s Morning Wood", "Chastity Doll", "Jun 24, 2026") +
		card("1791433", "5105068", "New Girl Daisy Erotica Is A Slutty Little Tattooed Spinner With An Appetite For Cock", "Daisy Erotica", "Jun 17, 2026") +
		pagination + "</body></html>"
	// page 2 has one scene and no further pagination.
	page2 := "<html><body>" +
		card("1769078", "5099999", "Older Scene Title", "Mia River", "Jun 11, 2026") +
		"</body></html>"

	mux := http.NewServeMux()
	mux.HandleFunc("/jays-pov-updates.html", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Cookie") != ageCookie {
			http.Redirect(w, r, "/AgeConfirmation?url2=/", http.StatusFound)
			return
		}
		switch r.URL.Query().Get("page") {
		case "2":
			_, _ = fmt.Fprint(w, page2)
		case "":
			_, _ = fmt.Fprint(w, page1)
		default:
			_, _ = fmt.Fprint(w, "<html><body></body></html>")
		}
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "streaming-scene-video.html") {
			_, _ = fmt.Fprint(w, detailPage)
			return
		}
		http.NotFound(w, r)
	})
	return httptest.NewServer(mux)
}

func collect(t *testing.T, ts *httptest.Server, studioURL string) []models.Scene {
	t.Helper()
	s := New()
	s.client = ts.Client()
	s.baseURL = ts.URL

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ch, err := s.ListScenes(ctx, studioURL, scraper.ListOpts{Workers: 2})
	if err != nil {
		t.Fatalf("ListScenes: %v", err)
	}
	var scenes []models.Scene
	for res := range ch {
		switch res.Kind {
		case scraper.KindScene:
			scenes = append(scenes, res.Scene)
		case scraper.KindError:
			t.Errorf("scrape error: %v", res.Err)
		}
	}
	return scenes
}

func TestListScenes(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	scenes := collect(t, ts, ts.URL+"/jays-pov-updates.html")
	if len(scenes) != 3 {
		t.Fatalf("expected 3 scenes (banner skipped), got %d", len(scenes))
	}

	byID := map[string]models.Scene{}
	for _, sc := range scenes {
		byID[sc.ID] = sc
	}

	sc, ok := byID["1791872"]
	if !ok {
		t.Fatal("scene 1791872 missing")
	}
	if sc.Title != "Petite Bubble Butt Cock Slut Chastity Doll Rides Step Daddy's Morning Wood" {
		t.Errorf("title = %q", sc.Title)
	}
	if sc.SiteID != siteID || sc.Studio != studioName {
		t.Errorf("siteID=%q studio=%q", sc.SiteID, sc.Studio)
	}
	if len(sc.Performers) != 1 || sc.Performers[0] != "Chastity Doll" {
		t.Errorf("performers = %v", sc.Performers)
	}
	if !strings.HasSuffix(sc.URL, "/1791872/jays-pov-slug-streaming-scene-video.html") {
		t.Errorf("url = %q", sc.URL)
	}
	if sc.Thumbnail != "https://imgs1cdn.adultempire.com/bn/600/5106201-custom-default.jpg" {
		t.Errorf("thumbnail = %q", sc.Thumbnail)
	}
	// Date comes from the detail page (Jun 24, 2026).
	want := time.Date(2026, 6, 24, 0, 0, 0, 0, time.UTC)
	if !sc.Date.Equal(want) {
		t.Errorf("date = %v, want %v", sc.Date, want)
	}
	// Duration + series + tags come from the detail page.
	if sc.Duration != 36*60 {
		t.Errorf("duration = %d, want %d", sc.Duration, 36*60)
	}
	if sc.Series != "Daddy Roleplay" {
		t.Errorf("series = %q", sc.Series)
	}
	if len(sc.Tags) != 3 || sc.Tags[0] != "Family Roleplaying" {
		t.Errorf("tags = %v", sc.Tags)
	}
}

func TestKnownIDEarlyStop(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	s := New()
	s.client = ts.Client()
	s.baseURL = ts.URL

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ch, err := s.ListScenes(ctx, ts.URL+"/jays-pov-updates.html", scraper.ListOpts{
		Workers:  2,
		KnownIDs: map[string]bool{"1791433": true},
	})
	if err != nil {
		t.Fatalf("ListScenes: %v", err)
	}
	count, stopped := 0, false
	for res := range ch {
		switch res.Kind {
		case scraper.KindScene:
			count++
		case scraper.KindStoppedEarly:
			stopped = true
		}
	}
	if !stopped {
		t.Error("expected StoppedEarly signal")
	}
	// Only the first card (1791872) is enqueued before the known ID is hit.
	if count != 1 {
		t.Errorf("expected 1 scene before early stop, got %d", count)
	}
}

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url  string
		want bool
	}{
		{"https://www.jayspov.net/jays-pov-updates.html", true},
		{"https://jayspov.net/", true},
		{"http://www.jayspov.net/streaming-video-by-scene.html?series=63303", true},
		{"https://www.jayspov.net/1791872/jays-pov-slug-streaming-scene-video.html", true},
		{"https://www.example.com/", false},
		{"https://www.notjayspov.net/", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.want {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.want)
		}
	}
}
