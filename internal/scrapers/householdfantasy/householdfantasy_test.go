package householdfantasy

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
	"github.com/Wasylq/FSS/scraper"
)

func readFixture(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/post.html")
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	return b
}

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url   string
		match bool
	}{
		{"https://householdfantasy.com", true},
		{"https://householdfantasy.com/", true},
		{"https://www.householdfantasy.com/cuban-in-the-cupboard/", true},
		{"http://householdfantasy.com/post-sitemap.xml", true},
		{"https://householdfantasy.net/", false},
		{"https://example.com/householdfantasy.com", false},
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

// TestParsePage covers the whole extraction against a real post.
func TestParsePage(t *testing.T) {
	now := time.Now().UTC()
	const pageURL = "https://householdfantasy.com/cuban-in-the-cupboard/"

	scene, skip, err := parsePage("https://householdfantasy.com", pageURL, readFixture(t), now)
	if err != nil {
		t.Fatalf("parsePage: %v", err)
	}
	if skip {
		t.Fatal("post was skipped; every sitemap entry is a scene")
	}

	// The numeric WordPress post id comes from the body class, not a shortlink
	// meta tag — the site emits none.
	if scene.ID != "117" {
		t.Errorf("ID = %q, want 117", scene.ID)
	}
	if scene.SiteID != siteID {
		t.Errorf("SiteID = %q", scene.SiteID)
	}
	if scene.Studio != studioName {
		t.Errorf("Studio = %q", scene.Studio)
	}
	// The " - Household Fantasy" suffix must be stripped from the og:title.
	if scene.Title != "Cuban in The Cupboard" {
		t.Errorf("Title = %q, want %q", scene.Title, "Cuban in The Cupboard")
	}
	if scene.URL != pageURL {
		t.Errorf("URL = %q", scene.URL)
	}
	want := time.Date(2025, time.April, 25, 19, 34, 57, 0, time.UTC)
	if !scene.Date.Equal(want) {
		t.Errorf("Date = %v, want %v", scene.Date, want)
	}
	if !strings.HasPrefix(scene.Description, "After moving into your new place") {
		t.Errorf("Description = %q", scene.Description)
	}
	if scene.Thumbnail != "https://householdfantasy.com/wp-content/uploads/2025/04/011-serena-santos.jpg" {
		t.Errorf("Thumbnail = %q", scene.Thumbnail)
	}
	// Performers come from /tag/ anchors in the body.
	if !slices.Equal(scene.Performers, []string{"Serena Santos"}) {
		t.Errorf("Performers = %v, want [Serena Santos]", scene.Performers)
	}
	// Genres come from the JSON-LD articleSection list.
	wantCats := []string{"Big Ass", "Big Tits", "Creampie", "Latina", "Teen"}
	if !slices.Equal(scene.Categories, wantCats) {
		t.Errorf("Categories = %v, want %v", scene.Categories, wantCats)
	}
	if !scene.ScrapedAt.Equal(now) {
		t.Errorf("ScrapedAt = %v, want %v", scene.ScrapedAt, now)
	}
}

// The site has no VideoObject JSON-LD, so wputil's HasVideo flag is always
// false. If parsePage ever gated on it, every scene would be dropped.
func TestParsePageDoesNotGateOnHasVideo(t *testing.T) {
	body := readFixture(t)
	if strings.Contains(string(body), "VideoObject") {
		t.Fatal("fixture unexpectedly contains a VideoObject; this test no longer guards anything")
	}
	if _, skip, _ := parsePage("https://householdfantasy.com", "https://householdfantasy.com/x/", body, time.Now()); skip {
		t.Error("parsePage skipped a post with no VideoObject JSON-LD")
	}
}

func TestParsePerformers(t *testing.T) {
	cases := []struct {
		name string
		body string
		want []string
	}{
		{
			name: "multiple performers",
			body: `<a href="https://householdfantasy.com/tag/lola-cheeks/" class="elementor-post-info__terms-list-item">Lola Cheeks</a>
			       <a href="https://householdfantasy.com/tag/luxe-la-fox/" class="elementor-post-info__terms-list-item">Luxe La Fox</a>`,
			want: []string{"Lola Cheeks", "Luxe La Fox"},
		},
		{
			name: "duplicates collapse",
			body: `<a href="/tag/eva-nyx/" class="elementor-post-info__terms-list-item">Eva Nyx</a>
			       <a href="/tag/eva-nyx/" class="elementor-post-info__terms-list-item">Eva Nyx</a>`,
			want: []string{"Eva Nyx"},
		},
		{
			name: "entities are unescaped",
			body: `<a href="/tag/a-b/" class="elementor-post-info__terms-list-item">A &amp; B</a>`,
			want: []string{"A & B"},
		},
		{
			// A plain /tag/ link elsewhere on the page is not a performer
			// credit — only the terms-list-item anchors are.
			name: "unrelated tag links ignored",
			body: `<a href="/tag/whatever/">Whatever</a>`,
			want: nil,
		},
		{
			name: "none",
			body: `<p>no performers here</p>`,
			want: nil,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := parsePerformers([]byte(c.body))
			if !slices.Equal(got, c.want) {
				t.Errorf("parsePerformers = %v, want %v", got, c.want)
			}
		})
	}
}

