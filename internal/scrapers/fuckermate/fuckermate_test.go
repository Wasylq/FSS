package fuckermate

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

// Real captured markup, trimmed to a representative card pair + the listing's
// rel="next" pager link.
const listingPage1 = `<html><body>
<div class="row">
	<div class="col-sm-6 col-md-4 col-lg-4">
		<div class="post mb-20">
			<div class="post-thumbnail">
				<a href="https://fuckermate.com/video/hot-medellin-jacuzzi-orgy">
					<img src="https://images.fuckermate.com/2025/08/orgy-640x360.jpg" alt="Thumbnail of Orgy"/>
				</a>
			</div>
			<div class="post-header font-alt">
				<h1 class="post-title">
					<a href="https://fuckermate.com/video/hot-medellin-jacuzzi-orgy">Hot Medellin Jacuzzi Orgy</a>
				</h1>
				<div class="post-meta">
					<a href="https://fuckermate.com/video/hot-medellin-jacuzzi-orgy">Group</a>
				</div>
			</div>
			<div class="post-entry"></div>
		</div>
	</div>
	<div class="col-sm-6 col-md-4 col-lg-4">
		<div class="post mb-20">
			<div class="post-thumbnail">
				<a href="https://fuckermate.com/video/chini-and-viktor-rom">
					<img src="https://images.fuckermate.com/2026/06/chini-and-viktor-rom-graded-640x360.jpg" alt="Thumbnail of Chini"/>
				</a>
			</div>
			<div class="post-header font-alt">
				<h1 class="post-title">
					<a href="https://fuckermate.com/video/chini-and-viktor-rom">Chini &amp; Viktor Rom</a>
				</h1>
				<div class="post-meta">
					<a href="https://fuckermate.com/video/chini-and-viktor-rom">Pico a Pico</a>
				</div>
			</div>
			<div class="post-entry"></div>
		</div>
	</div>
</div>
<ul class="pagination">
	<a class="page-link" href="http://www.fuckermate.com/video?page=2" rel="next" aria-label="Next">&rsaquo;</a>
</ul>
</body></html>`

// Detail fixture with a populated Cast section (multi-performer), real markup.
const detailOrgy = `<html><body>
<nav><a href="https://fuckermate.com/actor/role/top">Top</a><a href="https://fuckermate.com/actor/ethnicity/latin">Latin</a></nav>
<div class="post-header font-alt">
	<h1>Group</h1>
	<h2 class="post-title">Hot Medellin Jacuzzi Orgy</h2>
	<div class="post-meta">
		2025-08-01 | <a href="https://fuckermate.com/video/tag/bareback">bareback</a>, <a href="https://fuckermate.com/video/tag/orgy">orgy</a>, <a href="https://fuckermate.com/video/tag/jacuzzi">jacuzzi</a>
	</div>
</div>
<section class="module-small pt-20"><div class="container"><div class="row multi-columns-row post-columns">
	<div class="col-md-3 col-sm-6 col-xs-12">
		<div class="team-item">
			<a href="https://fuckermate.com/actor/viktor-rom">
				<div class="team-image"><img src="https://images.fuckermate.com/2018/05/vr-640x360.jpg" alt="Viktor Rom Photo"/></div>
			</a>
		</div>
		<div class="post mt-10 mb-20"><div class="post-header font-alt">
			<h1 class="post-title"><a href="https://fuckermate.com/actor/viktor-rom">Viktor Rom</a></h1>
			<div class="post-meta"><a href="https://fuckermate.com/actor/role/top">Top</a></div>
			<div class="post-meta"><a href="https://fuckermate.com/actor/ethnicity/latin">Latin</a></div>
		</div></div>
	</div>
	<div class="col-md-3 col-sm-6 col-xs-12">
		<div class="team-item">
			<a href="https://fuckermate.com/actor/christiam-titan">
				<div class="team-image"><img src="https://images.fuckermate.com/2025/06/ct-640x360.jpg" alt="Christiam Titan Photo"/></div>
			</a>
		</div>
		<div class="post mt-10 mb-20"><div class="post-header font-alt">
			<h1 class="post-title"><a href="https://fuckermate.com/actor/christiam-titan">Christiam Titan</a></h1>
			<div class="post-meta"><a href="https://fuckermate.com/actor/role/bottom">Bottom</a></div>
		</div></div>
	</div>
</div></div></section>
</body></html>`

