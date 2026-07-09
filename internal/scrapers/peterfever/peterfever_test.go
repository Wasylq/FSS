package peterfever

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wasylq/FSS/parseutil"
	"github.com/Wasylq/FSS/scraper"
)

// card renders a real Peter Fever listing/model card (markup captured from
// www.peterfever.com) with the test server as the base so detail fetches hit
// the httptest server rather than the live site.
func card(base, slug, title, date string) string {
	return fmt.Sprintf(`
   <div class="item col-lg-3 col-sm-6 col-xs-12 padx">
                <div class="product-item">
                  <div class="pi-img-wrapper">
                    <a href="%[1]s/scenes/%[2]s_vids.html"><img src="%[1]s/content/contentthumbs/41/76/%[2]s.jpg" class="img-responsive" alt="%[3]s"></a>
                      <a href="%[1]s/scenes/%[2]s_vids.html" class="mplay"><img src="%[1]s/frontend/media/play-icon.png" /></a>
						<div class="title-info"><h3 class="vid"><a href="%[1]s/scenes/%[2]s_vids.html">%[3]s</a><span class="pi-added"><i class="fa fa-calendar"></i>  %[4]s</span></h3></div>
                  </div>
                </div>
              </div>`, base, slug, title, date)
}

func listingPage(base string, cards ...string) string {
	return `<!DOCTYPE html><html><body><div class="product-list">` + strings.Join(cards, "\n") + `</div></body></html>`
}

func detailPage(base, fullTitle, desc string) string {
	// Real og markup: og:image/og:title self-closed, plus the description in an
	// <h4> body block that the parser intentionally ignores in favour of og.
	return fmt.Sprintf(`<!DOCTYPE html><html><head>
<meta property="og:title" content="%[2]s"/>
<meta property="og:image" content="%[1]s/content/contentthumbs/41/76/14176-3x.jpg" />
<meta property="og:description" content="%[3]s"/>
</head><body>
<div class="video-block"><div class="container content"><h4>%[3]s full body text</h4></div></div>
</body></html>`, base, fullTitle, desc)
}

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	var base string
	mux.HandleFunc("/categories/movies.html", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, listingPage(base,
			card(base, "Lost-In-Puerto-Rico-Beach-Blanket-Backrub", "Lost In Puerto Rico-Beach Blanket...", "23 Jun 26"),
			card(base, "Sauna-Nights-2-Good-Clean-Sex", "Sauna Nights 2-Good Clean Sex", "06 Jan 26"),
		))
	})
	// page 2+ is empty → ends pagination.
	mux.HandleFunc("/categories/movies_2_d.html", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, listingPage(base))
	})
	mux.HandleFunc("/scenes/Lost-In-Puerto-Rico-Beach-Blanket-Backrub_vids.html", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, detailPage(base, "Lost In Puerto Rico-Beach Blanket Backrub", "Sam wanders a quiet sundrenched beach in San Juan."))
	})
	mux.HandleFunc("/scenes/Sauna-Nights-2-Good-Clean-Sex_vids.html", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, detailPage(base, "Sauna Nights 2-Good Clean Sex", "Steam and skin in the sauna."))
	})
	mux.HandleFunc("/models/ToyDaddy.html", func(w http.ResponseWriter, _ *http.Request) {
		page := `<!DOCTYPE html><html><body><h1>Toy Daddy</h1><div class="product-list">` +
			card(base, "Lost-In-Puerto-Rico-Beach-Blanket-Backrub", "Lost In Puerto Rico-Beach Blanket...", "23 Jun 26") +
			`</div></body></html>`
		_, _ = fmt.Fprint(w, page)
	})
	ts := httptest.NewServer(mux)
	base = ts.URL
	return ts
}

