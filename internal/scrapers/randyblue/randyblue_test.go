package randyblue

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
<ul class="grid">
  <li><a href="/scenes/Alex-Kof-Serg-Shepard_vids.html">x</a></li>
  <li><a href="/scenes/Beau-Butler-Luiz-Silva_vids.html">y</a></li>
  <li><a href="/scenes/Alex-Kof-Serg-Shepard_vids.html">dup</a></li>
</ul>
</body></html>`
}

func detailHTML(name, contentID, upload string) string {
	return fmt.Sprintf(`<html><head>
<meta itemprop="name" content="%s" />
<meta itemprop="description" content="A passionate Ukrainian pairing.<br />Hot scene." />
<meta itemprop="thumbnailUrl" content="https://cdnhwct.randyblue.com/content/contentthumbs/55/09/%s-1x.jpg" />
<meta itemprop="uploadDate" content="%s" />
</head><body>
<ul class="scene-models-list"><li><a data-track="PORNSTAR_NAME" href="/models/Alex-Kof.html">Alex Kof</a>&nbsp;&amp;&nbsp; </li><li><a href="/models/Serg-Shepard.html">Serg Shepard</a></li></ul>
<ul class="scene-tags"><li><a href="/categories/anal.html">Anal</a></li><li><a href="/categories/bareback.html">Bareback Video</a></li></ul>
</body></html>`, name, contentID, upload)
}

// ---- TestMatchesURL ----

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url   string
		match bool
	}{
		{"https://www.randyblue.com/categories/videos_1_d.html", true},
		{"https://randyblue.com/scenes/Foo_vids.html", true},
		{"https://example.com/x", false},
		{"", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.match {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.match)
		}
	}
}

// ---- TestFetchListing ----

func TestFetchListing(t *testing.T) {
	orig := siteBase
	defer func() { siteBase = orig }()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, listingHTML())
	}))
	defer ts.Close()
	siteBase = ts.URL

	s := &Scraper{Client: ts.Client()}
	items, err := s.fetchListing(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("fetchListing error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2 (dup dropped): %+v", len(items), items)
	}
	if items[0].slug != "Alex-Kof-Serg-Shepard" {
		t.Errorf("item0 slug = %q", items[0].slug)
	}
	if items[0].url != siteBase+"/scenes/Alex-Kof-Serg-Shepard_vids.html" {
		t.Errorf("item0 url = %q", items[0].url)
	}
}

// ---- TestToScene ----

func TestToScene(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, detailHTML("Alex Kof &amp; Serg Shepard", "15509", "06/10/2026"))
	}))
	defer ts.Close()

	s := &Scraper{Client: ts.Client()}
	it := listItem{slug: "Alex-Kof-Serg-Shepard", url: ts.URL + "/scenes/Alex-Kof-Serg-Shepard_vids.html"}
	now := time.Date(2026, 6, 26, 0, 0, 0, 0, time.UTC)
	sc := s.toScene(context.Background(), "studioURL", it, now)

	if sc.ID != "15509" {
		t.Errorf("ID = %q, want 15509 (from contentthumbs)", sc.ID)
	}
	if sc.Title != "Alex Kof & Serg Shepard" {
		t.Errorf("Title = %q", sc.Title)
	}
	wantDate := time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC)
	if !sc.Date.Equal(wantDate) {
		t.Errorf("Date = %v, want %v", sc.Date, wantDate)
	}
	if strings.Join(sc.Performers, ",") != "Alex Kof,Serg Shepard" {
		t.Errorf("Performers = %v", sc.Performers)
	}
	if strings.Join(sc.Tags, ",") != "Anal,Bareback Video" {
		t.Errorf("Tags = %v", sc.Tags)
	}
	if !strings.Contains(sc.Description, "Ukrainian pairing") || strings.Contains(sc.Description, "<br") {
		t.Errorf("Description = %q", sc.Description)
	}
}

// ---- TestListScenes (end-to-end) ----

func TestListScenes(t *testing.T) {
	orig := siteBase
	defer func() { siteBase = orig }()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "videos_1_d.html"):
			_, _ = fmt.Fprint(w, listingHTML())
		case strings.Contains(r.URL.Path, "Alex-Kof"):
			_, _ = fmt.Fprint(w, detailHTML("Alex Kof & Serg Shepard", "15509", "06/10/2026"))
		case strings.Contains(r.URL.Path, "Beau-Butler"):
			_, _ = fmt.Fprint(w, detailHTML("Beau Butler & Luiz Silva", "15600", "06/03/2026"))
		default:
			// empty listing -> Done.
			_, _ = fmt.Fprint(w, "<html></html>")
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
	if got["15509"] != "Alex Kof & Serg Shepard" || got["15600"] != "Beau Butler & Luiz Silva" {
		t.Errorf("scenes = %v", got)
	}
}
