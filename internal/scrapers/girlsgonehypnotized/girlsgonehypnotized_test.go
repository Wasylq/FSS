package girlsgonehypnotized

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
	"github.com/Wasylq/FSS/scraper"
)

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url  string
		want bool
	}{
		{"https://girlsgonehypnotized.com/", true},
		{"https://www.girlsgonehypnotized.com/", true},
		{"https://girlsgonehypnotized.com/HypnoHeelsQuinn.html", true},
		{"https://example.com/girlsgonehypnotized", false},
		{"", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.want {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.want)
		}
	}
}

func TestParseHomepage(t *testing.T) {
	body := []byte(`
<table>
<tr>
<td valign="top"><a href="GirlSearch.html"><img src="nav.jpg" alt="Search"></a></td>
<td valign="top"><a href="SceneOne.html"><img src="GGH%20Thumbnails/Scene%20One%20Thumbnail.jpg" alt="Scene One" width="133" height="199"></a></td>
<td valign="top"><a href="SceneTwo.html"><img src="GGH%20Thumbnails/Scene%20Two%20Thumbnail.jpg" alt="Scene Two" width="133" height="200"></a></td>
<td valign="top"><a href="SceneOne.html"><img src="GGH%20Thumbnails/Scene%20One%20Thumbnail.jpg" alt="Scene One" width="133" height="199"></a></td>
</tr>
</table>
`)
	entries := parseHomepage(body, "https://test.local")
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}

	e := entries[0]
	if e.id != "SceneOne" {
		t.Errorf("id = %q", e.id)
	}
	if e.url != "https://test.local/SceneOne.html" {
		t.Errorf("url = %q", e.url)
	}
	if e.title != "Scene One" {
		t.Errorf("title = %q", e.title)
	}
	if e.thumb != "https://test.local/GGH Thumbnails/Scene One Thumbnail.jpg" {
		t.Errorf("thumb = %q", e.thumb)
	}

	e2 := entries[1]
	if e2.id != "SceneTwo" {
		t.Errorf("id = %q", e2.id)
	}
}

func TestParseDetailPage(t *testing.T) {
	body := []byte(`
<title>Girls Gone Hypnotized - Hypno Heels - Quinn</title>
<span style="font-weight: bold;">Full Download
  Details:</span><br>
<span style="font-weight: bold;">10 minutes, 35
  seconds</span><br>
<span style="font-weight: bold; color: rgb(51, 204, 0);">Only $8.99</span><br>
`)
	d := parseDetailPage(body)
	if d.title != "Hypno Heels - Quinn" {
		t.Errorf("title = %q", d.title)
	}
	if d.duration != 635 {
		t.Errorf("duration = %d, want 635", d.duration)
	}
	if d.price != 8.99 {
		t.Errorf("price = %f, want 8.99", d.price)
	}
}

func TestParseDetailPageLongDuration(t *testing.T) {
	body := []byte(`
<title>Girls Gone Hypnotized - Long Session</title>
<span style="font-weight: bold;">53 minutes, 42 seconds</span><br>
<span style="font-weight: bold; color: rgb(51, 204, 0);">Only $29.99</span><br>
`)
	d := parseDetailPage(body)
	if d.duration != 3222 {
		t.Errorf("duration = %d, want 3222", d.duration)
	}
	if d.price != 29.99 {
		t.Errorf("price = %f, want 29.99", d.price)
	}
}

func TestParseDetailPageNoPrice(t *testing.T) {
	body := []byte(`
<title>Girls Gone Hypnotized - Free Preview</title>
<span style="font-weight: bold;">5 minutes, 0 seconds</span><br>
`)
	d := parseDetailPage(body)
	if d.price != 0 {
		t.Errorf("price = %f, want 0", d.price)
	}
	if d.duration != 300 {
		t.Errorf("duration = %d, want 300", d.duration)
	}
}

const homepageTpl = `<table><tr>
%s
</tr></table>`

const entryTpl = `<td valign="top"><a href="%s.html"><img src="GGH%%20Thumbnails/%s%%20Thumbnail.jpg" alt="%s" width="133" height="199"></a></td>`

const detailTpl = `
<title>Girls Gone Hypnotized - %s</title>
<span style="font-weight: bold;">10 minutes, 0 seconds</span><br>
<span style="font-weight: bold; color: rgb(51, 204, 0);">Only $9.99</span><br>
`

func newTestServer(slugs []string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")

		switch r.URL.Path {
		case "/", "/index.html":
			var items string
			for _, slug := range slugs {
				items += fmt.Sprintf(entryTpl, slug, slug, slug)
			}
			_, _ = fmt.Fprintf(w, homepageTpl, items)
		default:
			slug := r.URL.Path[1:]
			slug = slug[:len(slug)-5] // strip .html
			_, _ = fmt.Fprintf(w, detailTpl, slug)
		}
	}))
}

func TestRun(t *testing.T) {
	ts := newTestServer([]string{"SceneAlpha", "SceneBeta"})
	defer ts.Close()

	s := &Scraper{client: ts.Client()}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/", scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}

	results := testutil.CollectScenes(t, ch)
	if len(results) != 2 {
		t.Fatalf("got %d scenes, want 2", len(results))
	}

	found := map[string]bool{}
	for _, r := range results {
		found[r.ID] = true
		if r.Duration != 600 {
			t.Errorf("scene %s duration = %d, want 600", r.ID, r.Duration)
		}
		if r.Studio != "GG Fetish Media" {
			t.Errorf("studio = %q", r.Studio)
		}
	}
	if !found["SceneAlpha"] || !found["SceneBeta"] {
		t.Errorf("missing scenes: %v", found)
	}
}

func TestRunKnownIDs(t *testing.T) {
	ts := newTestServer([]string{"SceneA", "SceneB", "SceneC"})
	defer ts.Close()

	s := &Scraper{client: ts.Client()}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/", scraper.ListOpts{
		KnownIDs: map[string]bool{"SceneB": true},
	})
	if err != nil {
		t.Fatal(err)
	}

	results, stoppedEarly := testutil.CollectScenesWithStop(t, ch)
	if !stoppedEarly {
		t.Error("expected StoppedEarly signal")
	}
	if len(results) != 1 {
		t.Fatalf("got %d scenes, want 1", len(results))
	}
	if results[0].ID != "SceneA" {
		t.Errorf("expected SceneA, got %s", results[0].ID)
	}
}

func TestResolveBase(t *testing.T) {
	cases := []struct {
		url  string
		want string
	}{
		{"https://girlsgonehypnotized.com/", "https://girlsgonehypnotized.com"},
		{"https://www.girlsgonehypnotized.com/", "https://www.girlsgonehypnotized.com"},
		{"https://girlsgonehypnotized.com/HypnoHeelsQuinn.html", "https://girlsgonehypnotized.com"},
		{"girlsgonehypnotized.com", defaultBase},
	}
	for _, c := range cases {
		if got := resolveBase(c.url); got != c.want {
			t.Errorf("resolveBase(%q) = %q, want %q", c.url, got, c.want)
		}
	}
}
