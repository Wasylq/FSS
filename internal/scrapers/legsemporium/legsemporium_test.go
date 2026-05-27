package legsemporium

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wasylq/FSS/scraper"
)

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url   string
		match bool
	}{
		{"https://legsemporium.com", true},
		{"https://legsemporium.com/", true},
		{"https://www.legsemporium.com", true},
		{"https://legsemporium.com/product-category/madalaine", true},
		{"https://legsemporium.com/product-category/gymnasts", true},
		{"https://legsemporium.com/product/some-video", true},
		{"https://www.manyvids.com/Profile/123", false},
		{"https://example.com", false},
		{"", false},
	}
	for _, c := range cases {
		got := s.MatchesURL(c.url)
		if got != c.match {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.match)
		}
	}
}

func TestExtractSlug(t *testing.T) {
	cases := []struct {
		url  string
		want string
	}{
		{"https://legsemporium.com/product-category/madalaine", "madalaine"},
		{"https://legsemporium.com/product-category/gymnasts/", "gymnasts"},
		{"https://legsemporium.com/product-category/gymnasts/some-sub", "gymnasts/some-sub"},
		{"https://legsemporium.com", ""},
		{"https://legsemporium.com/", ""},
	}
	for _, c := range cases {
		got := extractSlug(c.url)
		if got != c.want {
			t.Errorf("extractSlug(%q) = %q, want %q", c.url, got, c.want)
		}
	}
}

func TestParseCount(t *testing.T) {
	cases := []struct {
		input string
		want  int
	}{
		{"123", 123},
		{"1,234", 1234},
		{"2.5k", 2500},
		{"10k", 10000},
		{"0", 0},
		{"", 0},
	}
	for _, c := range cases {
		got := parseCount(c.input)
		if got != c.want {
			t.Errorf("parseCount(%q) = %d, want %d", c.input, got, c.want)
		}
	}
}

func TestDecodeHTMLEntities(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"foo &amp; bar", "foo & bar"},
		{"&#039;hello&#039;", "'hello'"},
		{"a &lt; b &gt; c", "a < b > c"},
		{"&quot;quoted&quot;", `"quoted"`},
		{"no entities", "no entities"},
	}
	for _, c := range cases {
		got := decodeHTMLEntities(c.input)
		if got != c.want {
			t.Errorf("decodeHTMLEntities(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

func TestParseProductCards(t *testing.T) {
	html := `
<div data-id="101" data-title="Flexy Splits" data-price="9.99">
  <a href="https://legsemporium.com/product/flexy-splits" class="a-card">
    <img class="a-img" src="/uploads/thumb1.jpg" alt="Flexy Splits">
  </a>
  <div class="stats">
    <i class="icon-eye"></i> <span>1,234</span>
    <i class="icon-clap"></i> <span>56</span>
  </div>
</div>
<div data-id="102" data-title="High Kicks &amp; Splits" data-price="12.50">
  <a href="https://legsemporium.com/product/high-kicks" class="a-card">
    <img class="a-img" src="/uploads/thumb2.jpg" alt="High Kicks">
  </a>
  <div class="stats">
    <i class="icon-eye"></i> <span>2.5k</span>
    <i class="icon-clap"></i> <span>100</span>
  </div>
</div>`

	entries := parseProductCards(html, defaultBaseURL)
	if len(entries) != 2 {
		t.Fatalf("parseProductCards returned %d entries, want 2", len(entries))
	}

	e := entries[0]
	if e.id != "101" {
		t.Errorf("id = %q, want %q", e.id, "101")
	}
	if e.title != "Flexy Splits" {
		t.Errorf("title = %q, want %q", e.title, "Flexy Splits")
	}
	if e.price != 9.99 {
		t.Errorf("price = %f, want 9.99", e.price)
	}
	if e.url != "https://legsemporium.com/product/flexy-splits" {
		t.Errorf("url = %q", e.url)
	}
	if e.thumbnail != "https://legsemporium.com/uploads/thumb1.jpg" {
		t.Errorf("thumbnail = %q", e.thumbnail)
	}
	if e.views != 1234 {
		t.Errorf("views = %d, want 1234", e.views)
	}
	if e.likes != 56 {
		t.Errorf("likes = %d, want 56", e.likes)
	}

	e2 := entries[1]
	if e2.title != "High Kicks & Splits" {
		t.Errorf("title = %q, want %q", e2.title, "High Kicks & Splits")
	}
	if e2.views != 2500 {
		t.Errorf("views = %d, want 2500", e2.views)
	}
}

func TestParseProductCardsSalePrice(t *testing.T) {
	html := `
<div data-id="200" data-title="Sale Video" data-price="15.00">
  <a href="https://legsemporium.com/product/sale-video" class="a-card">
    <img class="a-img" src="/uploads/sale.jpg" alt="Sale">
  </a>
  <div class="price"><u>$15.00</u> <span class="u-cl-red">$10.00</span></div>
  <div class="stats">
    <i class="icon-eye"></i> <span>50</span>
    <i class="icon-clap"></i> <span>5</span>
  </div>
</div>`

	entries := parseProductCards(html, defaultBaseURL)
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	if entries[0].salePrice != 10.0 {
		t.Errorf("salePrice = %f, want 10.0", entries[0].salePrice)
	}
}

func TestDedup(t *testing.T) {
	entries := []productEntry{
		{id: "1", title: "A"},
		{id: "2", title: "B"},
		{id: "1", title: "A duplicate"},
		{id: "3", title: "C"},
	}
	got := dedup(entries)
	if len(got) != 3 {
		t.Fatalf("dedup returned %d entries, want 3", len(got))
	}
	if got[0].id != "1" || got[1].id != "2" || got[2].id != "3" {
		t.Errorf("dedup result = %v", got)
	}
}

func TestFetchDetail(t *testing.T) {
	detailHTML := `<html><body>
<nav class="o-breadcrumbs">
  <a class="o-breadcrumbs-link" href="/">Home</a>
  <a class="o-breadcrumbs-link" href="/product-category/gymnasts">Gymnasts</a>
  <a class="o-breadcrumbs-link" href="/product-category/madalaine">Madalaine</a>
</nav>
<div class="duration">Duration 12:34</div>
<a href="https://legsemporium.com/product-tag/flexible" class="a-tag">flexible</a>
<a href="https://legsemporium.com/product-tag/splits" class="a-tag">splits</a>
<video poster="https://cdn.legsemporium.com/poster.jpg"></video>
</body></html>`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, detailHTML)
	}))
	defer ts.Close()

	sess := &session{client: ts.Client()}
	e := productEntry{
		id:    "101",
		title: "Flexy Splits",
		url:   ts.URL + "/product/flexy-splits",
		price: 9.99,
	}

	scene, err := fetchDetail(context.Background(), sess, e, "https://legsemporium.com/product-category/madalaine")
	if err != nil {
		t.Fatalf("fetchDetail error: %v", err)
	}

	if scene.ID != "101" {
		t.Errorf("ID = %q", scene.ID)
	}
	if scene.SiteID != "legsemporium" {
		t.Errorf("SiteID = %q", scene.SiteID)
	}
	if scene.Duration != 754 {
		t.Errorf("Duration = %d, want 754 (12:34)", scene.Duration)
	}
	if len(scene.Tags) != 2 || scene.Tags[0] != "flexible" || scene.Tags[1] != "splits" {
		t.Errorf("Tags = %v, want [flexible splits]", scene.Tags)
	}
	if len(scene.Performers) != 1 || scene.Performers[0] != "Madalaine" {
		t.Errorf("Performers = %v, want [Madalaine]", scene.Performers)
	}
	if len(scene.Categories) != 1 || scene.Categories[0] != "Gymnasts" {
		t.Errorf("Categories = %v, want [Gymnasts]", scene.Categories)
	}
	if len(scene.PriceHistory) != 1 || scene.PriceHistory[0].Regular != 9.99 {
		t.Errorf("PriceHistory = %v", scene.PriceHistory)
	}
}

