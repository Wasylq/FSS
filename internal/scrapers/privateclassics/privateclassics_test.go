package privateclassics

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wasylq/FSS/scraper"
)

const listingHTML = `<html><body>
<section class="latest-videos"><div class="container"><ul class="content-list">

<li class="site-movies">
  <article class="content video scene ">
    <figure>
      <a href="https://www.privateclassics.com/en/scene/Liz-Honey-enjoys-an-all-anal-fuck/10652">
        <img data-src="https://pclassics77.st-content.com/content/contentthumbs/34502.jpg?secure=ABC"
             class="img-responsive lazyload" alt="Liz Honey" title="Liz Honey">
      </a>
    </figure>
    <div class="content-text">
      <h1>
        <a href="https://www.privateclassics.com/en/scene/Liz-Honey-enjoys-an-all-anal-fuck/10652">Liz Honey enjoys an all anal fuck</a>
      </h1>
      <ul class="list-models">
        <li><a href="https://www.privateclassics.com/en/pornstar/135-liz-honey">Liz Honey</a></li>
      </ul>
      <div class="scene-votes">…</div>
    </div>
  </article>
</li>

<li class="site-movies">
  <article class="content video scene ">
    <figure>
      <a href="https://www.privateclassics.com/en/scene/Two-Girls-Threesome/10641">
        <img data-src="https://pclassics77.st-content.com/content/contentthumbs/34490.jpg" class="img-responsive lazyload">
      </a>
    </figure>
    <div class="content-text">
      <h1>
        <a href="https://www.privateclassics.com/en/scene/Two-Girls-Threesome/10641">Two Girls Have A Threesome &amp; More</a>
      </h1>
      <ul class="list-models">
        <li><a href="https://www.privateclassics.com/en/pornstar/100-boroka">Boroka Balls</a></li>
        <li><a href="https://www.privateclassics.com/en/pornstar/101-niki">Niki Belucci</a></li>
      </ul>
    </div>
  </article>
</li>

</ul></div></section>

<ul class="pagination">
  <li class="active"><span>1</span></li>
  <li><a href="https://www.privateclassics.com/en/scenes/2/">2</a></li>
  <li><a href="https://www.privateclassics.com/en/scenes/25/">25</a></li>
</ul>
</body></html>`

const emptyHTML = `<html><body><div class="container">no scenes</div></body></html>`

func TestParseListing(t *testing.T) {
	items := parseListing([]byte(listingHTML))
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}

	first := items[0]
	if first.id != "10652" {
		t.Errorf("ID = %q, want 10652", first.id)
	}
	if first.url != "https://www.privateclassics.com/en/scene/Liz-Honey-enjoys-an-all-anal-fuck/10652" {
		t.Errorf("URL = %q", first.url)
	}
	if first.title != "Liz Honey enjoys an all anal fuck" {
		t.Errorf("Title = %q", first.title)
	}
	if len(first.performers) != 1 || first.performers[0] != "Liz Honey" {
		t.Errorf("Performers = %v", first.performers)
	}
	if first.thumb != "https://pclassics77.st-content.com/content/contentthumbs/34502.jpg?secure=ABC" {
		t.Errorf("Thumb = %q (expected data-src lazyload URL)", first.thumb)
	}

	second := items[1]
	if second.id != "10641" {
		t.Errorf("Second ID = %q", second.id)
	}
	if second.title != "Two Girls Have A Threesome & More" {
		t.Errorf("Second title = %q", second.title)
	}
	if len(second.performers) != 2 {
		t.Errorf("Second performers = %v", second.performers)
	}
}

func TestEstimateTotal(t *testing.T) {
	// pagination lists pages 1, 2, 25 → max 25 × 2 cards = 50.
	got := estimateTotal([]byte(listingHTML), 2)
	if got != 50 {
		t.Errorf("estimateTotal = %d, want 50", got)
	}
}

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url string
		ok  bool
	}{
		{"https://www.privateclassics.com/", true},
		{"https://privateclassics.com/en/scenes/2/", true},
		{"https://example.com/", false},
		{"https://www.private.com/", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.ok {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.ok)
		}
	}
}

func TestListingURL(t *testing.T) {
	s := New()
	if got := s.listingURL(1); got != "https://www.privateclassics.com/en/scenes/" {
		t.Errorf("page 1 → %q", got)
	}
	if got := s.listingURL(5); got != "https://www.privateclassics.com/en/scenes/5/" {
		t.Errorf("page 5 → %q", got)
	}
}

func TestListScenes_endToEnd(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		switch r.URL.Path {
		case "/en/scenes/":
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
			if r.Scene.Series != "Private Classics" {
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
		case "/en/pornstar/1-gina-gerson":
			_, _ = fmt.Fprint(w, listingHTML)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client(), base: ts.URL}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/en/pornstar/1-gina-gerson", scraper.ListOpts{})
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
