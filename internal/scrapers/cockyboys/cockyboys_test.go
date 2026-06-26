package cockyboys

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wasylq/FSS/scraper"
)

// ---- fixtures ----

func listingHTML() string {
	return `<html><body>
<div class="grid">
  <div class="cell"><a href="/scenes/the-first-scene.html" class="abso" title="The First Scene">x</a></div>
  <div class="cell"><a href="/scenes/second-scene.html?type=vids" class="abso" title="Second &amp; Best">y</a></div>
  <div class="cell"><a href="/scenes/the-first-scene.html" class="abso" title="The First Scene">dup</a></div>
</div>
</body></html>`
}

func detailHTML(thumbID, title, released string) string {
	return fmt.Sprintf(`<html><head>
<meta property="og:image" content="https://cockyboys.com/contentthumbs/%s.jpg">
<meta property="og:title" content="%s">
</head><body>
<h1 class="title">%s</h1>
<p><strong>Released:</strong> %s</p>
<p><strong>Categorized Under:</strong>
  <a href="/categories/bareback.html">Bareback</a>,
  <a href="/categories/anal.html">Anal</a>
</p>
<div class="movieModels__grid">
  <a class="name gothamy" href="/models/john-doe.html" title="John Doe">John Doe</a>
  <a class="name gothamy" href="/models/jane-roe.html" title="Jane Roe">Jane Roe</a>
</div>
</body></html>`, thumbID, title, title, released)
}

// ---- TestMatchesURL ----

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url   string
		match bool
	}{
		{"https://cockyboys.com/categories/movies_1_d.html", true},
		{"https://www.cockyboys.com/scenes/foo.html", true},
		{"https://example.com/x", false},
		{"", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.match {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.match)
		}
	}
}

// ---- TestSceneSlug ----

func TestSceneSlug(t *testing.T) {
	cases := map[string]string{
		"/scenes/the-first-scene.html":        "the-first-scene",
		"/scenes/second-scene.html?type=vids": "second-scene",
		"/scenes/x.html":                      "x",
		"":                                    "",
	}
	for in, want := range cases {
		if got := sceneSlug(in); got != want {
			t.Errorf("sceneSlug(%q) = %q, want %q", in, got, want)
		}
	}
}

// ---- TestCleanText ----

func TestCleanText(t *testing.T) {
	got := cleanText(`  <span>Hello&amp;</span>   World  `)
	if got != "Hello& World" {
		t.Errorf("cleanText = %q, want %q", got, "Hello& World")
	}
}

// ---- TestFetchListing ----

func TestFetchListing(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, listingHTML())
	}))
	defer ts.Close()

	s := &Scraper{Client: ts.Client()}
	items, err := s.fetchListing(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("fetchListing error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2 (dup dropped): %+v", len(items), items)
	}
	if items[0].slug != "the-first-scene" || items[0].title != "The First Scene" {
		t.Errorf("item0 = %+v", items[0])
	}
	if items[1].slug != "second-scene" || items[1].title != "Second & Best" {
		t.Errorf("item1 = %+v", items[1])
	}
	if items[0].url != siteBase+"/scenes/the-first-scene.html" {
		t.Errorf("item0.url = %q", items[0].url)
	}
}

// ---- TestToScene (detail parse) ----

func TestToScene(t *testing.T) {
	orig := siteBase
	defer func() { siteBase = orig }()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, detailHTML("4521", "The First Scene", "03/14/2024"))
	}))
	defer ts.Close()
	siteBase = ts.URL

	s := &Scraper{Client: ts.Client()}
	it := listItem{slug: "the-first-scene", url: ts.URL + "/scenes/the-first-scene.html", title: "The First Scene"}
	now := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	sc := s.toScene(context.Background(), "studioURL", it, now)

	// og:image content id overrides the slug ID.
	if sc.ID != "4521" {
		t.Errorf("ID = %q, want 4521 (from contentthumbs)", sc.ID)
	}
	if sc.SiteID != siteID {
		t.Errorf("SiteID = %q", sc.SiteID)
	}
	if sc.Title != "The First Scene" {
		t.Errorf("Title = %q", sc.Title)
	}
	if sc.Thumbnail != "https://cockyboys.com/contentthumbs/4521.jpg" {
		t.Errorf("Thumbnail = %q", sc.Thumbnail)
	}
	wantDate := time.Date(2024, 3, 14, 0, 0, 0, 0, time.UTC)
	if !sc.Date.Equal(wantDate) {
		t.Errorf("Date = %v, want %v", sc.Date, wantDate)
	}
	if strings.Join(sc.Tags, ",") != "Bareback,Anal" {
		t.Errorf("Tags = %v, want [Bareback Anal]", sc.Tags)
	}
	if strings.Join(sc.Performers, ",") != "John Doe,Jane Roe" {
		t.Errorf("Performers = %v", sc.Performers)
	}
}

// ---- TestListScenes (end-to-end) ----

func TestListScenes(t *testing.T) {
	orig := siteBase
	defer func() { siteBase = orig }()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "movies_1_d.html"):
			_, _ = fmt.Fprint(w, listingHTML())
		case strings.HasPrefix(r.URL.Path, "/scenes/the-first-scene"):
			_, _ = fmt.Fprint(w, detailHTML("4521", "The First Scene", "03/14/2024"))
		case strings.HasPrefix(r.URL.Path, "/scenes/second-scene"):
			_, _ = fmt.Fprint(w, detailHTML("4522", "Second & Best", "04/01/2024"))
		default:
			// movies_2_d.html etc. repeat the listing -> dedup yields nothing -> Done.
			_, _ = fmt.Fprint(w, listingHTML())
		}
	}))
	defer ts.Close()
	siteBase = ts.URL

	s := &Scraper{Client: ts.Client()}
	ch, err := s.ListScenes(context.Background(), "studioURL", scraper.ListOpts{})
	if err != nil {
		t.Fatalf("ListScenes error: %v", err)
	}
	got := map[string]string{}
	for r := range ch {
		if r.Err != nil {
			t.Errorf("unexpected error: %v", r.Err)
			continue
		}
		if r.Kind == scraper.KindScene {
			got[r.Scene.ID] = r.Scene.Title
		}
	}
	if len(got) != 2 {
		t.Fatalf("got %d scenes, want 2: %v", len(got), got)
	}
	if got["4521"] != "The First Scene" || got["4522"] != "Second & Best" {
		t.Errorf("scenes = %v", got)
	}
}