func TestFetchDetailSalePrice(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, "<html><body></body></html>")
	}))
	defer ts.Close()

	sess := &session{client: ts.Client()}
	e := productEntry{
		id:        "202",
		title:     "On Sale",
		url:       ts.URL + "/product/on-sale",
		price:     20.00,
		salePrice: 15.00,
	}

	scene, err := fetchDetail(context.Background(), sess, e, "https://legsemporium.com")
	if err != nil {
		t.Fatalf("fetchDetail error: %v", err)
	}

	if len(scene.PriceHistory) != 1 {
		t.Fatalf("PriceHistory len = %d, want 1", len(scene.PriceHistory))
	}
	snap := scene.PriceHistory[0]
	if !snap.IsOnSale {
		t.Error("IsOnSale = false, want true")
	}
	if snap.Discounted != 15.00 {
		t.Errorf("Discounted = %f, want 15.00", snap.Discounted)
	}
	if snap.DiscountPercent != 25 {
		t.Errorf("DiscountPercent = %d, want 25", snap.DiscountPercent)
	}
}

func TestFetchDetailPosterFallback(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `<html><video poster="/media/poster.jpg"></video></html>`)
	}))
	defer ts.Close()

	sess := &session{client: ts.Client(), base: defaultBaseURL}
	e := productEntry{
		id:    "303",
		title: "No Thumb",
		url:   ts.URL + "/product/no-thumb",
	}

	scene, err := fetchDetail(context.Background(), sess, e, defaultBaseURL)
	if err != nil {
		t.Fatalf("fetchDetail error: %v", err)
	}
	if scene.Thumbnail != defaultBaseURL+"/media/poster.jpg" {
		t.Errorf("Thumbnail = %q, want poster fallback", scene.Thumbnail)
	}
}

