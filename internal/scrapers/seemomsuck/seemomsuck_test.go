package seemomsuck

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

func makeArticle(slug, title, performer, desc, thumb string) string {
	perfHTML := ""
	if performer != "" {
		var links []string
		for _, p := range strings.Split(performer, ", ") {
			pslug := strings.ToLower(strings.ReplaceAll(p, " ", "-"))
			links = append(links, fmt.Sprintf(`<a href="/models/%s.html?nats=abc">%s</a>`, pslug, p))
		}
		perfHTML = fmt.Sprintf(`<div class="name-date"><div class="model-name">%s</div></div>`, strings.Join(links, " &amp; "))
	}
	return fmt.Sprintf(`<article class="content-list__item content-list__item--3 dfc__c1">
<figure class="item-image">
<a href="/videos/%s.html?nats=abc123">
<img src="/content/%s.jpg" alt="Watch" /></a>
</figure>
<h3 class="item-title">%s</h3>
%s
<p class="item-description">%s</p>
</article>`, slug, thumb, title, perfHTML, desc)
}

func makePagination(pages int) string {
	var links []string
	for i := 2; i <= pages; i++ {
		links = append(links, fmt.Sprintf(`<a href="./updates_%d.html?sort=date" class="pagination__page">%d</a>`, i, i))
	}
	return `<nav class="pagination">` + strings.Join(links, "") + `</nav>`
}

const listingHTML = `<html><body>` +
	`<article class="content-list__item content-list__item--3 dfc__c1">
<figure class="item-image">
<a href="/videos/scene-one-slug.html?nats=abc">
<img src="/content/thumb1.jpg" alt="Watch" /></a>
</figure>
<h3 class="item-title">Scene One Title</h3>
<div class="name-date"><div class="model-name"><a href="/models/performer-a.html?nats=abc">Performer A</a> &amp; <a href="/models/performer-b.html?nats=abc">Performer B</a></div></div>
<p class="item-description">Description for scene one.</p>
</article>` +
	`<article class="content-list__item content-list__item--3 dfc__c1">
<figure class="item-image">
<a href="/videos/scene-two-slug.html?nats=abc">
<img src="/content/thumb2.jpg" alt="Watch" /></a>
</figure>
<h3 class="item-title">Scene Two Title</h3>
<div class="name-date"><div class="model-name"><a href="/models/performer-c.html?nats=abc">Performer C</a></div></div>
<p class="item-description">Description for scene two.</p>
</article>` +
	`<nav class="pagination">
<a href="./updates_2.html?sort=date" class="pagination__page">2</a>
<a href="./updates_3.html?sort=date" class="pagination__page">3</a>
</nav>` +
	`</body></html>`

func TestParseListingPage(t *testing.T) {
	scenes := parseListingPage([]byte(listingHTML), "https://www.example.com")
	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2", len(scenes))
	}

	s := scenes[0]
	if s.slug != "scene-one-slug" {
		t.Errorf("slug = %q", s.slug)
	}
	if s.title != "Scene One Title" {
		t.Errorf("title = %q", s.title)
	}
	if len(s.performers) != 2 || s.performers[0] != "Performer A" || s.performers[1] != "Performer B" {
		t.Errorf("performers = %v", s.performers)
	}
	if s.description != "Description for scene one." {
		t.Errorf("description = %q", s.description)
	}
	if s.thumb != "https://www.example.com/content/thumb1.jpg" {
		t.Errorf("thumb = %q", s.thumb)
	}

	s2 := scenes[1]
	if s2.slug != "scene-two-slug" {
		t.Errorf("slug = %q", s2.slug)
	}
	if len(s2.performers) != 1 || s2.performers[0] != "Performer C" {
		t.Errorf("performers = %v", s2.performers)
	}
}

func TestExtractMaxPage(t *testing.T) {
	max := extractMaxPage([]byte(listingHTML))
	if max != 3 {
		t.Errorf("maxPage = %d, want 3", max)
	}
}

func TestHasNextPage(t *testing.T) {
	if !hasNextPage([]byte(listingHTML), 1) {
		t.Error("should have next page from page 1")
	}
	if hasNextPage([]byte(listingHTML), 3) {
		t.Error("should not have next page from page 3")
	}
}

func TestExtractModelName(t *testing.T) {
	html := `<span class="model-name">Stacie Starr</span>`
	name := extractModelName([]byte(html))
	if name != "Stacie Starr" {
		t.Errorf("model name = %q, want %q", name, "Stacie Starr")
	}
}

func TestModelPagePerformers(t *testing.T) {
	modelHTML := `<html><body>
<span class="model-name">Test Model</span>
<article class="content-list__item content-list__item--3 dfc__c1">
<figure class="item-image">
<a href="/videos/test-scene.html?nats=abc">
<img src="/content/thumb.jpg" alt="Watch" /></a>
</figure>
<h3 class="item-title">Test Scene</h3>
<p class="item-description">Description.</p>
</article>
</body></html>`

	modelName := extractModelName([]byte(modelHTML))
	scenes := parseListingPage([]byte(modelHTML), "https://www.example.com")
	for i := range scenes {
		if len(scenes[i].performers) == 0 && modelName != "" {
			scenes[i].performers = []string{modelName}
		}
	}

	if len(scenes) != 1 {
		t.Fatalf("got %d scenes, want 1", len(scenes))
	}
	if len(scenes[0].performers) != 1 || scenes[0].performers[0] != "Test Model" {
		t.Errorf("performers = %v, want [Test Model]", scenes[0].performers)
	}
}

