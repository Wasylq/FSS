package nextcontents

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
	s := newScraper(siteConfig{SiteID: "freakmob", Base: "https://www.freakmobmedia.com", ListPath: "videos"})
	cases := map[string]bool{
		"https://www.freakmobmedia.com/videos":           true,
		"https://www.freakmobmedia.com/scenes/some-slug": true,
		"https://www.freakmobmedia.com/":                 true,
		"https://example.com/videos":                     false,
	}
	for u, want := range cases {
		if got := s.MatchesURL(u); got != want {
			t.Errorf("MatchesURL(%q) = %v, want %v", u, got, want)
		}
	}
}

func TestParsePublishDate(t *testing.T) {
	got := parsePublishDate("2026/06/13 12:00:00")
	if got.Format("2006-01-02") != "2026-06-13" {
		t.Errorf("parsePublishDate = %v", got)
	}
	if !parsePublishDate("").IsZero() {
		t.Error("empty date should be zero")
	}
}

const listingHTML = `<html><head></head><body>
<script id="__NEXT_DATA__" type="application/json">{"props":{},"buildId":"TESTBUILD123","page":"/videos"}</script>
</body></html>`

const pageJSON = `{"pageProps":{"contents":{"total":2,"page":"1","per_page":"24","total_pages":1,"data":[
  {"id":679,"title":"Carmen gets Creampied","slug":"carmen-gets-creampied","publish_date":"2026/06/26 00:00:00","seconds_duration":1790,"thumb":"https://cdn/thumb8.jpg","description":"drool &amp; slurps","content_price":20,"tags":["Big Tits","Creampies"],"models_slugs":[{"name":"Carmen Caliente","slug":"carmen-caliente"},{"name":"FREAKMOB","slug":"freakmob"}]}
]}}}`

func TestRunParsesContents(t *testing.T) {
	var sawBuildPath bool
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/videos":
			_, _ = fmt.Fprint(w, listingHTML)
		case strings.Contains(r.URL.Path, "/_next/data/TESTBUILD123/videos.json"):
			sawBuildPath = true
			_, _ = fmt.Fprint(w, pageJSON)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	s := newScraper(siteConfig{SiteID: "freakmob", Studio: "FreakMob Media", Base: ts.URL, ListPath: "videos", BrandSlug: "freakmob"})
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
	if !sawBuildPath {
		t.Fatal("build-id JSON route never fetched (buildId extraction failed)")
	}
	if len(scenes) != 1 {
		t.Fatalf("expected 1 scene, got %d", len(scenes))
	}
	sc := scenes[0].Scene
	if sc.Title != "Carmen gets Creampied" || sc.ID != "679" {
		t.Errorf("scene = %+v", sc)
	}
	if sc.Description != "drool & slurps" {
		t.Errorf("description should be HTML-unescaped, got %q", sc.Description)
	}
	if sc.URL != ts.URL+"/scenes/carmen-gets-creampied" {
		t.Errorf("url = %q", sc.URL)
	}
	if sc.Duration != 1790 {
		t.Errorf("duration = %d", sc.Duration)
	}
	if sc.Date.Format("2006-01-02") != "2026-06-26" {
		t.Errorf("date = %v", sc.Date)
	}
	// FREAKMOB brand dropped, only the real performer kept.
	if len(sc.Performers) != 1 || sc.Performers[0] != "Carmen Caliente" {
		t.Errorf("performers = %v (brand should be filtered)", sc.Performers)
	}
	if sc.LowestPrice != 20 {
		t.Errorf("price = %v", sc.LowestPrice)
	}
}
