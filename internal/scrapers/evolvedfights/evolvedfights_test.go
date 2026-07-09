package evolvedfights

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wasylq/FSS/scraper"
)

func TestUniqueSiteIDs(t *testing.T) {
	seen := map[string]bool{}
	for _, c := range sites {
		if seen[c.SiteID] {
			t.Errorf("duplicate SiteID: %s", c.SiteID)
		}
		seen[c.SiteID] = true
	}
}

func TestMatchesURL(t *testing.T) {
	s := newScraper(siteConfig{SiteID: "evolvedfights", Base: "https://evolvedfights.com", Listing: "updates", Sort: "p"})
	cases := map[string]bool{
		"https://evolvedfights.com/categories/updates_1_p.html": true,
		"https://www.evolvedfights.com/":                        true,
		"https://evolvedfights.com/scenes/foo_vids.html":        true,
		"https://example.com/":                                  false,
	}
	for u, want := range cases {
		if got := s.MatchesURL(u); got != want {
			t.Errorf("MatchesURL(%q) = %v, want %v", u, got, want)
		}
	}
}

const listingFixture = `<html><body>
<div class="latestUpdateB" data-setid="83">
  <div class="videoPic">
    <a href="https://evolvedfights.com/scenes/Kira-Noir-vs-Nathan-Bronson_vids.html">
      <img id="set-target-83" class="update_thumb thumbs stdimage" src0_1x="/content//contentthumbs/07/62/762-1x.jpg" />
    </a>
  </div>
  <div class="latestUpdateBinfo">
    <h4 class="link_bright"><a href="https://evolvedfights.com/scenes/Kira-Noir-vs-Nathan-Bronson_vids.html">Kira Noir vs Nathan Bronson</a></h4>
    <p class="link_light">
      <a class="link_bright infolink" href="/models/kira-noir.html">Kira Noir</a>,
      <a class="link_bright infolink" href="/models/NathanBronson.html">Nathan Bronson</a>
    </p>
  </div>
</div>
<div class="latestUpdateB" data-setid="99">
  <h4 class="link_bright"><a href="https://evolvedfights.com/dvds/some-dvd.html">A DVD Set</a></h4>
</div>
</body></html>`

const detailFixture = `<html><body>
<ul class="videoInfo">
  <li class="text_med"><span class="s_icon"><i class="fa-solid fa-calendar"></i></span><!-- Date --> 10/11/2019</li>
  <li class="text_med"><i class="fas fa-video"></i>46 min</li>
</ul>
<div class="updateInfo">Kira takes on Nathan in a brutal &amp; sweaty match.</div>
</body></html>`

func TestRunParsesScenes(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/categories/updates_1_"):
			_, _ = fmt.Fprint(w, listingFixture)
		case strings.Contains(r.URL.Path, "_vids.html"):
			_, _ = fmt.Fprint(w, detailFixture)
		default:
			_, _ = fmt.Fprint(w, `<html></html>`) // empty subsequent pages
		}
	}))
	defer ts.Close()

	s := newScraper(siteConfig{SiteID: "evolvedfights", Studio: "Evolved Fights", Base: ts.URL, Listing: "updates", Sort: "p"})
	s.base = ts.URL

	out := make(chan scraper.SceneResult)
	go s.run(context.Background(), scraper.ListOpts{}, out)

	var scenes []scraper.SceneResult
	for r := range out {
		if r.Kind == scraper.KindScene {
			scenes = append(scenes, r)
		}
		if r.Kind == scraper.KindError {
			t.Fatalf("unexpected error: %v", r.Err)
		}
	}
	if len(scenes) != 1 {
		t.Fatalf("expected 1 scene (DVD card filtered out), got %d", len(scenes))
	}
	sc := scenes[0].Scene
	if sc.ID != "83" {
		t.Errorf("id = %q", sc.ID)
	}
	if sc.Title != "Kira Noir vs Nathan Bronson" {
		t.Errorf("title = %q", sc.Title)
	}
	if !strings.HasSuffix(sc.URL, "/scenes/Kira-Noir-vs-Nathan-Bronson_vids.html") {
		t.Errorf("url = %q", sc.URL)
	}
	if sc.Date.Format("2006-01-02") != "2019-10-11" {
		t.Errorf("date = %v", sc.Date)
	}
	if sc.Duration != 46*60 {
		t.Errorf("duration = %d", sc.Duration)
	}
	if len(sc.Performers) != 2 || sc.Performers[0] != "Kira Noir" || sc.Performers[1] != "Nathan Bronson" {
		t.Errorf("performers = %v", sc.Performers)
	}
	if sc.Description != "Kira takes on Nathan in a brutal & sweaty match." {
		t.Errorf("description = %q", sc.Description)
	}
	if !strings.HasSuffix(sc.Thumbnail, "/content//contentthumbs/07/62/762-1x.jpg") {
		t.Errorf("thumbnail = %q", sc.Thumbnail)
	}
}

func TestParseDetailDate(t *testing.T) {
	if d := parseDetailDate([]byte(detailFixture)); d.Format("2006-01-02") != "2019-10-11" {
		t.Errorf("date = %v", d)
	}
}