func TestToScene(t *testing.T) {
	ls := listingScene{
		slug:        "test-scene-slug",
		title:       "Test Scene",
		performers:  []string{"Performer A"},
		description: "A description.",
		thumb:       "https://www.example.com/content/thumb.jpg",
	}
	scene := ls.toScene("https://www.example.com", fixedTime)
	if scene.ID != "test-scene-slug" {
		t.Errorf("ID = %q", scene.ID)
	}
	if scene.SiteID != "seemomsuck" {
		t.Errorf("SiteID = %q", scene.SiteID)
	}
	if scene.URL != "https://www.example.com/videos/test-scene-slug.html" {
		t.Errorf("URL = %q", scene.URL)
	}
	if scene.Studio != "See Mom Suck" {
		t.Errorf("Studio = %q", scene.Studio)
	}
}

var fixedTime = func() time.Time {
	t, _ := time.Parse(time.RFC3339, "2026-04-27T12:00:00Z")
	return t
}()

func TestStripNATS(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"https://example.com/models/a.html?nats=abc123", "https://example.com/models/a.html"},
		{"https://example.com/models/a.html", "https://example.com/models/a.html"},
		{"https://example.com/page?foo=bar&nats=abc", "https://example.com/page?foo=bar"},
	}
	for _, c := range cases {
		if got := stripNATS(c.in); got != c.want {
			t.Errorf("stripNATS(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url  string
		want bool
	}{
		{"https://www.seemomsuck.com", true},
		{"https://seemomsuck.com/updates.html", true},
		{"https://www.seemomsuck.com/models/stacie-starr.html?nats=abc", true},
		{"https://example.com", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.want {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.want)
		}
	}
}

func TestRun(t *testing.T) {
	page1 := `<html><body>` +
		makeArticle("scene-a", "Scene A", "Performer A", "Desc A", "a") +
		makeArticle("scene-b", "Scene B", "Performer B", "Desc B", "b") +
		`</body></html>`

	var tsURL string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, strings.ReplaceAll(page1, "https://www.example.com", tsURL))
	}))
	defer ts.Close()
	tsURL = ts.URL

	s := &Scraper{client: ts.Client(), siteBase: ts.URL}
	out := make(chan scraper.SceneResult)
	go s.run(context.Background(), ts.URL, scraper.ListOpts{}, out)

	var scenes []string
	for r := range out {
		if r.Total > 0 || r.StoppedEarly {
			continue
		}
		if r.Err != nil {
			t.Logf("error: %v", r.Err)
			continue
		}
		scenes = append(scenes, r.Scene.ID)
	}
	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2: %v", len(scenes), scenes)
	}
}

func TestKnownIDsStopsEarly(t *testing.T) {
	page1 := `<html><body>` +
		makeArticle("scene-a", "Scene A", "Performer A", "Desc A", "a") +
		makeArticle("scene-b", "Scene B", "Performer B", "Desc B", "b") +
		`</body></html>`

	var tsURL string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, strings.ReplaceAll(page1, "https://www.example.com", tsURL))
	}))
	defer ts.Close()
	tsURL = ts.URL

	s := &Scraper{client: ts.Client(), siteBase: ts.URL}
	out := make(chan scraper.SceneResult)
	go s.run(context.Background(), ts.URL, scraper.ListOpts{
		KnownIDs: map[string]bool{"scene-b": true},
	}, out)

	var ids []string
	var stoppedEarly bool
	for r := range out {
		if r.Total > 0 {
			continue
		}
		if r.StoppedEarly {
			stoppedEarly = true
			continue
		}
		if r.Err != nil {
			t.Logf("error: %v", r.Err)
			continue
		}
		ids = append(ids, r.Scene.ID)
	}
	if !stoppedEarly {
		t.Error("expected StoppedEarly")
	}
	if len(ids) != 1 || ids[0] != "scene-a" {
		t.Errorf("got IDs %v, want [scene-a]", ids)
	}
}

func TestModelURL(t *testing.T) {
	modelPage := `<html><body>
<span class="model-name">Test Model</span>
` + makeArticle("model-scene-a", "Model Scene A", "", "Desc A", "a") +
		makeArticle("model-scene-b", "Model Scene B", "", "Desc B", "b") +
		`</body></html>`

	var tsURL string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, strings.ReplaceAll(modelPage, "https://www.example.com", tsURL))
	}))
	defer ts.Close()
	tsURL = ts.URL

	s := &Scraper{client: ts.Client(), siteBase: ts.URL}
	modelURL := tsURL + "/models/test-model.html"
	out := make(chan scraper.SceneResult)
	go s.run(context.Background(), modelURL, scraper.ListOpts{}, out)

	var scenes []scraper.SceneResult
	var total int
	for r := range out {
		if r.Total > 0 {
			total = r.Total
			continue
		}
		if r.StoppedEarly {
			continue
		}
		scenes = append(scenes, r)
	}
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2", len(scenes))
	}
	for _, r := range scenes {
		if r.Err != nil {
			t.Errorf("error: %v", r.Err)
			continue
		}
		if len(r.Scene.Performers) != 1 || r.Scene.Performers[0] != "Test Model" {
			t.Errorf("performers = %v, want [Test Model]", r.Scene.Performers)
		}
	}
}

func TestCleanDescription(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"Simple description.", "Simple description."},
		{"  Spaces around.  ", "Spaces around."},
		{"Line\none.", "Line one."},
		{"Entity &amp; test.", "Entity & test."},
	}
	for _, c := range cases {
		if got := cleanDescription(c.in); got != c.want {
			t.Errorf("cleanDescription(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestScraperInterface(t *testing.T) {
	var _ scraper.StudioScraper = New()
}
