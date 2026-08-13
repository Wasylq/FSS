package thelisaannvod

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Anastylosis/FSS/models"
	"github.com/Anastylosis/FSS/scraper"
)

func TestScraperInterface(t *testing.T) {
	var _ scraper.StudioScraper = New()
}

// The bare host belongs to the `thelisaann` marketing scraper; only /vod/ is
// this one. Both matching would make which scraper runs depend on registration
// order, which ForURL resolves silently.
func TestMatchesURLIsScopedToVod(t *testing.T) {
	s := New()
	yes := []string{
		"https://thelisaann.com/vod/",
		"https://www.thelisaann.com/vod/",
		"http://thelisaann.com/vod",
		"https://thelisaann.com/vod/categories/movies.html",
		"https://thelisaann.com/vod/categories/movies_3_d.html",
		"https://thelisaann.com/vod/models/lisa-ann.html",
		"https://thelisaann.com/vod/scenes/Some-Scene_vids.html",
	}
	no := []string{
		"https://www.thelisaann.com/",
		"https://www.thelisaann.com",
		"https://www.thelisaann.com/updates/page_2.html",
		"https://thelisaann.com/vodka/",
		"https://example.com/vod/",
		"https://thelisaann.com.evil.test/vod/",
	}
	for _, u := range yes {
		if !s.MatchesURL(u) {
			t.Errorf("MatchesURL(%q) = false, want true", u)
		}
	}
	for _, u := range no {
		if s.MatchesURL(u) {
			t.Errorf("MatchesURL(%q) = true, want false", u)
		}
	}
}

// One card, in the shape the live tour serves it.
const cardHTML = `
<div class="category_listing_wrapper_updates">
<div class="update_details" data-setid="1081">
	<a  href="https://thelisaann.com/vod/scenes/I-Am-Your-Stay-At-Home-Milf-Video_vids.html" >
		<img id="set-target-1081" alt="" class="update_thumb thumbs stdimage"
		     src0_1x="/vod/content//contentthumbs/36/64/3664-1x.jpg"
		     src0_4x="/vod/content//contentthumbs/36/64/3664-4x.jpg" cnt="2" v="0" />	</a>
	<div class="cart_buttons cart_setid_1081">
		<a href="javascript:rent_buy_options('1081')"><div class="buy_button">Buy $4.99 - $5.99</div></a>
	</div>
	<div id="packageinfo_1081" style="display:none;" data-title="I Am Your Stay At Home Milf Video" data-redirect="https://thelisaann.com/vod/scenes/I-Am-Your-Stay-At-Home-Milf-Video.html">{"rent":[{"Label":"HD","FullPrice":"4.99","PurchaseType":"Rent","Price":"4.99"}],"buy":[{"Label":"HD","FullPrice":"5.99","PurchaseType":"Buy","Price":"5.99"}],"all":[{"Label":"HD","Price":"4.99"},{"Label":"HD","Price":"5.99"}]}</div>
</div>
</div>`

func TestParseCards(t *testing.T) {
	cards := parseCards(cardHTML)
	if len(cards) != 1 {
		t.Fatalf("got %d cards, want 1", len(cards))
	}
	c := cards[0]
	if c.id != "1081" {
		t.Errorf("id = %q", c.id)
	}
	if c.path != "/vod/scenes/I-Am-Your-Stay-At-Home-Milf-Video_vids.html" {
		t.Errorf("path = %q — must be the _vids.html detail page", c.path)
	}
	if c.title != "I Am Your Stay At Home Milf Video" {
		t.Errorf("title = %q", c.title)
	}
	// The 4x thumbnail resolves on this site, unlike some other Elevated X tours.
	if c.thumb != "/vod/content//contentthumbs/36/64/3664-4x.jpg" {
		t.Errorf("thumb = %q, want the 4x variant", c.thumb)
	}
	if c.rent != 4.99 || c.buy != 5.99 {
		t.Errorf("rent/buy = %v/%v, want 4.99/5.99", c.rent, c.buy)
	}
}

// The visible label is a range across both purchase types ("Buy $4.99 - $5.99"),
// so reading it would file the rental price as the purchase price.
func TestParsePricesUsesTheJSONNotTheLabel(t *testing.T) {
	rent, buy := parsePrices(cardHTML)
	if rent != 4.99 || buy != 5.99 {
		t.Fatalf("rent/buy = %v/%v, want 4.99/5.99", rent, buy)
	}
	if r, b := parsePrices(`<div class="update_details" data-setid="1">no packages</div>`); r != 0 || b != 0 {
		t.Errorf("missing package JSON gave %v/%v, want 0/0", r, b)
	}
}

func TestParseCardsSkipsCardsWithoutASceneLink(t *testing.T) {
	body := `<div class="update_details" data-setid="99"><a href="join.php">Join</a></div>` + cardHTML
	if got := parseCards(body); len(got) != 1 || got[0].id != "1081" {
		t.Fatalf("got %+v, want only the real scene card", got)
	}
}

