package sexmexutil

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

	"github.com/Wasylq/FSS/scraper"
)

func testScraper(siteBase string) *Scraper {
	return &Scraper{
		Cfg: SiteConfig{
			ID:       "sexmex",
			Studio:   "SexMex",
			SiteBase: siteBase,
			MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?sexmex\.xxx(/|$)`),
		},
	}
}

func TestMatchesURL(t *testing.T) {
	s := testScraper("https://sexmex.xxx")
	cases := []struct {
		url  string
		want bool
	}{
		{"https://sexmex.xxx", true},
		{"https://sexmex.xxx/", true},
		{"https://sexmex.xxx/tour/updates", true},
		{"https://sexmex.xxx/tour/models/NickyFerrari.html", true},
		{"https://sexmex.xxx/tour/categories/milf.html", true},
		{"https://www.sexmex.xxx/tour/categories/movies.html", true},
		{"https://example.com/tour/updates", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.want {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.want)
		}
	}
}

func TestResolveListingSlug(t *testing.T) {
	cases := []struct {
		url, want string
	}{
		{"https://sexmex.xxx", "categories/movies"},
		{"https://sexmex.xxx/", "categories/movies"},
		{"https://sexmex.xxx/tour/updates", "categories/movies"},
		{"https://sexmex.xxx/tour/categories/movies.html", "categories/movies"},
		{"https://sexmex.xxx/tour/categories/milf.html", "categories/milf"},
		{"https://sexmex.xxx/tour/models/NickyFerrari.html", "models/NickyFerrari"},
		{"https://sexmex.xxx/tour/categories/stepmom-therapy.html", "categories/stepmom-therapy"},
	}
	for _, c := range cases {
		if got := resolveListingSlug(c.url, "https://sexmex.xxx"); got != c.want {
			t.Errorf("resolveListingSlug(%q) = %q, want %q", c.url, got, c.want)
		}
	}
}

func TestPageURL(t *testing.T) {
	cases := []struct {
		slug string
		page int
		want string
	}{
		{"categories/movies", 1, "https://sexmex.xxx/tour/categories/movies.html"},
		{"categories/movies", 2, "https://sexmex.xxx/tour/categories/movies_2_d.html"},
		{"categories/milf", 5, "https://sexmex.xxx/tour/categories/milf_5_d.html"},
		{"models/NickyFerrari", 1, "https://sexmex.xxx/tour/models/NickyFerrari.html"},
		{"models/NickyFerrari", 3, "https://sexmex.xxx/tour/models/NickyFerrari_3_d.html"},
	}
	for _, c := range cases {
		if got := pageURL("https://sexmex.xxx", c.slug, c.page); got != c.want {
			t.Errorf("pageURL(%q, %d) = %q, want %q", c.slug, c.page, got, c.want)
		}
	}
}

const fixtureCard = `
    <div class="col-lg-3 col-md-3 col-xs-16 thumb clearfix" data-setid="3112">
    <div class="videothumbnail">
        <a title="MY BEST FRIEND&rsquo;S STEPMOM PART 2 . Nicky Ferrari" href="https://sexmex.xxx/tour/updates/MY-BEST-FRIENDS-STEPMOM-PART-2-Nicky-Ferrari.html">
        <div id="setimage_3112" class="update_thumb thumbs" style="position:relative;">
            <img loading="lazy" class="img-fluid w-100" alt="scene" src="https://c7711000c7.sexmex-cdn.com/tour/content/contentthumbs/82/45/118245-1x.jpg?expires=123&l=47&token=abc">
        </div>
        </a>
        <div><h5 class="scene-title h-n_mrgn_btm"><a class="modelnamesut" style="color: rgb(251, 202, 39)!important;" title="MY BEST FRIEND&rsquo;S STEPMOM PART 2 . Nicky Ferrari" href="https://sexmex.xxx/tour/updates/MY-BEST-FRIENDS-STEPMOM-PART-2-Nicky-Ferrari.html">MY BEST FRIEND'S STEPMOM PART 2 . Nicky Ferrari</a></h5>
        <p class="scene-descr" style="height: 27px;">He got jealous and went to confront her.
...</p>
        <p>
            <a class="modelnamesut" style="color:rgb(251, 202, 39)!important;" href="https://sexmex.xxx/tour/models/NickyFerrari.html">Nicky Ferrari</a>
        </p>
        <p class="scene-date" style="">04/23/2026</p>
    </div>
</div>
</a></div>`

func TestParseCards(t *testing.T) {
	cards := parseCards([]byte(fixtureCard))
	if len(cards) != 1 {
		t.Fatalf("got %d cards, want 1", len(cards))
	}
	c := cards[0]

	if c.id != "3112" {
		t.Errorf("id = %q", c.id)
	}
	if c.title != "MY BEST FRIEND’S STEPMOM PART 2 . Nicky Ferrari" {
		t.Errorf("title = %q", c.title)
	}
	if c.url != "https://sexmex.xxx/tour/updates/MY-BEST-FRIENDS-STEPMOM-PART-2-Nicky-Ferrari.html" {
		t.Errorf("url = %q", c.url)
	}
	if c.thumbnail == "" {
		t.Error("thumbnail is empty")
	}
	if c.description != "He got jealous and went to confront her." {
		t.Errorf("description = %q", c.description)
	}
	if len(c.performers) != 1 || c.performers[0] != "Nicky Ferrari" {
		t.Errorf("performers = %v", c.performers)
	}
	if c.date.Year() != 2026 || c.date.Month() != 4 || c.date.Day() != 23 {
		t.Errorf("date = %v", c.date)
	}
}

func TestCardToScene_TitleStripping(t *testing.T) {
	s := testScraper("https://sexmex.xxx")
	c := card{
		id:    "1",
		title: "MY BEST FRIEND’S STEPMOM PART 2 . Nicky Ferrari",
		url:   "https://sexmex.xxx/tour/updates/scene.html",
	}
	scene := s.cardToScene("https://sexmex.xxx/tour/updates", c, fixedTime())
	if scene.Title != "MY BEST FRIEND’S STEPMOM PART 2" {
		t.Errorf("Title = %q", scene.Title)
	}
	if scene.Studio != "SexMex" {
		t.Errorf("Studio = %q", scene.Studio)
	}
}

func TestListScenes(t *testing.T) {
	page1HTML := `<html><body>
    <div class="col-lg-3 col-md-3 col-xs-16 thumb clearfix" data-setid="100">
    <div class="videothumbnail">
        <a title="Scene One . Model A" href="%s/tour/updates/scene-one.html">
        <div class="update_thumb thumbs"><img src="https://c7711000c7.sexmex-cdn.com/tour/content/contentthumbs/1/1/1-1x.jpg"></div>
        </a>
        <div><h5 class="scene-title h-n_mrgn_btm"><a class="modelnamesut" title="Scene One . Model A" href="%s/tour/updates/scene-one.html">Scene One . Model A</a></h5>
        <p class="scene-descr">A description.</p>
        <p><a class="modelnamesut" href="%s/tour/models/ModelA.html">Model A</a></p>
        <p class="scene-date">04/20/2026</p>
    </div></div>
</a></div>
</body></html>`

	var ts *httptest.Server
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/tour/categories/movies.html":
			_, _ = fmt.Fprintf(w, page1HTML, ts.URL, ts.URL, ts.URL)
		default:
			_, _ = w.Write([]byte(`<html><body></body></html>`))
		}
	}))
	defer ts.Close()

	s := &Scraper{
		Cfg: SiteConfig{
			ID:       "sexmex",
			Studio:   "SexMex",
			SiteBase: ts.URL,
			MatchRe:  regexp.MustCompile(`.*`),
		},
		Client: ts.Client(),
	}

	ch, err := s.ListScenes(context.Background(), ts.URL+"/tour/updates", scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}

	var scenes []string
	for r := range ch {
		if r.Kind == scraper.KindStoppedEarly {
			continue
		}
		if r.Err != nil {
			t.Errorf("unexpected error: %v", r.Err)
			continue
		}
		scenes = append(scenes, r.Scene.Title)
		if r.Scene.Performers[0] != "Model A" {
			t.Errorf("Performers = %v", r.Scene.Performers)
		}
	}

	if len(scenes) != 1 || scenes[0] != "Scene One" {
		t.Errorf("scenes = %v, want [Scene One]", scenes)
	}
}

func TestListScenesKnownIDs(t *testing.T) {
	page1HTML := `<html><body>
    <div class="col-lg-3 col-md-3 col-xs-16 thumb clearfix" data-setid="200">
    <div class="videothumbnail">
        <a title="New Scene" href="%s/tour/updates/new.html">
        <div class="update_thumb thumbs"><img src="https://c7711000c7.sexmex-cdn.com/tour/content/contentthumbs/1/1/1-1x.jpg"></div>
        </a>
        <div><h5 class="scene-title h-n_mrgn_btm"><a class="modelnamesut" title="New Scene" href="%s/tour/updates/new.html">New Scene</a></h5>
        <p class="scene-descr">Desc.</p>
        <p><a class="modelnamesut" href="%s/tour/models/X.html">X</a></p>
        <p class="scene-date">04/20/2026</p>
    </div></div>
</a></div>
    <div class="col-lg-3 col-md-3 col-xs-16 thumb clearfix" data-setid="199">
    <div class="videothumbnail">
        <a title="Known Scene" href="%s/tour/updates/known.html">
        <div class="update_thumb thumbs"><img src="https://c7711000c7.sexmex-cdn.com/tour/content/contentthumbs/1/1/1-1x.jpg"></div>
        </a>
        <div><h5 class="scene-title h-n_mrgn_btm"><a class="modelnamesut" title="Known Scene" href="%s/tour/updates/known.html">Known Scene</a></h5>
        <p class="scene-descr">Desc.</p>
        <p><a class="modelnamesut" href="%s/tour/models/X.html">X</a></p>
        <p class="scene-date">04/19/2026</p>
    </div></div>
</a></div>
</body></html>`

	var ts *httptest.Server
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, page1HTML, ts.URL, ts.URL, ts.URL, ts.URL, ts.URL, ts.URL)
	}))
	defer ts.Close()

	s := &Scraper{
		Cfg: SiteConfig{
			ID: "sexmex", Studio: "SexMex", SiteBase: ts.URL,
			MatchRe: regexp.MustCompile(`.*`),
		},
		Client: ts.Client(),
	}

	ch, err := s.ListScenes(context.Background(), ts.URL+"/tour/updates", scraper.ListOpts{
		KnownIDs: map[string]bool{"199": true},
	})
	if err != nil {
		t.Fatal(err)
	}

	var titles []string
	stoppedEarly := false
	for r := range ch {
		if r.Kind == scraper.KindStoppedEarly {
			stoppedEarly = true
			continue
		}
		if r.Err != nil {
			t.Errorf("error: %v", r.Err)
			continue
		}
		titles = append(titles, r.Scene.Title)
	}

	if !stoppedEarly {
		t.Error("expected StoppedEarly")
	}
	if len(titles) != 1 || titles[0] != "New Scene" {
		t.Errorf("titles = %v, want [New Scene]", titles)
	}
}

func fixedTime() time.Time {
	return time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
}
