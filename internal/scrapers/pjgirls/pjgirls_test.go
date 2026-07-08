package pjgirls

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/scraper"
)

const fixtureSitemap = `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>http://www.pjgirls.com/</loc></url>
  <url><loc>http://www.pjgirls.com/en/models/</loc></url>
  <url><loc>http://www.pjgirls.com/de/video/12-joy-of-pee/</loc></url>
  <url><loc>http://www.pjgirls.com/en/video/12-joy-of-pee/</loc></url>
  <url><loc>http://www.pjgirls.com/en/video/2000-diamond-shaped-pussy/</loc></url>
</urlset>`

// fixtureDetail mirrors the real PJ Girls detail markup: the scene's own
// metadata is in <div class="info">, while "SIMILAR VIDEOS" lower down carries
// its own /model/ links and durations that must NOT be picked up.
const fixtureDetail = `<!DOCTYPE html><html><head>
<title>Joy of pee - porn video [January 5, 2013] | PJGirls</title>
</head><body>
<div class="videoObal"><img src="/photo.php?type=intro2&amp;id_project=12" alt="Joy of pee - intro photo" /></div>
<h1>VIDEO: JOY OF PEE</h1>
<div class="detailUvod clear">
  <div class="text"><p></p></div>
  <div class="info">
    <h3>JANUARY 5, 2013</h3>
    <h3>LENGTH: 1:36</h3>
    <h3><a href="/en/model/gina" title="Gina">GINA</a></h3>
    <h3><a href="/en/model/delphine" title="Delphine">DELPHINE</a></h3>
  </div>
</div>
<h2 class="h1">SIMILAR VIDEOS</h2>
<div class="thumbs clear">
  <div class="thumb video">
    <a class="img" href="/en/video/4284-bonus-with-silvia-black/"><img src="x" /></a>
    <span><i class="fa fa-film"></i> 7:30</span>
    <h3><a href="/en/model/silviablack" title="Silvia Black">SILVIA</a></h3>
  </div>
</div>
</body></html>`

const fixtureDetail2 = `<!DOCTYPE html><html><head>
<title>Diamond-Shaped Pussy - porn video [February 4, 2019] | PJGirls</title>
</head><body>
<div class="detailUvod clear"><div class="info">
  <h3>FEBRUARY 4, 2019</h3>
  <h3>LENGTH: 17:34</h3>
  <h3><a href="/en/model/jessicadiamond" title="Jessica Diamond">JESSICA DIAMOND</a></h3>
</div></div>
</body></html>`

func newTestServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/sitemap.xml":
			_, _ = fmt.Fprint(w, fixtureSitemap)
		case "/en/video/12-joy-of-pee/":
			_, _ = fmt.Fprint(w, fixtureDetail)
		case "/en/video/2000-diamond-shaped-pussy/":
			_, _ = fmt.Fprint(w, fixtureDetail2)
		default:
			http.NotFound(w, r)
		}
	}))
}

func collectScenes(t *testing.T, s *Scraper) []models.Scene {
	t.Helper()
	ch, err := s.ListScenes(context.Background(), "https://www.pjgirls.com/", scraper.ListOpts{})
	if err != nil {
		t.Fatalf("ListScenes: %v", err)
	}
	var scenes []models.Scene
	for res := range ch {
		switch res.Kind {
		case scraper.KindScene:
			scenes = append(scenes, res.Scene)
		case scraper.KindError:
			t.Fatalf("scrape error: %v", res.Err)
		}
	}
	return scenes
}

func TestScrape(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	s := New()
	s.client = ts.Client()
	s.siteBase = ts.URL

	scenes := collectScenes(t, s)
	if len(scenes) != 2 {
		t.Fatalf("expected 2 scenes, got %d", len(scenes))
	}

	byID := map[string]models.Scene{}
	for _, sc := range scenes {
		byID[sc.ID] = sc
	}

	joy, ok := byID["12"]
	if !ok {
		t.Fatalf("scene id 12 not found; got %v", byID)
	}
	if joy.Title != "Joy of pee" {
		t.Errorf("title = %q, want %q", joy.Title, "Joy of pee")
	}
	if joy.SiteID != siteID {
		t.Errorf("siteID = %q, want %q", joy.SiteID, siteID)
	}
	if joy.Studio != studio {
		t.Errorf("studio = %q, want %q", joy.Studio, studio)
	}
	if got := joy.Date.Format("2006-01-02"); got != "2013-01-05" {
		t.Errorf("date = %q, want 2013-01-05", got)
	}
	if joy.Duration != 96 { // 1:36
		t.Errorf("duration = %d, want 96", joy.Duration)
	}
	wantPerf := []string{"Gina", "Delphine"}
	if strings.Join(joy.Performers, ",") != strings.Join(wantPerf, ",") {
		t.Errorf("performers = %v, want %v", joy.Performers, wantPerf)
	}
	if !strings.HasSuffix(joy.URL, "/en/video/12-joy-of-pee/") {
		t.Errorf("url = %q", joy.URL)
	}
	if !strings.Contains(joy.Thumbnail, "id_project=12") {
		t.Errorf("thumbnail = %q", joy.Thumbnail)
	}

	dia, ok := byID["2000"]
	if !ok {
		t.Fatalf("scene id 2000 not found")
	}
	if dia.Title != "Diamond-Shaped Pussy" {
		t.Errorf("title = %q, want %q", dia.Title, "Diamond-Shaped Pussy")
	}
	if dia.Duration != 17*60+34 {
		t.Errorf("duration = %d, want %d", dia.Duration, 17*60+34)
	}
	if len(dia.Performers) != 1 || dia.Performers[0] != "Jessica Diamond" {
		t.Errorf("performers = %v", dia.Performers)
	}
}

func TestTitleDateParse(t *testing.T) {
	page := `<title>Stretching Wide - porn video [January 29, 2016] | PJGirls</title>`
	m := titleRe.FindStringSubmatch(page)
	if m == nil {
		t.Fatal("titleRe did not match")
	}
	if got := cleanText(m[1]); got != "Stretching Wide" {
		t.Errorf("title = %q, want %q", got, "Stretching Wide")
	}
	if got := strings.TrimSpace(m[2]); got != "January 29, 2016" {
		t.Errorf("date = %q, want %q", got, "January 29, 2016")
	}
}

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url  string
		want bool
	}{
		{"https://www.pjgirls.com/", true},
		{"http://pjgirls.com/en/video/12-joy-of-pee/", true},
		{"https://www.pjgirls.com/en/models/", true},
		{"https://example.com/", false},
		{"https://notpjgirls.com/", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.want {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.want)
		}
	}
}

func TestSceneID(t *testing.T) {
	if got := sceneID("https://www.pjgirls.com/en/video/4284-bonus-with-silvia-black/"); got != "4284" {
		t.Errorf("sceneID = %q, want 4284", got)
	}
}