const detailHTML = `<html><head><title>I Am Your Stay At Home Milf Video</title></head><body>
<div class="gallery_info"><div class="table"><div class="row">
	<div class="cell update_date"> 06/13/2025 </div>
	<div class="cell avg_rating"> 4.7 </div>
</div></div>
<span class="update_description">
	I Am Your Stay At Home MILF video, hot naughty strip tease &amp; more.
</span>
<span class="update_models">
	Featuring: <a href="https://thelisaann.com/vod/models/lisa-ann.html">Lisa Ann</a>
</span>
<span class="update_tags">
	Tags:&nbsp;<a href="https://thelisaann.com/vod/categories/Butt.html">Butt</a><a href="https://thelisaann.com/vod/categories/TrimmedPussy.html">Trimmed Pussy</a>
</span>
</body></html>`

func TestParseDetail(t *testing.T) {
	d := parseDetail(detailHTML)
	if d.date != "06/13/2025" {
		t.Errorf("date = %q", d.date)
	}
	if d.description != "I Am Your Stay At Home MILF video, hot naughty strip tease & more." {
		t.Errorf("description = %q", d.description)
	}
	if len(d.performers) != 1 || d.performers[0] != "Lisa Ann" {
		t.Errorf("performers = %v — the 'Featuring:' label is not an anchor and must not leak in", d.performers)
	}
	if len(d.tags) != 2 || d.tags[0] != "Butt" || d.tags[1] != "Trimmed Pussy" {
		t.Errorf("tags = %v", d.tags)
	}
}

func TestParseDate(t *testing.T) {
	got, ok := parseDate("06/13/2025")
	if !ok || got.Format("2006-01-02") != "2025-06-13" {
		t.Errorf("parseDate = %v %v, want 2025-06-13 (MM/DD/YYYY)", got, ok)
	}
	if _, ok := parseDate("13/06/2025"); ok {
		t.Error("parseDate accepted DD/MM/YYYY")
	}
	if _, ok := parseDate(""); ok {
		t.Error("parseDate accepted an empty string")
	}
}

// ---- end to end ----

type tourServer struct {
	*httptest.Server
	detailHits atomic.Int32
}

func newTourServer(t *testing.T, pages [][]string) *tourServer {
	t.Helper()
	ts := &tourServer{}
	mux := http.NewServeMux()
	mux.HandleFunc("/vod/categories/", func(w http.ResponseWriter, r *http.Request) {
		var n int
		_, _ = fmt.Sscanf(strings.TrimPrefix(r.URL.Path, "/vod/categories/movies_"), "%d_d.html", &n)
		if n < 1 || n > len(pages) {
			return // out of range: no cards
		}
		var sb strings.Builder
		for _, id := range pages[n-1] {
			fmt.Fprintf(&sb, `<div class="update_details" data-setid="%s">
				<a href="/vod/scenes/scene%s_vids.html"><img src0_4x="/vod/content/t%s-4x.jpg" /></a>
				<div id="packageinfo_%s" data-title="Scene %s">{"rent":[{"Price":"2.99"}],"buy":[{"Price":"7.99"}]}</div>
				</div>`, id, id, id, id, id)
		}
		_, _ = fmt.Fprint(w, sb.String())
	})
	mux.HandleFunc("/vod/scenes/", func(w http.ResponseWriter, r *http.Request) {
		ts.detailHits.Add(1)
		id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/vod/scenes/scene"), "_vids.html")
		_, _ = fmt.Fprintf(w, `<html><head><title>Scene %s</title></head><body>
			<div class="cell update_date"> 06/1%s/2025 </div>
			<span class="update_description">Desc %s</span>
			<span class="update_models">Featuring: <a href="/vod/models/lisa-ann.html">Lisa Ann</a></span>
			<span class="update_tags">Tags:&nbsp;<a href="/vod/categories/Solo.html">Solo</a></span>
			</body></html>`, id, id, id)
	})
	ts.Server = httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts
}

func newTestScraper(ts *tourServer) *Scraper {
	s := New()
	s.Client = ts.Client()
	s.baseOverride = ts.URL
	return s
}

func collect(t *testing.T, ch <-chan scraper.SceneResult) ([]models.Scene, []error, bool) {
	t.Helper()
	var scenes []models.Scene
	var errs []error
	stopped := false
	for r := range ch {
		switch r.Kind {
		case scraper.KindScene:
			scenes = append(scenes, r.Scene)
		case scraper.KindError:
			errs = append(errs, r.Err)
		case scraper.KindStoppedEarly:
			stopped = true
		}
	}
	return scenes, errs, stopped
}

