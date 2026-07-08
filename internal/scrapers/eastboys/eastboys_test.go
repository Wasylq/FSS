package eastboys

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

// card1 / card2 are real listing-card markup captured from
// eastboys.com/tour/video?order=newest.
const card1 = `<div class="item col-xl-3 col-lg-4 col-md-6 col-sm-12 lofaser">
<div class="movie type-movie status-publish has-post-thumbnail hentry">
<div class="gen-movie-contain">
<a href="/tour/eastboys-trailer/3730/casting-matias-robles">
<div class="gen-movie-img" id="nakladacka3">
<img class="nahledak" src="https://cdn1-static.eastboys.com/tour/content/matias-robles-01/0.jpg" alt="owl-carousel-video-image">
</div>
</a>
<div class="gen-info-contain">
<div class="gen-movie-info">
<h3><a href="/tour/eastboys-trailer/3730/casting-matias-robles"></a><a href="/tour/eastboys-trailer/3730/casting-matias-robles">Casting - Matias Robles</a></h3>
</div>
<div class="gen-movie-meta-holder">
<ul>
<li> 19:15 Minutes </li>
<li><a href="/tour/eastboys-trailer/3730/casting-matias-robles"><span>22-06-2026</span></a></li>
</ul>
</div>
</div>
</div>
</div>
</div>`

const card2 = `<div class="item col-xl-3 col-lg-4 col-md-6 col-sm-12 lofaser">
<div class="movie type-movie status-publish has-post-thumbnail hentry">
<div class="gen-movie-contain">
<a href="/tour/eastboys-trailer/3729/daniel-rabit-handjob-in-a-hammock">
<div class="gen-movie-img" id="nakladacka3">
<img class="nahledak" src="https://cdn1-static.eastboys.com/tour/content/daniel-rabit-03/0.jpg" alt="owl-carousel-video-image">
</div>
</a>
<div class="gen-info-contain">
<div class="gen-movie-info">
<h3><a href="/tour/eastboys-trailer/3729/daniel-rabit-handjob-in-a-hammock"></a><a href="/tour/eastboys-trailer/3729/daniel-rabit-handjob-in-a-hammock">Daniel Rabit - Handjob in a hammock</a></h3>
</div>
<div class="gen-movie-meta-holder">
<ul>
<li> 12:30 Minutes </li>
<li><a href="/tour/eastboys-trailer/3729/daniel-rabit-handjob-in-a-hammock"><span>20-06-2026</span></a></li>
</ul>
</div>
</div>
</div>
</div>
</div>`

const paginationBlock = `<div class="gen-pagination2">
<nav aria-label="Page navigation"><ul class="page-numbers">
<li><a class="page-numbers current">1</a></li>
</ul></nav>
</div>`

func listingPage(cards string) string {
	return `<html><body><div class="row">` + cards + `</div>` + paginationBlock + `</body></html>`
}

// detailPage builds a detail fixture with a JSON-LD VideoObject + categories.
const detail3730 = `<html><body>
<script type="application/ld+json">
{
  "@context": "https://schema.org",
  "@type": "VideoObject",
  "name": "Casting - Matias Robles",
  "description": "Sitting shirtless and already horny, Matias Robles pulls out his cock.",
  "thumbnailUrl": "https://cdn1-static.eastboys.com//tour/content/matias-robles-01/0.jpg",
  "uploadDate": "2026-06-22T00:00:00+00:00",
  "duration": "PT19M15S",
  "embedUrl": "https://www.eastboys.com/tour/eastboys-trailer/3730/casting-matias-robles",
  "actor": [
    { "@type": "Person", "name": "Matias Robles", "url": "https://eastboys.com/actor-from-eastboys/1904/matias-robles" }
  ]
}
</script>
<div class="col-lg-12 pt-2">
<h6>Categories</h6>
<span class="kat-single"><a href="/tour/categories/5/movies/">Movies</a></span>
<span class="kat-single"><a href="/tour/categories/43/solo/">Solo</a></span>
<span class="kat-single"><a href="/tour/categories/108/latino/">Latino</a></span>
</div>
</body></html>`

