package privatecastings

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wasylq/FSS/scraper"
)

const listingHTML = `<html><body>
<ul class='thumb-list row scenes'>

<li class="col-lg-3 col-md-4 col-sm-6 col-xs-12">
  <div class="scene">
    <a href="https://www.privatecastings.com/scene/casting-michelle-is-dpd-in-her-first-porn-scene/125"
       id="vthumb_125" class="scene-thumb" title="Casting: Michelle is DP&rsquo;d in her First Porn Scene">
      <img src="https://pcastings77.st-content.com/content/contentthumbs/3136.jpg?secure=ABC"
           title="Casting" alt="Private Castings" class="img-responsive"/>
    </a>
    <ul class="scene-votes">
      <li><span class="glyphicon glyphicon-thumbs-up"></span> <a href="https://www.privatecastings.com/scene/casting-michelle-is-dpd-in-her-first-porn-scene/125">38</a></li>
    </ul>
    <ul class="scene-models">
      <li><a href="https://www.privatecastings.com/pornstar/1775-suzi/">Suzi</a></li>
    </ul>
    <h3>
      <a href="https://www.privatecastings.com/scene/casting-michelle-is-dpd-in-her-first-porn-scene/125">Casting: Michelle is DP&rsquo;d in her First Porn Scene</a>
    </h3>
  </div>
</li>

<li class="col-lg-3 col-md-4 col-sm-6 col-xs-12">
  <div class="scene">
    <a href="https://www.privatecastings.com/scene/brigita-pretty-teen/353" id="vthumb_353" class="scene-thumb" title="Brigita Casting">
      <img src="https://pcastings77.st-content.com/content/contentthumbs/1883.jpg" class="img-responsive"/>
    </a>
    <ul class="scene-models">
      <li><a href="https://www.privatecastings.com/pornstar/1812-brigita/">Brigita</a></li>
      <li><a href="https://www.privatecastings.com/pornstar/1813-other/">Other Girl</a></li>
    </ul>
    <h3>
      <a href="https://www.privatecastings.com/scene/brigita-pretty-teen/353">Brigita, Pretty Teen &amp; Blonde</a>
    </h3>
  </div>
</li>

</ul>

<ul class="pagination">
  <li><span class="current">1</span></li>
  <li><a href="https://www.privatecastings.com/scenes/2/">2</a></li>
  <li><a href="https://www.privatecastings.com/scenes/11/">11</a></li>
</ul>
</body></html>`

const emptyHTML = `<html><body><div class="container">no scenes</div></body></html>`

func TestParseListing(t *testing.T) {
	items := parseListing([]byte(listingHTML))
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}

	first := items[0]
	if first.id != "125" {
		t.Errorf("ID = %q, want 125", first.id)
	}
	if first.url != "https://www.privatecastings.com/scene/casting-michelle-is-dpd-in-her-first-porn-scene/125" {
		t.Errorf("URL = %q", first.url)
	}
	// &rsquo; → ’ via html.UnescapeString
	if first.title != "Casting: Michelle is DP’d in her First Porn Scene" {
		t.Errorf("Title = %q (entity unescape failed?)", first.title)
	}
	if len(first.performers) != 1 || first.performers[0] != "Suzi" {
		t.Errorf("Performers = %v", first.performers)
	}
	// privatecastings cards have no date — the struct intentionally omits a
	// date field and Scene.Date stays zero downstream.
	if first.thumb != "https://pcastings77.st-content.com/content/contentthumbs/3136.jpg?secure=ABC" {
		t.Errorf("Thumb = %q", first.thumb)
	}

	second := items[1]
	if second.id != "353" {
		t.Errorf("Second ID = %q", second.id)
	}
	if len(second.performers) != 2 {
		t.Errorf("Second performers = %v", second.performers)
	}
	if second.title != "Brigita, Pretty Teen & Blonde" {
		t.Errorf("Second title = %q", second.title)
	}
}

func TestEstimateTotal(t *testing.T) {
	// pagination lists pages 1, 2, 11 → max 11 × 2 cards = 22.
	got := estimateTotal([]byte(listingHTML), 2)
	if got != 22 {
		t.Errorf("estimateTotal = %d, want 22", got)
	}
}

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url string
		ok  bool
	}{
		{"https://www.privatecastings.com/", true},
		{"https://privatecastings.com/scenes/3/", true},
		{"https://example.com/", false},
		{"https://www.privateblack.com/", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.ok {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.ok)
		}
	}
}

func TestListingURL(t *testing.T) {
	s := New()
	if got := s.listingURL(1); got != "https://www.privatecastings.com/scenes" {
		t.Errorf("page 1 → %q", got)
	}
	if got := s.listingURL(3); got != "https://www.privatecastings.com/scenes/3/" {
		t.Errorf("page 3 → %q", got)
	}
}

func TestListScenes_endToEnd(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		switch r.URL.Path {
		case "/scenes":
			_, _ = fmt.Fprint(w, listingHTML)
		default:
			_, _ = fmt.Fprint(w, emptyHTML)
		}
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client(), base: ts.URL}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/", scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}
	var scenes int
	for r := range ch {
		switch r.Kind {
		case scraper.KindScene:
			scenes++
			if r.Scene.Series != "Private Castings" {
				t.Errorf("Series = %q", r.Scene.Series)
			}
		case scraper.KindError:
			t.Errorf("unexpected error: %v", r.Err)
		}
	}
	if scenes != 2 {
		t.Errorf("got %d scenes, want 2", scenes)
	}
}

func TestListScenes_pornstarPage(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		switch r.URL.Path {
		case "/pornstar/1-gina-gerson/":
			_, _ = fmt.Fprint(w, listingHTML)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client(), base: ts.URL}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/pornstar/1-gina-gerson/", scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}
	var scenes int
	for r := range ch {
		switch r.Kind {
		case scraper.KindScene:
			scenes++
		case scraper.KindError:
			t.Errorf("unexpected error: %v", r.Err)
		}
	}
	if scenes != 2 {
		t.Errorf("got %d scenes, want 2", scenes)
	}
}