func collect(t *testing.T, sc *Scraper, url string) []scraper.SceneResult {
	t.Helper()
	ch, err := sc.ListScenes(context.Background(), url, scraper.ListOpts{})
	if err != nil {
		t.Fatalf("ListScenes: %v", err)
	}
	var out []scraper.SceneResult
	for r := range ch {
		out = append(out, r)
	}
	return out
}

func TestListing(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()
	oldBase := siteBase
	siteBase = ts.URL
	defer func() { siteBase = oldBase }()

	sc := New()
	sc.Client = ts.Client()

	results := collect(t, sc, ts.URL+"/categories/movies.html")

	scenes := make(map[string]scraper.SceneResult)
	for _, r := range results {
		if r.Kind == scraper.KindError {
			t.Fatalf("unexpected error result: %v", r.Err)
		}
		if r.Kind == scraper.KindScene {
			scenes[r.Scene.ID] = r
		}
	}
	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2", len(scenes))
	}

	got, ok := scenes["Lost-In-Puerto-Rico-Beach-Blanket-Backrub"]
	if !ok {
		t.Fatal("missing expected scene")
	}
	s := got.Scene
	if s.SiteID != "peterfever" {
		t.Errorf("SiteID = %q", s.SiteID)
	}
	if s.Studio != "Peter Fever" {
		t.Errorf("Studio = %q", s.Studio)
	}
	// Full title from og:title, not the truncated listing title.
	if s.Title != "Lost In Puerto Rico-Beach Blanket Backrub" {
		t.Errorf("Title = %q (want full og:title)", s.Title)
	}
	if !strings.HasSuffix(s.URL, "/scenes/Lost-In-Puerto-Rico-Beach-Blanket-Backrub_vids.html") {
		t.Errorf("URL = %q", s.URL)
	}
	if s.Date.Format("2006-01-02") != "2026-06-23" {
		t.Errorf("Date = %v, want 2026-06-23", s.Date)
	}
	if !strings.Contains(s.Description, "sundrenched beach") {
		t.Errorf("Description = %q", s.Description)
	}
	if !strings.HasSuffix(s.Thumbnail, "-3x.jpg") {
		t.Errorf("Thumbnail = %q (want og:image)", s.Thumbnail)
	}
}

func TestModelPage(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()
	oldBase := siteBase
	siteBase = ts.URL
	defer func() { siteBase = oldBase }()

	sc := New()
	sc.Client = ts.Client()

	results := collect(t, sc, ts.URL+"/models/ToyDaddy.html")

	var scene *scraper.SceneResult
	for i := range results {
		if results[i].Kind == scraper.KindError {
			t.Fatalf("unexpected error: %v", results[i].Err)
		}
		if results[i].Kind == scraper.KindScene {
			scene = &results[i]
		}
	}
	if scene == nil {
		t.Fatal("no scene from model page")
	}
	if len(scene.Scene.Performers) != 1 || scene.Scene.Performers[0] != "Toy Daddy" {
		t.Errorf("Performers = %v, want [Toy Daddy]", scene.Scene.Performers)
	}
	if scene.Scene.ID != "Lost-In-Puerto-Rico-Beach-Blanket-Backrub" {
		t.Errorf("ID = %q", scene.Scene.ID)
	}
}

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url  string
		want bool
	}{
		{"https://www.peterfever.com/categories/movies.html", true},
		{"https://www.peterfever.com/categories/movies_2_d.html", true},
		{"https://peterfever.com/models/ToyDaddy.html", true},
		{"http://www.peterfever.com/scenes/foo_vids.html", true},
		{"https://www.example.com/", false},
		{"https://www.notpeterfever.com/", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.want {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.want)
		}
	}
}

func TestDateParse(t *testing.T) {
	d, err := parseutil.TryParseDate("06 Jan 26", dateLayout)
	if err != nil {
		t.Fatalf("TryParseDate: %v", err)
	}
	if d.Format("2006-01-02") != "2026-01-06" {
		t.Errorf("got %v, want 2026-01-06", d)
	}
}
