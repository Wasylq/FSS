package wifeysworld

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
	"github.com/Anastylosis/FSS/models"
	"github.com/Anastylosis/FSS/scraper"
)

func TestMatchesURL(t *testing.T) {
	s := New()
	for _, u := range []string{
		"https://wifeysworld.com",
		"https://wifeysworld.com/",
		"https://www.wifeysworld.com/store/",
		"http://wifeysworld.com/store/product.php?slug=adonis-remaster",
	} {
		if !s.MatchesURL(u) {
			t.Errorf("expected match: %s", u)
		}
	}
	for _, u := range []string{"https://example.com/", "https://wifeysworld.com.evil.test/"} {
		if s.MatchesURL(u) {
			t.Errorf("expected no match: %s", u)
		}
	}
}

// The store's other two categories are clothing and commissioned shoots, so a
// bare host means the movie catalogue; an explicit category is honoured.
func TestListingURL(t *testing.T) {
	old := baseURL
	baseURL = "https://wifeysworld.com"
	defer func() { baseURL = old }()

	cases := map[string]string{
		"https://wifeysworld.com/":                               "https://wifeysworld.com/store/?category=wifey_movies&sort=newest",
		"https://wifeysworld.com/store/":                         "https://wifeysworld.com/store/?category=wifey_movies&sort=newest",
		"https://wifeysworld.com/store/?category=wifey_trinkets": "https://wifeysworld.com/store/?category=wifey_trinkets&sort=newest",
	}
	for in, want := range cases {
		if got := listingURL(in); got != want {
			t.Errorf("listingURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseListing(t *testing.T) {
	cards := parseListing([]byte(storeHTML))
	if len(cards) != 2 {
		t.Fatalf("got %d cards, want 2", len(cards))
	}
	if cards[0].slug != "wifey-s-bbc-encounter" {
		t.Errorf("slug = %q", cards[0].slug)
	}
	// The card title is HTML-escaped in the markup.
	if cards[0].title != "Wifey's BBC Encounter 4K Remaster! 🎥" {
		t.Errorf("title = %q", cards[0].title)
	}
	if cards[0].price != 25 {
		t.Errorf("price = %v, want 25", cards[0].price)
	}
	if !strings.HasSuffix(cards[0].thumb, "/assets/img/store/a.jpeg") {
		t.Errorf("thumb = %q, want an absolute URL", cards[0].thumb)
	}
	if cards[1].slug != "ninja-returns" || cards[1].price != 19.5 {
		t.Errorf("second card = %+v", cards[1])
	}
}

func TestRunEndToEnd(t *testing.T) {
	srv := newTestStore(t)
	defer srv.Close()

	scenes, errs := collect(t, srv, srv.URL+"/", scraper.ListOpts{Workers: 2})
	if len(errs) != 0 {
		t.Fatalf("errors: %v", errs)
	}
	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2", len(scenes))
	}

	var got = map[string]bool{}
	for _, sc := range scenes {
		got[sc.ID] = true
		if sc.SiteID != siteID || sc.Studio != studio {
			t.Errorf("siteID/studio = %q/%q", sc.SiteID, sc.Studio)
		}
		if sc.ScrapedAt.IsZero() {
			t.Errorf("scene %s has zero ScrapedAt", sc.ID)
		}
	}
	if !got["wifey-s-bbc-encounter"] || !got["ninja-returns"] {
		t.Fatalf("scenes = %v", got)
	}

	for _, sc := range scenes {
		if sc.ID != "wifey-s-bbc-encounter" {
			continue
		}
		// The detail page's title, description, poster and SKU win over the card.
		if sc.Title != "Wifey's BBC Encounter 4K Remaster! 🎥" {
			t.Errorf("title = %q", sc.Title)
		}
		if !strings.HasPrefix(sc.Description, "Completely remastered") {
			t.Errorf("description = %q", sc.Description)
		}
		if !strings.HasSuffix(sc.Thumbnail, "/assets/img/store/big.png") {
			t.Errorf("thumbnail = %q, want the detail poster", sc.Thumbnail)
		}
		if want := []string{"🎬 Wifey Movies", "⬇️ Download Only"}; !eq(sc.Categories, want) {
			t.Errorf("categories = %v, want %v", sc.Categories, want)
		}
		if sc.ExternalIDs["wifeysworld_sku"] != "REMASTER-BBC" {
			t.Errorf("externalIDs = %v", sc.ExternalIDs)
		}
		// The store is the only place this catalogue carries a price, so it is
		// the one thing the rewrite must not drop.
		if len(sc.PriceHistory) != 1 || sc.PriceHistory[0].Regular != 25 {
			t.Errorf("priceHistory = %+v", sc.PriceHistory)
		}
		if sc.LowestPrice != 25 {
			t.Errorf("lowestPrice = %v", sc.LowestPrice)
		}
	}
}

// The listing is one page for the whole category, so a stored product must be
// skipped rather than ending the walk — only its detail fetch is saved.
func TestKnownIDsSkipsAndContinues(t *testing.T) {
	srv := newTestStore(t)
	defer srv.Close()

	s := useTestStore(t, srv)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	ch, err := s.ListScenes(ctx, srv.URL+"/", scraper.ListOpts{
		KnownIDs: map[string]bool{"wifey-s-bbc-encounter": true},
	})
	if err != nil {
		t.Fatal(err)
	}

	scenes, stopped := testutil.CollectScenesWithStop(t, ch)
	if !stopped {
		t.Error("expected StoppedEarly to report that something was skipped")
	}
	if len(scenes) != 1 || scenes[0].ID != "ninja-returns" {
		t.Fatalf("got %+v, want only the product behind the stored one", scenes)
	}
}

// Pointing at one product must not read the listing at all.
func TestSingleProductURL(t *testing.T) {
	srv := newTestStore(t)
	defer srv.Close()

	scenes, errs := collect(t, srv, srv.URL+"/store/product.php?slug=ninja-returns", scraper.ListOpts{})
	if len(errs) != 0 {
		t.Fatalf("errors: %v", errs)
	}
	if len(scenes) != 1 || scenes[0].ID != "ninja-returns" {
		t.Fatalf("got %+v", scenes)
	}
}

// A store that parses to nothing is a redesign, not an empty shop. Reporting it
// is what keeps an authoritative --full from deleting the catalogue on the
// strength of a broken parser.
func TestEmptyListingIsReported(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, "<html><body>redesigned again</body></html>")
	}))
	defer srv.Close()

	scenes, errs := collect(t, srv, srv.URL+"/", scraper.ListOpts{})
	if len(scenes) != 0 {
		t.Fatalf("got %d scenes, want none", len(scenes))
	}
	if len(errs) != 1 {
		t.Fatalf("errors = %v, want one", errs)
	}
	if k := scraper.Classify(errs[0]); k != scraper.FailureParse {
		t.Errorf("classified %v, want FailureParse", k)
	}
}

