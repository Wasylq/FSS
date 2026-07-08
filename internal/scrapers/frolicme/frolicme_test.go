package frolicme

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wasylq/FSS/scraper"
)

// Real captured JSON-LD + body fragments from www.frolicme.com.
const filmDetailHTML = `<!DOCTYPE html><html lang="en-US"><head>
<meta property="og:locale" content="en_US" />
<meta property="og:type" content="article" />
<meta property="og:title" content="Sexy Penelope cums over her fingers with some sexy clitoral stimulation" />
<meta property="og:url" content="https://www.frolicme.com/films/sexy-clitoral-stimulation/" />
<script type="application/ld+json" class="yoast-schema-graph">{"@context":"https://schema.org","@graph":[{"@type":"WebPage","@id":"https://www.frolicme.com/films/sexy-clitoral-stimulation/","url":"https://www.frolicme.com/films/sexy-clitoral-stimulation/","name":"Sexy Penelope cums over her fingers with some sexy clitoral stimulation","thumbnailUrl":"https://www.frolicme.com/wp-content/uploads/2017/10/152-FILM-FEATURE.jpg","datePublished":"2017-10-29T21:43:43+00:00","dateModified":"2020-06-30T14:12:48+00:00","description":"Enjoy now this online erotic adult film of a pretty Spanish girl enjoying some sexy clitoral stimulation as she watches her girlfriend.","inLanguage":"en-US"},{"@type":"ImageObject","@id":"https://www.frolicme.com/films/sexy-clitoral-stimulation/#primaryimage","url":"https://www.frolicme.com/wp-content/uploads/2017/10/152-FILM-FEATURE.jpg"},{"@type":"WebSite","@id":"https://www.frolicme.com/#website","name":"FrolicMe"}]}</script>
</head><body>
<span class="inline-flex items-center"><i class="inline-block w-4 h-4 i-mdi:account"></i> <a href="https://www.frolicme.com/models/penelope-cum/" rel="tag">Penelope Cum</a></span>
<span class="inline-flex items-center"><a href="https://www.frolicme.com/models/nick-moreno/" rel="tag">Nick Moreno</a></span>
<a href="https://www.frolicme.com/models/">Models</a>
<a href="https://www.frolicme.com/porn-films/female-masturbation/" rel="tag">Female Masturbation</a>
<a href="https://www.frolicme.com/porn-films/shaved-pussy/" rel="tag">Shaved Pussy</a>
<a href="https://www.frolicme.com/porn-films/brunette-porn/" rel="tag">Brunettes</a>
</body></html>`

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/sitemap.xml", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?>
<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
<sitemap><loc>https://www.frolicme.com/page-sitemap.xml</loc></sitemap>
<sitemap><loc>https://www.frolicme.com/cpt_films-sitemap.xml</loc></sitemap>
<sitemap><loc>https://www.frolicme.com/cpt_stories-sitemap.xml</loc></sitemap>
</sitemapindex>`)
	})
	mux.HandleFunc("/cpt_films-sitemap.xml", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns:image="http://www.google.com/schemas/sitemap-image/1.1" xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
<url><loc>https://www.frolicme.com/films/sexy-clitoral-stimulation/</loc><lastmod>2020-06-30T14:12:48+00:00</lastmod>
<image:image><image:loc>https://www.frolicme.com/wp-content/uploads/2017/10/152-FILM-FEATURE.jpg</image:loc></image:image></url>
</urlset>`)
	})
	mux.HandleFunc("/films/sexy-clitoral-stimulation/", func(w http.ResponseWriter, r *http.Request) {
		// Cloudflare returns the real body with a 403; mirror that here.
		w.WriteHeader(http.StatusForbidden)
		_, _ = fmt.Fprint(w, filmDetailHTML)
	})
	return httptest.NewServer(mux)
}

func TestListScenes(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	s := New()
	s.base = ts.URL

	ch, err := s.ListScenes(context.Background(), ts.URL+"/", scraper.ListOpts{})
	if err != nil {
		t.Fatalf("ListScenes: %v", err)
	}

	var scenes []scraper.SceneResult
	for r := range ch {
		if r.Kind == scraper.KindError {
			t.Fatalf("scrape error: %v", r.Err)
		}
		if r.Kind == scraper.KindScene {
			scenes = append(scenes, r)
		}
	}
	if len(scenes) != 1 {
		t.Fatalf("got %d scenes, want 1", len(scenes))
	}

	sc := scenes[0].Scene
	if sc.ID != "sexy-clitoral-stimulation" {
		t.Errorf("ID = %q", sc.ID)
	}
	if sc.SiteID != "frolicme" {
		t.Errorf("SiteID = %q", sc.SiteID)
	}
	if sc.Studio != "Frolic Me" {
		t.Errorf("Studio = %q", sc.Studio)
	}
	if sc.Title != "Sexy Penelope cums over her fingers with some sexy clitoral stimulation" {
		t.Errorf("Title = %q", sc.Title)
	}
	if got := sc.Date.Format("2006-01-02"); got != "2017-10-29" {
		t.Errorf("Date = %q, want 2017-10-29", got)
	}
	if sc.Description == "" {
		t.Errorf("Description empty")
	}
	if sc.Thumbnail != "https://www.frolicme.com/wp-content/uploads/2017/10/152-FILM-FEATURE.jpg" {
		t.Errorf("Thumbnail = %q", sc.Thumbnail)
	}
	wantPerf := []string{"Penelope Cum", "Nick Moreno"}
	if len(sc.Performers) != len(wantPerf) {
		t.Fatalf("Performers = %v, want %v", sc.Performers, wantPerf)
	}
	for i, p := range wantPerf {
		if sc.Performers[i] != p {
			t.Errorf("Performers[%d] = %q, want %q", i, sc.Performers[i], p)
		}
	}
	wantTags := []string{"Female Masturbation", "Shaved Pussy", "Brunettes"}
	if len(sc.Tags) != len(wantTags) {
		t.Fatalf("Tags = %v, want %v", sc.Tags, wantTags)
	}
	for i, tg := range wantTags {
		if sc.Tags[i] != tg {
			t.Errorf("Tags[%d] = %q, want %q", i, sc.Tags[i], tg)
		}
	}
}

func TestParseWebPage(t *testing.T) {
	node, ok := parseWebPage([]byte(filmDetailHTML))
	if !ok {
		t.Fatal("parseWebPage: not found")
	}
	if node.Name != "Sexy Penelope cums over her fingers with some sexy clitoral stimulation" {
		t.Errorf("Name = %q", node.Name)
	}
	if node.DatePublished != "2017-10-29T21:43:43+00:00" {
		t.Errorf("DatePublished = %q", node.DatePublished)
	}
	if node.Description == "" {
		t.Error("Description empty")
	}
	if node.ThumbnailURL == "" {
		t.Error("ThumbnailURL empty")
	}
}

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url  string
		want bool
	}{
		{"https://www.frolicme.com/", true},
		{"https://frolicme.com/films/foo/", true},
		{"http://www.frolicme.com/models/penelope-cum/", true},
		{"https://example.com/", false},
		{"https://notfrolicme.com/", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.want {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.want)
		}
	}
}

func TestFilmID(t *testing.T) {
	if got := filmID("https://www.frolicme.com/films/sexy-clitoral-stimulation/"); got != "sexy-clitoral-stimulation" {
		t.Errorf("filmID = %q", got)
	}
}