func TestListScenes(t *testing.T) {
	homepageHTML := `<html><script>this.csrf = "test-token-123"</script></html>`

	ajaxPage1 := ajaxResponse{
		HTMLMod: []struct {
			Value string `json:"value"`
		}{
			{Value: `
<div data-id="1" data-title="Video One" data-price="5.00">
  <a href="DETAIL_URL/product/video-one">
    <img class="a-img" src="/thumb1.jpg">
  </a>
  <i class="icon-eye"></i> <span>100</span>
  <i class="icon-clap"></i> <span>10</span>
</div>
<div data-id="2" data-title="Video Two" data-price="8.00">
  <a href="DETAIL_URL/product/video-two">
    <img class="a-img" src="/thumb2.jpg">
  </a>
  <i class="icon-eye"></i> <span>200</span>
  <i class="icon-clap"></i> <span>20</span>
</div>`},
		},
		IsLast:   true,
		NextPage: 0,
	}

	detailHTML := `<html><body>
<nav class="o-breadcrumbs">
  <a class="o-breadcrumbs-link" href="/">Home</a>
  <a class="o-breadcrumbs-link" href="/product-category/cats">Category</a>
  <a class="o-breadcrumbs-link" href="/product-category/model">Model</a>
</nav>
<div>Duration 5:30</div>
<a href="https://legsemporium.com/product-tag/legs" class="a-tag">legs</a>
</body></html>`

	var ts *httptest.Server
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/" || r.URL.Path == "":
			_, _ = fmt.Fprint(w, homepageHTML)
		case r.URL.Path == "/product-category" && r.Method == http.MethodPost:
			html := strings.ReplaceAll(ajaxPage1.HTMLMod[0].Value, "DETAIL_URL", ts.URL)
			resp := ajaxResponse{
				HTMLMod: []struct {
					Value string `json:"value"`
				}{{Value: html}},
				IsLast: true,
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		case r.URL.Path == "/product-category/testmodel":
			_, _ = fmt.Fprint(w, `<html><body>no subcategories here</body></html>`)
		case strings.HasPrefix(r.URL.Path, "/product/"):
			_, _ = fmt.Fprint(w, detailHTML)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	s := newWithBase(ts.URL)
	ch, err := s.ListScenes(context.Background(), ts.URL+"/product-category/testmodel", scraper.ListOpts{})
	if err != nil {
		t.Fatalf("ListScenes error: %v", err)
	}

	scenes := map[string]string{}
	for r := range ch {
		switch r.Kind {
		case scraper.KindError:
			t.Errorf("unexpected error: %v", r.Err)
		case scraper.KindScene:
			scenes[r.Scene.ID] = r.Scene.Title
		}
	}

	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2: %v", len(scenes), scenes)
	}
	if scenes["1"] != "Video One" {
		t.Errorf("scene 1 title = %q, want %q", scenes["1"], "Video One")
	}
	if scenes["2"] != "Video Two" {
		t.Errorf("scene 2 title = %q, want %q", scenes["2"], "Video Two")
	}
}

func TestListScenesKnownIDs(t *testing.T) {
	homepageHTML := `<html><script>this.csrf = "test-token"</script></html>`

	ajaxResp := ajaxResponse{
		HTMLMod: []struct {
			Value string `json:"value"`
		}{
			{Value: `
<div data-id="1" data-title="New" data-price="5.00">
  <a href="DETAIL_URL/product/new">
    <img class="a-img" src="/t1.jpg">
  </a>
  <i class="icon-eye"></i> <span>10</span>
  <i class="icon-clap"></i> <span>1</span>
</div>
<div data-id="2" data-title="Known" data-price="5.00">
  <a href="DETAIL_URL/product/known">
    <img class="a-img" src="/t2.jpg">
  </a>
  <i class="icon-eye"></i> <span>20</span>
  <i class="icon-clap"></i> <span>2</span>
</div>`},
		},
		IsLast: true,
	}

	var ts *httptest.Server
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/" || r.URL.Path == "":
			_, _ = fmt.Fprint(w, homepageHTML)
		case r.URL.Path == "/product-category" && r.Method == http.MethodPost:
			html := strings.ReplaceAll(ajaxResp.HTMLMod[0].Value, "DETAIL_URL", ts.URL)
			resp := ajaxResponse{
				HTMLMod: []struct {
					Value string `json:"value"`
				}{{Value: html}},
				IsLast: true,
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		case r.URL.Path == "/product-category/leaf":
			_, _ = fmt.Fprint(w, `<html><body>leaf page</body></html>`)
		case strings.HasPrefix(r.URL.Path, "/product/"):
			_, _ = fmt.Fprint(w, `<html><body></body></html>`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	s := newWithBase(ts.URL)
	ch, err := s.ListScenes(context.Background(), ts.URL+"/product-category/leaf", scraper.ListOpts{
		KnownIDs: map[string]bool{"2": true},
	})
	if err != nil {
		t.Fatalf("ListScenes error: %v", err)
	}

	var count int
	for r := range ch {
		if r.Kind == scraper.KindScene {
			count++
		}
	}
	if count != 1 {
		t.Errorf("got %d scenes, want 1 (early stop at known ID)", count)
	}
}