// TestIDFallsBackToSlug covers a post whose body class lost the postid-N token.
func TestIDFallsBackToSlug(t *testing.T) {
	body := []byte(`<html><head><meta property="og:title" content="A Scene - Household Fantasy" /></head><body class="single post">x</body></html>`)
	scene, _, err := parsePage("https://householdfantasy.com", "https://householdfantasy.com/a-scene/", body, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if scene.ID != "a-scene" {
		t.Errorf("ID = %q, want the URL slug %q", scene.ID, "a-scene")
	}
}

// ---- end-to-end over the sitemap ----

func TestListScenes(t *testing.T) {
	post := readFixture(t)

	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/post-sitemap.xml" {
			w.Header().Set("Content-Type", "application/xml")
			_, _ = fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>%s/cuban-in-the-cupboard/</loc></url>
  <url><loc>%s/red-light-green-light/</loc></url>
</urlset>`, srv.URL, srv.URL)
			return
		}
		_, _ = w.Write(post)
	}))
	defer srv.Close()

	orig := siteBase
	siteBase = srv.URL
	defer func() { siteBase = orig }()

	s := New()
	s.client = srv.Client()

	ch, err := s.ListScenes(context.Background(), srv.URL, scraper.ListOpts{Workers: 1})
	if err != nil {
		t.Fatal(err)
	}
	scenes := testutil.CollectScenes(t, ch)
	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2", len(scenes))
	}
	for _, sc := range scenes {
		if sc.Title == "" {
			t.Errorf("scene %s has no title", sc.ID)
		}
		if sc.Date.IsZero() {
			t.Errorf("scene %s has no date", sc.ID)
		}
	}
}

// TestPerformersPreferJSONLDKeywords pins the primary source. The elementor
// anchor is a theme detail; the JSON-LD keywords array is emitted by the SEO
// plugin and is the more durable signal.
func TestPerformersPreferJSONLDKeywords(t *testing.T) {
	body := []byte(`{"keywords":["From JSON-LD"]}
	<a href="/tag/from-anchor/" class="elementor-post-info__terms-list-item">From Anchor</a>`)
	if got := parsePerformers(body); !slices.Equal(got, []string{"From JSON-LD"}) {
		t.Errorf("parsePerformers = %v, want [From JSON-LD]", got)
	}
}

func TestPerformersFallBackToAnchors(t *testing.T) {
	body := []byte(`<a href="/tag/only-anchor/" class="elementor-post-info__terms-list-item">Only Anchor</a>`)
	if got := parsePerformers(body); !slices.Equal(got, []string{"Only Anchor"}) {
		t.Errorf("parsePerformers = %v, want [Only Anchor]", got)
	}
}

// The site emits articleSection as a JSON array; wputil only understands the
// string form, so the array parsing lives in this package.
func TestParseJSONArray(t *testing.T) {
	cases := []struct {
		name string
		body string
		want []string
	}{
		{"array", `"articleSection":["Big Ass","Teen"]`, []string{"Big Ass", "Teen"}},
		{"single", `"articleSection":["Solo"]`, []string{"Solo"}},
		{"empty", `"articleSection":[]`, nil},
		{"absent", `{"other":1}`, nil},
		{"entities unescaped", `"articleSection":["R&amp;B"]`, []string{"R&B"}},
		{"duplicates collapse", `"articleSection":["Teen","Teen"]`, []string{"Teen"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := parseJSONArray(articleSectionRe, []byte(c.body))
			if !slices.Equal(got, c.want) {
				t.Errorf("parseJSONArray = %v, want %v", got, c.want)
			}
		})
	}
}
