package scissorgoddess

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
	"github.com/Wasylq/FSS/scraper"
)

func readFixture(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/products.json")
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	return b
}

func fixtureProducts(t *testing.T) []wpProduct {
	t.Helper()
	var ps []wpProduct
	if err := json.Unmarshal(readFixture(t), &ps); err != nil {
		t.Fatalf("decoding fixture: %v", err)
	}
	return ps
}

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url   string
		match bool
	}{
		{"https://scissorgoddess.net", true},
		{"https://scissorgoddess.net/", true},
		{"https://www.scissorgoddess.net/product/the-session/", true},
		{"https://scissorgoddess.com/", false},
		{"", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.match {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.match)
		}
	}
}

func TestID(t *testing.T) {
	if got := New().ID(); got != siteID {
		t.Errorf("ID() = %q, want %q", got, siteID)
	}
}

func TestToScene(t *testing.T) {
	ps := fixtureProducts(t)
	if len(ps) == 0 {
		t.Fatal("fixture is empty")
	}
	now := time.Now().UTC()

	sc := toScene("https://scissorgoddess.net", ps[0], now)

	if sc.ID != strconv.Itoa(ps[0].ID) {
		t.Errorf("ID = %q", sc.ID)
	}
	if sc.SiteID != siteID || sc.Studio != studioName {
		t.Errorf("SiteID = %q, Studio = %q", sc.SiteID, sc.Studio)
	}
	if sc.Title == "" {
		t.Error("Title is empty")
	}
	// WP renders typographic entities into titles; they must be decoded.
	if strings.Contains(sc.Title, "&#") || strings.Contains(sc.Title, "&amp;") {
		t.Errorf("Title still holds HTML entities: %q", sc.Title)
	}
	if sc.Description == "" {
		t.Error("Description is empty")
	}
	if strings.Contains(sc.Description, "<") {
		t.Errorf("Description still holds markup: %q", sc.Description)
	}
	if sc.Date.IsZero() {
		t.Error("Date is zero")
	}
	if sc.URL == "" {
		t.Error("URL is empty")
	}
	if !sc.ScrapedAt.Equal(now) {
		t.Errorf("ScrapedAt = %v", sc.ScrapedAt)
	}
}

// Only three of the embedded taxonomies are scene metadata. product_cat is the
// storefront section ("Video") and product_brand the publisher — neither may
// leak into performers, categories or tags.
func TestToSceneTaxonomyMapping(t *testing.T) {
	p := wpProduct{ID: 1, Title: wpRendered{Rendered: "T"}}
	p.Embedded.Terms = [][]wpTerm{
		{{Name: "Media Solutions Inc", Taxonomy: "product_brand"}},
		{{Name: "Video", Taxonomy: "product_cat"}},
		{{Name: "anal", Taxonomy: "product_tag"}, {Name: "femdom", Taxonomy: "product_tag"}},
		{{Name: "Goddess Rapture", Taxonomy: "model"}},
		{{Name: "Female Domination", Taxonomy: "genre"}, {Name: "Pegging", Taxonomy: "genre"}},
	}

	sc := toScene("https://scissorgoddess.net", p, time.Now())

	if !slices.Equal(sc.Performers, []string{"Goddess Rapture"}) {
		t.Errorf("Performers = %v", sc.Performers)
	}
	if !slices.Equal(sc.Categories, []string{"Female Domination", "Pegging"}) {
		t.Errorf("Categories = %v", sc.Categories)
	}
	if !slices.Equal(sc.Tags, []string{"anal", "femdom"}) {
		t.Errorf("Tags = %v", sc.Tags)
	}
	for _, unwanted := range []string{"Video", "Media Solutions Inc"} {
		if slices.Contains(sc.Tags, unwanted) ||
			slices.Contains(sc.Categories, unwanted) ||
			slices.Contains(sc.Performers, unwanted) {
			t.Errorf("storefront term %q leaked into scene metadata: %+v", unwanted, sc)
		}
	}
}

