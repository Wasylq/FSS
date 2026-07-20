package nextcontents

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
	// ListPath is "videos" here, so the detail route is /videos/ too — a
	// hardcoded /scenes/ 404s on FreakMob.
	if sc.URL != ts.URL+"/videos/carmen-gets-creampied" {
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

// TestStickyDollarsSites pins the Sticky Dollars rows, in particular that
// Dirty Auditions points at its apex domain. tour.dirtyauditions.com is a
// misconfigured host serving an unrelated site (AD4X, 1309 scenes on a
// /videos route), so a tour.* Base there would scrape the wrong catalogue.
func TestStickyDollarsSites(t *testing.T) {
	byID := map[string]siteConfig{}
	for _, c := range sites {
		byID[c.SiteID] = c
	}

	want := map[string]string{
		"trueanal":       "https://tour.trueanal.com",
		"nympho":         "https://tour.nympho.com",
		"dirtyauditions": "https://dirtyauditions.com",
		"allanal":        "https://tour.allanal.com",
		"analonly":       "https://tour.analonly.com",
	}
	for id, base := range want {
		cfg, ok := byID[id]
		if !ok {
			t.Errorf("missing site %q", id)
			continue
		}
		if cfg.Base != base {
			t.Errorf("%s: Base = %q, want %q", id, cfg.Base, base)
		}
		if cfg.ListPath != "scenes" {
			t.Errorf("%s: ListPath = %q, want scenes", id, cfg.ListPath)
		}
		if cfg.Studio == "" {
			t.Errorf("%s: Studio is empty", id)
		}
	}

	if got := byID["dirtyauditions"].Base; strings.HasPrefix(got, "https://tour.") {
		t.Errorf("dirtyauditions Base is %q — the tour subdomain serves AD4X, not Dirty Auditions", got)
	}
}

// Several hub-only brands legitimately share a Base, so the unique key is the
// Base and ListPath together — that pair is what identifies a catalogue.
func TestUniqueSiteIDsAndRoutes(t *testing.T) {
	ids := map[string]bool{}
	routes := map[string]bool{}
	for _, c := range sites {
		if ids[c.SiteID] {
			t.Errorf("duplicate SiteID %q", c.SiteID)
		}
		ids[c.SiteID] = true
		route := c.Base + "/" + c.ListPath
		if routes[route] {
			t.Errorf("duplicate route %q", route)
		}
		routes[route] = true
	}
}

// TestHubOnlyBrandsMatchTheirOwnPath covers the Top Web Models brands that have
// no domain of their own. They share the hub's Base, so each must match only
// its /sites/{domain} path — otherwise whichever registered first would answer
// for all of them, and for the bare hub URL, which aggregates other brands.
func TestHubOnlyBrandsMatchTheirOwnPath(t *testing.T) {
	byID := map[string]*Scraper{}
	for _, c := range sites {
		byID[c.SiteID] = newScraper(c)
	}

	classics, vault := byID["twmclassics"], byID["twmpornvault"]
	if classics == nil || vault == nil {
		t.Fatal("missing hub-only TWM brands")
	}

	const classicsURL = "https://tour.topwebmodels.com/sites/twmclassics.com"
	const vaultURL = "https://tour.topwebmodels.com/sites/twm-porn-vault.com"

	if !classics.MatchesURL(classicsURL) {
		t.Errorf("twmclassics does not match its own path")
	}
	if classics.MatchesURL(vaultURL) {
		t.Errorf("twmclassics wrongly matches the porn-vault path")
	}
	if !vault.MatchesURL(vaultURL) {
		t.Errorf("twmpornvault does not match its own path")
	}
	if vault.MatchesURL(classicsURL) {
		t.Errorf("twmpornvault wrongly matches the classics path")
	}

	// The bare hub is an aggregate of brands covered by other entries, so no
	// hub-only scraper may claim it.
	for _, id := range []string{"topwebmodels", "twmclassics", "twminterviews", "twmpornvault"} {
		if byID[id].MatchesURL("https://tour.topwebmodels.com/scenes") {
			t.Errorf("%s must not match the bare hub listing", id)
		}
	}

	// Own-domain brands still match on host alone.
	if !byID["biggulpgirls"].MatchesURL("https://tour.biggulpgirls.com/scenes") {
		t.Error("biggulpgirls should match its own host")
	}
}

// The scene route follows the listing route: a site whose catalogue is at
// /videos serves its detail pages at /videos/{slug}, and a hardcoded /scenes/
// would 404 (FreakMob and AltErotic both do).
func TestScenePathFollowsListPath(t *testing.T) {
	cases := map[string]string{
		"videos":                 "videos",
		"scenes":                 "scenes",
		"sites/twmclassics.com":  "scenes",
		"sites/topwebmodels.com": "scenes",
	}
	for listPath, want := range cases {
		s := &Scraper{cfg: siteConfig{ListPath: listPath}}
		if got := s.scenePath(); got != want {
			t.Errorf("scenePath() for ListPath %q = %q, want %q", listPath, got, want)
		}
	}
}

func TestScenePathIsUsedInSceneURL(t *testing.T) {
	for _, cfg := range sites {
		if cfg.SiteID != "alterotic" {
			continue
		}
		s := newScraper(cfg)
		sc := s.toScene(contentItem{ID: 1848, Slug: "some-scene"}, time.Time{})
		if want := "https://alterotic.com/videos/some-scene"; sc.URL != want {
			t.Errorf("URL = %q, want %q", sc.URL, want)
		}
	}
}

// The CMS prefixes many tags with a non-breaking space, which is not stripped
// by taking the raw JSON values.
func TestCleanTags(t *testing.T) {
	got := cleanTags([]string{" arm Tattoo", "Bbc", " Bbc ", "", "   ", " Bbc", "Sleeve Tattoo"})
	want := []string{"arm Tattoo", "Bbc", "Sleeve Tattoo"}
	if len(got) != len(want) {
		t.Fatalf("cleanTags = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("cleanTags[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// Not every site in the family fills the same fields: some leave
// seconds_duration null and publish the runtime only as a display string, in
// either of two formats.
func TestDurationFallbacks(t *testing.T) {
	cases := []struct {
		name string
		item contentItem
		want int
	}{
		{"seconds wins", contentItem{SecondsDuration: 1337, VideosDuration: "22:17"}, 1337},
		{"colon fallback", contentItem{VideosDuration: "22:17"}, 22*60 + 17},
		{"float-seconds fallback", contentItem{VideosDuration: "1964.77"}, 1964},
		{"integer-seconds fallback", contentItem{VideosDuration: "600"}, 600},
		{"absent", contentItem{}, 0},
		{"garbage", contentItem{VideosDuration: "soon"}, 0},
		{"negative", contentItem{VideosDuration: "-5"}, 0},
	}
	for _, c := range cases {
		if got := duration(c.item); got != c.want {
			t.Errorf("%s: duration = %d, want %d", c.name, got, c.want)
		}
	}
}

// The older field name is protocol-relative, so it cannot be used verbatim.
func TestThumbnailFallbacks(t *testing.T) {
	cases := []struct {
		name string
		item contentItem
		want string
	}{
		{"thumb wins", contentItem{Thumb: "https://a/x.jpg", Thumbnail: "//b/y.jpg"}, "https://a/x.jpg"},
		{"protocol-relative gets a scheme", contentItem{Thumbnail: "//b/y.jpg"}, "https://b/y.jpg"},
		{"absolute passes through", contentItem{Thumbnail: "https://b/y.jpg"}, "https://b/y.jpg"},
		{"absent", contentItem{}, ""},
	}
	for _, c := range cases {
		if got := thumbnail(c.item); got != c.want {
			t.Errorf("%s: thumbnail = %q, want %q", c.name, got, c.want)
		}
	}
}