// ---- helpers ----

// useTestStore points the package's base URL at the test server for the
// duration of one test. baseURL is a package var, so this must be undone.
func useTestStore(t *testing.T, srv *httptest.Server) *Scraper {
	t.Helper()
	old := baseURL
	baseURL = srv.URL
	t.Cleanup(func() { baseURL = old })

	s := New()
	s.Client = srv.Client()
	return s
}

func collect(t *testing.T, srv *httptest.Server, u string, opts scraper.ListOpts) ([]models.Scene, []error) {
	t.Helper()
	s := useTestStore(t, srv)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	ch, err := s.ListScenes(ctx, u, opts)
	if err != nil {
		t.Fatalf("ListScenes: %v", err)
	}
	var scenes []models.Scene
	var errs []error
	for r := range ch {
		switch r.Kind {
		case scraper.KindScene:
			scenes = append(scenes, r.Scene)
		case scraper.KindError:
			errs = append(errs, r.Err)
		}
	}
	return scenes, errs
}

func eq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func newTestStore(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/store/product.php", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("slug") {
		case "wifey-s-bbc-encounter":
			_, _ = fmt.Fprint(w, productHTML)
		case "ninja-returns":
			_, _ = fmt.Fprint(w, strings.NewReplacer(
				"Wifey&#039;s BBC Encounter 4K Remaster! 🎥", "Ninja Returns",
				"REMASTER-BBC", "NINJA-1",
				"$25.00", "$19.50",
			).Replace(productHTML))
		default:
			http.NotFound(w, r)
		}
	})
	mux.HandleFunc("/store/", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, storeHTML)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		http.NotFound(w, nil)
	})
	return httptest.NewServer(mux)
}

const storeHTML = `<html><body><main class="d1-wrap"><div class="d1-grid">
<div class="d1-card">
  <a class="d1-card-img" href="/store/product.php?slug=wifey-s-bbc-encounter">
    <img src="/assets/img/store/a.jpeg" alt="Wifey&#039;s BBC Encounter 4K Remaster! &#127909;" loading="lazy">
    <span class="d1-badge">&#11015;&#65039; Download Only</span>
  </a>
  <div class="d1-card-body">
    <div class="d1-card-title"><a href="/store/product.php?slug=wifey-s-bbc-encounter">Wifey&#039;s BBC Encounter 4K Remaster! &#127909;</a></div>
    <p class="d1-card-excerpt">&nbsp;</p>
    <div class="d1-card-foot">
      <div class="d1-price-wrap"><span class="d1-price">$25.00</span></div>
    </div>
  </div>
</div>
<div class="d1-card">
  <a class="d1-card-img" href="/store/product.php?slug=ninja-returns">
    <img src="/assets/img/store/b.jpeg" alt="Ninja Returns" loading="lazy">
  </a>
  <div class="d1-card-body">
    <div class="d1-card-title"><a href="/store/product.php?slug=ninja-returns">Ninja Returns</a></div>
    <p class="d1-card-excerpt">&nbsp;</p>
    <div class="d1-card-foot">
      <div class="d1-price-wrap"><span class="d1-price">$19.50</span></div>
    </div>
  </div>
</div>
</div></main></body></html>`

const productHTML = `<html><head><title>x</title></head><body>
<div class="d1-pd-wrap">
  <div class="d1-pd-gallery"><div class="d1-pd-main">
    <img id="pdMain" src="/assets/img/store/big.png" alt="poster">
  </div></div>
  <div class="d1-pd-details">
    <div class="d1-pd-type">&#127916; Wifey Movies &middot; &#11015;&#65039; Download Only</div>
    <h1 class="d1-pd-title">Wifey&#039;s BBC Encounter 4K Remaster! 🎥</h1>
    <div class="d1-pd-price-row"><span class="d1-pd-price">$25.00</span></div>
    <div class="d1-pd-desc"> Completely remastered and re-edited, this scene runs almost 40 minutes. </div>
    <div class="d1-pd-meta"><span><strong>SKU:</strong> REMASTER-BBC</span></div>
  </div>
</div>
</body></html>`