func TestToSceneUsesFeaturedMedia(t *testing.T) {
	p := wpProduct{ID: 2, Title: wpRendered{Rendered: "T"}}
	p.Embedded.FeaturedMedia = []struct {
		SourceURL string `json:"source_url"`
	}{{SourceURL: "https://scissorgoddess.net/wp-content/uploads/a.jpg"}}

	if got := toScene("x", p, time.Now()).Thumbnail; got != "https://scissorgoddess.net/wp-content/uploads/a.jpg" {
		t.Errorf("Thumbnail = %q", got)
	}
}

func TestCleanText(t *testing.T) {
	cases := map[string]string{
		"The Session &#8211; Rapture":    "The Session – Rapture",
		"Pegging &#038; Humiliation":     "Pegging & Humiliation",
		"<p>Some  text</p>\n<p>more</p>": "<p>Some text</p> <p>more</p>",
		"":                               "",
	}
	for in, want := range cases {
		if got := cleanText(in); got != want {
			t.Errorf("cleanText(%q) = %q, want %q", in, got, want)
		}
	}
}

// ---- end-to-end ----

// newTestServer serves `total` products at perPage per page and answers HTTP
// 400 past the last page, the way WordPress does.
func newTestServer(t *testing.T, total int) *httptest.Server {
	t.Helper()
	tmpl := fixtureProducts(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/wp-json/wp/v2/product" {
			http.NotFound(w, r)
			return
		}
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		if page < 1 {
			page = 1
		}
		totalPages := (total + perPage - 1) / perPage
		if page > totalPages {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = fmt.Fprint(w, `{"code":"rest_post_invalid_page_number"}`)
			return
		}

		start := (page - 1) * perPage
		end := min(start+perPage, total)
		out := make([]wpProduct, 0, end-start)
		for i := start; i < end; i++ {
			p := tmpl[i%len(tmpl)]
			p.ID = 10000 + i
			out = append(out, p)
		}

		w.Header().Set("X-WP-Total", strconv.Itoa(total))
		w.Header().Set("X-WP-TotalPages", strconv.Itoa(totalPages))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(out)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func run(t *testing.T, srv *httptest.Server) []string {
	t.Helper()
	orig := siteBase
	siteBase = srv.URL
	t.Cleanup(func() { siteBase = orig })

	s := New()
	s.Client = srv.Client()

	ch, err := s.ListScenes(context.Background(), srv.URL, scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, sc := range testutil.CollectScenes(t, ch) {
		ids = append(ids, sc.ID)
	}
	return ids
}

func TestListScenes(t *testing.T) {
	ids := run(t, newTestServer(t, 250))
	if len(ids) != 250 {
		t.Fatalf("got %d scenes, want 250", len(ids))
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if seen[id] {
			t.Fatalf("duplicate scene ID %q", id)
		}
		seen[id] = true
	}
}

// At an exact multiple of the page size the loop asks for one page past the
// end and WP answers 400. That must end the walk, not fail the run.
func TestListScenesExactPageMultiple(t *testing.T) {
	if ids := run(t, newTestServer(t, 200)); len(ids) != 200 {
		t.Fatalf("got %d scenes, want 200", len(ids))
	}
}

func TestListScenesSinglePage(t *testing.T) {
	if ids := run(t, newTestServer(t, 2)); len(ids) != 2 {
		t.Fatalf("got %d scenes, want 2", len(ids))
	}
}

// A first-page failure is a real error, not the end of the listing.
func TestFirstPageErrorIsReported(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	orig := siteBase
	siteBase = srv.URL
	defer func() { siteBase = orig }()

	s := New()
	s.Client = srv.Client()

	ch, err := s.ListScenes(context.Background(), srv.URL, scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}
	sawErr := false
	for res := range ch {
		if res.Kind == scraper.KindError {
			sawErr = true
		}
	}
	if !sawErr {
		t.Error("a page-1 failure produced no error result")
	}
}
