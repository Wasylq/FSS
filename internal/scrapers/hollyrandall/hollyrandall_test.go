package hollyrandall

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

func listingHTML(base string) string {
	block := func(setid, slug, tsu string) string {
		return fmt.Sprintf(`
<div class="latestUpdateB" data-setid="%s">
  <div class="videoPic">
    <a href="%s/scenes/%s_vids.html"><img class="update_thumb"></a>
  </div>
  <div class="latestUpdateBinfo">
    <div id="packageinfo_%s" data-title="x">{"buy":[{"Id":7,"TSU":"%s 11:39:44","FullPrice":"9.99"}]}</div>
  </div>
</div>`, setid, base, slug, setid, tsu)
	}
	return `<html><body>` + block("1285", "built-for-speed", "2025-10-06") +
		block("1325", "festival-vibes", "2025-09-15") + `</body></html>`
}

func detailHTML(contentID, title, desc, modelSlug, modelName string) string {
	return fmt.Sprintf(`<html><head>
<meta property="og:title" content="Holly Randall - %s - Movies">
<meta property="og:image" content="https://hollyrandall.com/content/contentthumbs/85/14/%s-2x.jpg">
<meta property="og:description" content="%s">
</head><body>
<a href="/models/models.html">Models</a>
<a href="/models/%s.html">%s</a>
</body></html>`, title, contentID, desc, modelSlug, modelName)
}

// ---- TestMatchesURL ----

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url   string
		match bool
	}{
		{"https://hollyrandall.com/categories/updates_1_p.html", true},
		{"https://www.hollyrandall.com/scenes/foo_vids.html", true},
		{"https://example.com/x", false},
		{"", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.match {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.match)
		}
	}
}

// ---- TestCleanTitle ----

func TestCleanTitle(t *testing.T) {
	cases := map[string]string{
		"Holly Randall - Built for Speed - Movies": "Built for Speed",
		"Holly Randall - Solo - Movies":            "Solo",
		"Built for Speed":                          "Built for Speed",
	}
	for in, want := range cases {
		if got := cleanTitle(in); got != want {
			t.Errorf("cleanTitle(%q) = %q, want %q", in, got, want)
		}
	}
}

// ---- TestFetchListing ----

func TestFetchListing(t *testing.T) {
	orig := siteBase
	defer func() { siteBase = orig }()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, listingHTML(siteBase))
	}))
	defer ts.Close()
	siteBase = ts.URL

	s := &Scraper{Client: ts.Client()}
	items, err := s.fetchListing(context.Background(), ts.URL+"/categories/updates_1_p.html")
	if err != nil {
		t.Fatalf("fetchListing error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2: %+v", len(items), items)
	}
	if !strings.HasSuffix(items[0].url, "/scenes/built-for-speed_vids.html") {
		t.Errorf("item0.url = %q", items[0].url)
	}
	want := time.Date(2025, 10, 6, 0, 0, 0, 0, time.UTC)
	if !items[0].date.Equal(want) {
		t.Errorf("item0.date = %v, want %v", items[0].date, want)
	}
}

// ---- TestListScenes (end-to-end) ----

func TestListScenes(t *testing.T) {
	orig := siteBase
	defer func() { siteBase = orig }()

	var ts *httptest.Server
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "updates_1_p.html"):
			_, _ = fmt.Fprint(w, listingHTML(ts.URL))
		case strings.Contains(r.URL.Path, "built-for-speed"):
			_, _ = fmt.Fprint(w, detailHTML("8514", "Built for Speed", "Angela is our Bombshell.", "AngelaWhite", "Angela White"))
		case strings.Contains(r.URL.Path, "festival-vibes"):
			_, _ = fmt.Fprint(w, detailHTML("8520", "Festival Vibes", "Festival fun.", "JaneDoe", "Jane Doe"))
		default:
			// updates_2_p.html etc. -> empty page -> stop.
			_, _ = fmt.Fprint(w, `<html><body></body></html>`)
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
	perf := map[string][]string{}
	for r := range ch {
		if r.Err != nil {
			t.Errorf("unexpected error: %v", r.Err)
			continue
		}
		if r.Kind == scraper.KindScene {
			got[r.Scene.ID] = r.Scene.Title
			perf[r.Scene.ID] = r.Scene.Performers
		}
	}
	if len(got) != 2 {
		t.Fatalf("got %d scenes, want 2: %v", len(got), got)
	}
	if got["8514"] != "Built for Speed" || got["8520"] != "Festival Vibes" {
		t.Errorf("scenes = %v", got)
	}
	if strings.Join(perf["8514"], ",") != "Angela White" {
		t.Errorf("8514 performers = %v (want [Angela White], Models nav excluded)", perf["8514"])
	}
}
