package meanawolf

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
	"github.com/Wasylq/FSS/scraper"
)

const listingFixture = `
<div class="item" data-setid="357" data-videoposter="https://cdn.example/06/53/653-2x.jpg?token=abc">
	<a href="https://meanawolf.com/scenes/Convince-Him_vids.html"><img/></a>
</div>
<div class="item" data-setid="397" data-videoposter="https://cdn.example/06/93/693-2x.jpg?token=def">
	<a href="/scenes/Nonutchallenge2_vids.html"><img/></a>
</div>
`

const detailFixture = `
<h1>Convince Him</h1>
<ul class="videoInfo">
	<li><span>RUNTIME:</span> 48:34</li>
	<li><span>PHOTOS:</span> <a href="https://meanawolf.com/scenes/Convince-Him_highres.html">58</a></li>
	<li><span>FEATURED:</span> June 26, 2026</li>
	<li><span>FEATURING:</span> 		<a href="https://meanawolf.com/models/MeanaWolf.html">Meana Wolf</a> </li>
	<li><span>CATEGORIES:</span> <a href="https://meanawolf.com/categories/bad-girl.html">Bad Girl</a> <a href="https://meanawolf.com/categories/pov.html">POV</a></li>
</ul>
`

func TestParseListing(t *testing.T) {
	scenes := parseListing(siteBase, []byte(listingFixture))
	if len(scenes) != 2 {
		t.Fatalf("expected 2 scenes, got %d", len(scenes))
	}
	if scenes[0].slug != "Convince-Him" {
		t.Errorf("slug = %q, want Convince-Him", scenes[0].slug)
	}
	if scenes[0].url != "https://meanawolf.com/scenes/Convince-Him_vids.html" {
		t.Errorf("url = %q", scenes[0].url)
	}
	if scenes[0].thumb == "" {
		t.Errorf("thumb should be set")
	}
	// relative scene href resolves to absolute
	if scenes[1].url != "https://meanawolf.com/scenes/Nonutchallenge2_vids.html" {
		t.Errorf("relative url = %q", scenes[1].url)
	}
}

func TestParseDetail(t *testing.T) {
	d := parseDetail([]byte(detailFixture))
	if d.title != "Convince Him" {
		t.Errorf("title = %q, want Convince Him", d.title)
	}
	if d.duration != 48*60+34 {
		t.Errorf("duration = %d, want %d", d.duration, 48*60+34)
	}
	if d.date.Format("2006-01-02") != "2026-06-26" {
		t.Errorf("date = %v, want 2026-06-26", d.date)
	}
	if len(d.performers) != 1 || d.performers[0] != "Meana Wolf" {
		t.Errorf("performers = %v, want [Meana Wolf]", d.performers)
	}
	if len(d.tags) != 2 || d.tags[0] != "Bad Girl" || d.tags[1] != "POV" {
		t.Errorf("tags = %v, want [Bad Girl POV]", d.tags)
	}
}

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := map[string]bool{
		"https://meanawolf.com/":                        true,
		"https://www.meanawolf.com/updates/page_1.html": true,
		"https://meanawolf.com/models/MeanaWolf.html":   true,
		"https://example.com/meanawolf":                 false,
	}
	for u, want := range cases {
		if got := s.MatchesURL(u); got != want {
			t.Errorf("MatchesURL(%q) = %v, want %v", u, got, want)
		}
	}
}

func TestSlugToTitle(t *testing.T) {
	if got := slugToTitle("No-Nut-Challenge"); got != "No Nut Challenge" {
		t.Errorf("slugToTitle = %q", got)
	}
}

// --- end to end ---------------------------------------------------------------
//
// The tests above call the parsers directly, which left ListScenes, run,
// enqueueListing, fetchPage and fetchDetail at 0% and the package at 33.8%. The
// base is now a field and parseListing/absURL take it, so the listing -> detail
// walk runs entirely against httptest.
func TestListScenesEndToEnd(t *testing.T) {
	var listingHits, detailHits int
	var ts *httptest.Server
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/updates/page_"):
			listingHits++
			if !strings.Contains(r.URL.Path, "page_1.html") {
				_, _ = fmt.Fprint(w, `<div class="none"></div>`)
				return
			}
			// The fixture mixes an absolute live href with a relative one; rewrite
			// the absolute one so both detail fetches stay on this server.
			_, _ = fmt.Fprint(w, strings.ReplaceAll(listingFixture, "https://meanawolf.com", ts.URL))
		case strings.HasPrefix(r.URL.Path, "/scenes/"):
			detailHits++
			_, _ = fmt.Fprint(w, detailFixture)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	s := New()
	s.client = ts.Client()
	s.base = ts.URL

	ch, err := s.ListScenes(context.Background(), ts.URL+"/updates/page_1.html", scraper.ListOpts{Workers: 2})
	if err != nil {
		t.Fatal(err)
	}
	scenes := testutil.CollectScenes(t, ch)
	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2", len(scenes))
	}
	if listingHits == 0 || detailHits != 2 {
		t.Errorf("listing/detail fetches = %d/%d, want listing>0 and detail=2", listingHits, detailHits)
	}
	for _, sc := range scenes {
		if !strings.HasPrefix(sc.URL, ts.URL) {
			t.Errorf("scene %q URL %q escaped the test server", sc.ID, sc.URL)
		}
		if sc.Title == "" {
			t.Errorf("scene %q has empty Title", sc.ID)
		}
		if sc.Duration == 0 {
			t.Errorf("scene %q has zero Duration — the detail RUNTIME was not parsed", sc.ID)
		}
	}
}