// Detail fixture with an empty Cast section (recent scene, no linked cast).
const detailChini = `<html><body>
<div class="post-header font-alt">
	<h2 class="post-title">Chini and Viktor Rom</h2>
	<div class="post-meta">
		2026-06-17 | <a href="https://fuckermate.com/video/tag/tattoo">tattoo</a>, <a href="https://fuckermate.com/video/tag/hard-sex">hard sex</a>
	</div>
</div>
<section class="module-small pt-20"><div class="row multi-columns-row post-columns"></div></section>
</body></html>`

func newTestServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/video", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") == "1" || r.URL.Query().Get("page") == "" {
			_, _ = fmt.Fprint(w, listingPage1)
			return
		}
		// page 2+ : no cards -> Paginate stops.
		_, _ = fmt.Fprint(w, `<html><body></body></html>`)
	})
	mux.HandleFunc("/video/hot-medellin-jacuzzi-orgy", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, detailOrgy)
	})
	mux.HandleFunc("/video/chini-and-viktor-rom", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, detailChini)
	})
	return httptest.NewServer(mux)
}

func TestRunScrapesScenes(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	s := New()
	s.base = ts.URL

	out := make(chan scraper.SceneResult)
	go s.run(context.Background(), scraper.ListOpts{}, out)

	var scenes []models.Scene
	for res := range out {
		switch res.Kind {
		case scraper.KindScene:
			scenes = append(scenes, res.Scene)
		case scraper.KindError:
			t.Fatalf("unexpected error: %v", res.Err)
		}
	}

	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2", len(scenes))
	}

	orgy := scenes[0]
	if orgy.ID != "hot-medellin-jacuzzi-orgy" {
		t.Errorf("ID = %q", orgy.ID)
	}
	if orgy.Title != "Hot Medellin Jacuzzi Orgy" {
		t.Errorf("Title = %q", orgy.Title)
	}
	if orgy.URL != "https://fuckermate.com/video/hot-medellin-jacuzzi-orgy" {
		t.Errorf("URL = %q", orgy.URL)
	}
	if orgy.SiteID != siteID || orgy.Studio != studio {
		t.Errorf("SiteID/Studio = %q/%q", orgy.SiteID, orgy.Studio)
	}
	if got := orgy.Date.Format("2006-01-02"); got != "2025-08-01" {
		t.Errorf("Date = %q", got)
	}
	if orgy.ScrapedAt.IsZero() {
		t.Errorf("ScrapedAt not set")
	}
	if want := []string{"bareback", "orgy", "jacuzzi"}; strings.Join(orgy.Tags, ",") != strings.Join(want, ",") {
		t.Errorf("Tags = %v, want %v", orgy.Tags, want)
	}
	if want := []string{"Viktor Rom", "Christiam Titan"}; strings.Join(orgy.Performers, ",") != strings.Join(want, ",") {
		t.Errorf("Performers = %v, want %v", orgy.Performers, want)
	}
	if orgy.Thumbnail != "https://images.fuckermate.com/2025/08/orgy-640x360.jpg" {
		t.Errorf("Thumbnail = %q", orgy.Thumbnail)
	}

	chini := scenes[1]
	if chini.ID != "chini-and-viktor-rom" {
		t.Errorf("ID = %q", chini.ID)
	}
	// Detail page <h2> overrides the HTML-entity listing title.
	if chini.Title != "Chini and Viktor Rom" {
		t.Errorf("Title = %q", chini.Title)
	}
	if got := chini.Date.Format("2006-01-02"); got != "2026-06-17" {
		t.Errorf("Date = %q", got)
	}
	if len(chini.Performers) != 0 {
		t.Errorf("Performers = %v, want empty", chini.Performers)
	}
	if want := []string{"tattoo", "hard sex"}; strings.Join(chini.Tags, ",") != strings.Join(want, ",") {
		t.Errorf("Tags = %v, want %v", chini.Tags, want)
	}
}

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url  string
		want bool
	}{
		{"https://www.fuckermate.com/video", true},
		{"https://fuckermate.com/video/chini-and-viktor-rom", true},
		{"http://fuckermate.com/", true},
		{"https://www.example.com/video", false},
		{"https://notfuckermate.example.com", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.want {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.want)
		}
	}
}