const detail3729 = `<html><body>
<script type="application/ld+json">
{
  "@context": "https://schema.org",
  "@type": "VideoObject",
  "name": "Daniel Rabit - Handjob in a hammock",
  "description": "Daniel relaxes in a hammock.",
  "thumbnailUrl": "https://cdn1-static.eastboys.com//tour/content/daniel-rabit-03/0.jpg",
  "uploadDate": "2026-06-20T00:00:00+00:00",
  "duration": "PT12M30S",
  "actor": [
    { "@type": "Person", "name": "Daniel Rabit", "url": "https://eastboys.com/actor-from-eastboys/1901/daniel-rabit" }
  ]
}
</script>
<div class="col-lg-12 pt-2">
<h6>Categories</h6>
<span class="kat-single"><a href="/tour/categories/43/solo/">Solo</a></span>
</div>
</body></html>`

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/tour/video", func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")
		if page == "" || page == "1" {
			_, _ = fmt.Fprint(w, listingPage(card1+card2))
			return
		}
		// Page 2+ has no cards -> Paginate stops.
		_, _ = fmt.Fprint(w, listingPage(""))
	})
	mux.HandleFunc("/tour/eastboys-trailer/3730/casting-matias-robles", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, detail3730)
	})
	mux.HandleFunc("/tour/eastboys-trailer/3729/daniel-rabit-handjob-in-a-hammock", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, detail3729)
	})
	return httptest.NewServer(mux)
}

func collect(t *testing.T, s *Scraper) []models.Scene {
	t.Helper()
	ch, err := s.ListScenes(context.Background(), s.base, scraper.ListOpts{})
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
	ts := newTestServer(t)
	defer ts.Close()

	s := New()
	s.client = ts.Client()
	s.base = ts.URL

	scenes := collect(t, s)
	if len(scenes) != 2 {
		t.Fatalf("expected 2 scenes, got %d", len(scenes))
	}

	byID := map[string]models.Scene{}
	for _, sc := range scenes {
		byID[sc.ID] = sc
	}

	sc, ok := byID["3730"]
	if !ok {
		t.Fatal("missing scene 3730")
	}
	if sc.Title != "Casting - Matias Robles" {
		t.Errorf("title = %q", sc.Title)
	}
	if sc.SiteID != siteID || sc.Studio != studio {
		t.Errorf("siteID/studio = %q/%q", sc.SiteID, sc.Studio)
	}
	wantURL := ts.URL + "/tour/eastboys-trailer/3730/casting-matias-robles"
	if sc.URL != wantURL {
		t.Errorf("url = %q want %q", sc.URL, wantURL)
	}
	if got := sc.Date.Format("2006-01-02"); got != "2026-06-22" {
		t.Errorf("date = %q", got)
	}
	if sc.Duration != 19*60+15 {
		t.Errorf("duration = %d", sc.Duration)
	}
	if len(sc.Performers) != 1 || sc.Performers[0] != "Matias Robles" {
		t.Errorf("performers = %v", sc.Performers)
	}
	wantTags := []string{"Movies", "Solo", "Latino"}
	if strings.Join(sc.Tags, ",") != strings.Join(wantTags, ",") {
		t.Errorf("tags = %v want %v", sc.Tags, wantTags)
	}
	if sc.Thumbnail == "" {
		t.Error("thumbnail empty")
	}
	if sc.Description == "" {
		t.Error("description empty")
	}

	sc2 := byID["3729"]
	if len(sc2.Performers) != 1 || sc2.Performers[0] != "Daniel Rabit" {
		t.Errorf("scene 3729 performers = %v", sc2.Performers)
	}
	if sc2.Duration != 12*60+30 {
		t.Errorf("scene 3729 duration = %d", sc2.Duration)
	}
}

func TestParseMaxPage(t *testing.T) {
	block := `<div class="gen-pagination2">
<ul class="page-numbers">
<li><a class="page-numbers current">1</a></li>
<li><a class="page-numbers" href="https://www.eastboys.com/tour/video?order=newest&page=2">2</a></li>
<li><a class="page-numbers" href="https://www.eastboys.com/tour/video?order=newest&page=81">81</a></li>
<li><a class="page-numbers" href="https://www.eastboys.com/tour/video?order=newest&page=82">82</a></li>
</ul></div>`
	if got := parseMaxPage([]byte(block)); got != 82 {
		t.Errorf("parseMaxPage = %d want 82", got)
	}
	if got := parseMaxPage([]byte("<div>no pagination</div>")); got != 0 {
		t.Errorf("parseMaxPage(none) = %d want 0", got)
	}
}

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url  string
		want bool
	}{
		{"https://www.eastboys.com/tour/video", true},
		{"https://www.eastboys.com/tour/video?order=newest", true},
		{"https://eastboys.com/tour/eastboys-trailer/3730/casting-matias-robles", true},
		{"http://www.eastboys.com/tour/actor-from-eastboys/1904/matias-robles", true},
		{"https://www.eastboys.com/tour/categories/108/latino", true},
		{"https://www.hammerboys.com/", false},
		{"https://example.com/eastboys.com", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.want {
			t.Errorf("MatchesURL(%q) = %v want %v", c.url, got, c.want)
		}
	}
}