func TestListScenes(t *testing.T) {
	ts := newTourServer(t, [][]string{{"1", "2"}, {"3"}})
	s := newTestScraper(ts)

	ch, err := s.ListScenes(context.Background(), "https://thelisaann.com/vod/", scraper.ListOpts{Workers: 2})
	if err != nil {
		t.Fatal(err)
	}
	scenes, errs, _ := collect(t, ch)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(scenes) != 3 {
		t.Fatalf("got %d scenes, want 3 across two pages", len(scenes))
	}

	byID := map[string]models.Scene{}
	for _, sc := range scenes {
		byID[sc.ID] = sc
		// A scraper following the absolute URLs the real pages embed would hit
		// production from an offline test.
		if !strings.HasPrefix(sc.URL, ts.URL) {
			t.Errorf("scene %s URL %q left the test server", sc.ID, sc.URL)
		}
		if !strings.HasPrefix(sc.Thumbnail, ts.URL+"/vod/content/") {
			t.Errorf("scene %s thumbnail %q not resolved against the site base", sc.ID, sc.Thumbnail)
		}
		if sc.Studio != studioName || sc.SiteID != siteID {
			t.Errorf("scene %s = %q/%q", sc.ID, sc.Studio, sc.SiteID)
		}
	}
	one := byID["1"]
	if one.Title != "Scene 1" {
		t.Errorf("title = %q", one.Title)
	}
	if one.Description != "Desc 1" {
		t.Errorf("description = %q", one.Description)
	}
	if len(one.Performers) != 1 || one.Performers[0] != "Lisa Ann" {
		t.Errorf("performers = %v", one.Performers)
	}
	if one.Date.Format("2006-01-02") != "2025-06-11" {
		t.Errorf("date = %v", one.Date)
	}
	if len(one.PriceHistory) != 1 {
		t.Fatalf("price snapshots = %d, want 1", len(one.PriceHistory))
	}
	p := one.PriceHistory[0]
	// Purchase price only: the rental is a standing second tier, not a sale,
	// and recording it as one would make LowestPrice track rentals.
	if p.Regular != 7.99 {
		t.Errorf("Regular = %v, want the 7.99 purchase price", p.Regular)
	}
	if p.Discounted != 0 || p.IsOnSale {
		t.Errorf("Discounted/IsOnSale = %v/%v, want the rental left out of the sale fields", p.Discounted, p.IsOnSale)
	}
	if one.LowestPrice != 7.99 {
		t.Errorf("LowestPrice = %v, want 7.99", one.LowestPrice)
	}
}

// The listing is the CMS's newest-first sort, so a stored id means the rest is
// already held.
func TestKnownIDStopsEarly(t *testing.T) {
	ts := newTourServer(t, [][]string{{"1", "2"}, {"3"}})
	s := newTestScraper(ts)

	ch, err := s.ListScenes(context.Background(), "https://thelisaann.com/vod/",
		scraper.ListOpts{Workers: 2, KnownIDs: map[string]bool{"2": true}})
	if err != nil {
		t.Fatal(err)
	}
	scenes, errs, stopped := collect(t, ch)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if !stopped {
		t.Error("no StoppedEarly emitted")
	}
	if len(scenes) != 1 || scenes[0].ID != "1" {
		t.Fatalf("got %v, want only the newer scene 1", scenes)
	}
	if got := ts.detailHits.Load(); got != 1 {
		t.Errorf("detail fetches = %d, want 1 — a known scene must cost no request", got)
	}
}

func TestModelURLWalksTheModelListing(t *testing.T) {
	var gotPaths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPaths = append(gotPaths, r.URL.Path)
	}))
	defer srv.Close()

	s := New()
	s.Client = srv.Client()
	s.baseOverride = srv.URL

	ch, err := s.ListScenes(context.Background(),
		"https://thelisaann.com/vod/models/lisa-ann.html", scraper.ListOpts{Workers: 1})
	if err != nil {
		t.Fatal(err)
	}
	collect(t, ch)

	if len(gotPaths) == 0 || gotPaths[0] != "/vod/models/lisa-ann_1_d.html" {
		t.Errorf("first request = %v, want the paginated model listing", gotPaths)
	}
}

func TestContextCancellationStopsTheScrape(t *testing.T) {
	ts := newTourServer(t, [][]string{{"1", "2", "3"}})
	s := newTestScraper(ts)
	ctx, cancel := context.WithCancel(context.Background())
	ch, err := s.ListScenes(ctx, "https://thelisaann.com/vod/", scraper.ListOpts{Workers: 1})
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	for range ch { //nolint:revive // draining is the point
	}
}

// A scene with no purchase tier still gets a price: there the rental is what it
// costs, not a discount on something else.
func TestRentOnlySceneUsesTheRentalAsRegular(t *testing.T) {
	c := listCard{id: "1", rent: 2.99}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, `<html><head><title>T</title></head><body></body></html>`)
	}))
	defer srv.Close()

	s := New()
	s.Client = srv.Client()
	sc, err := s.buildScene(context.Background(), "https://thelisaann.com/vod/", srv.URL, c)
	if err != nil {
		t.Fatal(err)
	}
	if len(sc.PriceHistory) != 1 || sc.PriceHistory[0].Regular != 2.99 {
		t.Fatalf("price history = %+v, want one 2.99 snapshot", sc.PriceHistory)
	}
	if sc.LowestPrice != 2.99 {
		t.Errorf("LowestPrice = %v, want 2.99", sc.LowestPrice)
	}
}
