package hookuphotshot

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

func listingHTML() string {
	return `<html><body>
<div class="items clear">
<div class="item-video hover">
	<div class="item-thumb">
		<a href="/trailers/Episode-466-Lola-Grant-2-Anal.html" title="Episode 466 - Lola Grant 2 Anal">
			<img class="thumbs stdimage" src0_1x="/content//contentthumbs/16/36/11636-1x.jpg">
		</a>
	</div>
	<div class="time">457&nbsp;Photos, 01:04:23</div>
	<div class="date">2026-06-19</div>
</div>
<div class="item-video hover">
	<div class="item-thumb">
		<a href="/trailers/Episode-465-Mira-Luv.html" title="Episode 465 - Mira Luv">
			<img class="thumbs stdimage" src0_1x="/content//contentthumbs/15/73/11573-1x.jpg">
		</a>
	</div>
	<div class="date">2026-06-14</div>
</div>
</div>
</body></html>`
}

func detailHTML(performer string) string {
	return fmt.Sprintf(`<html><head>
<meta property="og:title" content="Episode 466 - Lola Grant 2 Anal">
</head><body>
<div class="models">
  <a href="/models/%s.html">%s</a>
</div>
</body></html>`, strings.ReplaceAll(performer, " ", "-"), performer)
}

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url   string
		match bool
	}{
		{"https://hookuphotshot.com/categories/movies/1/latest/", true},
		{"https://www.hookuphotshot.com/trailers/foo.html", true},
		{"https://example.com/x", false},
		{"", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.match {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.match)
		}
	}
}

func TestParseListing(t *testing.T) {
	orig := baseURL
	defer func() { baseURL = orig }()
	baseURL = "https://hookuphotshot.com"

	items := parseListing([]byte(listingHTML()), time.Now().UTC())
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	it := items[0]
	if it.id != "11636" {
		t.Errorf("id = %q, want 11636", it.id)
	}
	if it.title != "Episode 466 - Lola Grant 2 Anal" {
		t.Errorf("title = %q", it.title)
	}
	if it.url != "https://hookuphotshot.com/trailers/Episode-466-Lola-Grant-2-Anal.html" {
		t.Errorf("url = %q", it.url)
	}
	if it.thumb != "https://hookuphotshot.com/content//contentthumbs/16/36/11636-1x.jpg" {
		t.Errorf("thumb = %q", it.thumb)
	}
	if y, m, d := it.date.Date(); y != 2026 || m != 6 || d != 19 {
		t.Errorf("date = %v, want 2026-06-19", it.date)
	}
}

func TestParsePerformers(t *testing.T) {
	got := parsePerformers([]byte(detailHTML("Lola Grant")))
	if len(got) != 1 || got[0] != "Lola Grant" {
		t.Errorf("performers = %v, want [Lola Grant]", got)
	}
}

func TestListScenes(t *testing.T) {
	orig := baseURL
	defer func() { baseURL = orig }()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/movies/1/"):
			_, _ = fmt.Fprint(w, listingHTML())
		case strings.Contains(r.URL.Path, "Episode-466"):
			_, _ = fmt.Fprint(w, detailHTML("Lola Grant"))
		case strings.Contains(r.URL.Path, "Episode-465"):
			_, _ = fmt.Fprint(w, detailHTML("Mira Luv"))
		default:
			_, _ = fmt.Fprint(w, "<html></html>")
		}
	}))
	defer ts.Close()
	baseURL = ts.URL

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
	if got["11636"] != "Episode 466 - Lola Grant 2 Anal" {
		t.Errorf("scenes = %v", got)
	}
	if len(perf["11636"]) != 1 || perf["11636"][0] != "Lola Grant" {
		t.Errorf("performers = %v", perf["11636"])
	}
}
