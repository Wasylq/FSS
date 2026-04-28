package auntjudys

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
	"github.com/Wasylq/FSS/scraper"
)

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url  string
		want bool
	}{
		{"https://www.auntjudysxxx.com/tour/categories/movies.html", true},
		{"https://auntjudysxxx.com/tour/categories/movies.html", true},
		{"https://www.auntjudysxxx.com/tour/models/andi-james.html", true},
		{"https://www.auntjudysxxx.com/", true},
		{"https://example.com/auntjudys", false},
		{"", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.want {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.want)
		}
	}
}

func TestParseListingPage(t *testing.T) {
	body := []byte(`
<div class="update_details" data-setid="1298">
	<a href="https://test.local/tour/scenes/Scene-One_vids.html">
		<img class="update_thumb" src0_1x="/tour/content/thumbs/1298-1x.jpg" />
	</a>
	<div class="cell update_date">04/28/2026</div>
	<div class="update_models">
		<a href="/tour/models/joolz.html">Joolz</a>
	</div>
</div>
<div class="update_details" data-setid="1294">
	<a href="/tour/scenes/Scene-Two_vids.html">
		<img class="update_thumb" src0_1x="/tour/content/thumbs/1294-1x.jpg" />
	</a>
	<div class="cell update_date">04/26/2026</div>
	<div class="update_models">
		<a href="/tour/models/taylor-vixxen.html">Taylor Vixxen</a>,
		<a href="/tour/models/other.html">Other Model</a>
	</div>
</div>
`)
	scenes := parseListingPage(body, "https://test.local")

	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2", len(scenes))
	}

	s := scenes[0]
	if s.id != "1298" {
		t.Errorf("id = %q", s.id)
	}
	if s.url != "https://test.local/tour/scenes/Scene-One_vids.html" {
		t.Errorf("url = %q", s.url)
	}
	wantDate := time.Date(2026, 4, 28, 0, 0, 0, 0, time.UTC)
	if !s.date.Equal(wantDate) {
		t.Errorf("date = %v, want %v", s.date, wantDate)
	}
	if len(s.performers) != 1 || s.performers[0] != "Joolz" {
		t.Errorf("performers = %v", s.performers)
	}
	if s.thumb != "https://test.local/tour/content/thumbs/1298-1x.jpg" {
		t.Errorf("thumb = %q", s.thumb)
	}

	s2 := scenes[1]
	if s2.url != "https://test.local/tour/scenes/Scene-Two_vids.html" {
		t.Errorf("url = %q", s2.url)
	}
	if len(s2.performers) != 2 || s2.performers[1] != "Other Model" {
		t.Errorf("performers = %v", s2.performers)
	}
}

func TestParseDetailPage(t *testing.T) {
	body := []byte(`
<span class="title_bar_hilite">Curvy GILF Scene Title</span>
<span class="update_description">A great description of the scene.</span>
<span class="update_tags">
	<a href="/tags/milf.html">MILF</a>
	<a href="/tags/pov.html">POV</a>
	<a href="/tags/hardcore.html">Hardcore</a>
</span>
<span class="update_models">
	<a href="/tour/models/joolz.html">Joolz</a>
</span>
<div class="update_counts">20&nbsp;min&nbsp;of video</div>
`)
	d := parseDetailPage(body)
	if d.title != "Curvy GILF Scene Title" {
		t.Errorf("title = %q", d.title)
	}
	if d.description != "A great description of the scene." {
		t.Errorf("description = %q", d.description)
	}
	if len(d.tags) != 3 || d.tags[0] != "MILF" {
		t.Errorf("tags = %v", d.tags)
	}
	if len(d.performers) != 1 || d.performers[0] != "Joolz" {
		t.Errorf("performers = %v", d.performers)
	}
	if d.duration != 1200 {
		t.Errorf("duration = %d, want 1200", d.duration)
	}
}

func TestParseDetailPageEmpty(t *testing.T) {
	d := parseDetailPage([]byte(`<div>nothing here</div>`))
	if d.title != "" || d.description != "" || len(d.tags) != 0 {
		t.Errorf("expected empty detail, got %+v", d)
	}
}

func TestEstimateTotal(t *testing.T) {
	body := []byte(`<a href="movies_5_d.html">5</a><a href="movies_10_d.html">10</a>`)
	if got := estimateTotal(body, 24); got != 240 {
		t.Errorf("estimateTotal = %d, want 240", got)
	}
}

const listingTpl = `%s
%s`

const itemTpl = `<div class="update_details" data-setid="%d">
<a href="%s/tour/scenes/scene-%d_vids.html">
<img class="update_thumb" src0_1x="/thumbs/%d-1x.jpg" /></a>
<div class="cell update_date">01/15/2026</div>
<div class="update_models"><a href="/tour/models/test.html">Test Model</a></div>
</div>`

const detailTpl = `
<span class="title_bar_hilite">Test Scene Title</span>
<span class="update_description">Test description.</span>
<span class="update_tags"><a href="#">MILF</a></span>
<span class="update_models"><a href="/tour/models/test.html">Test Model</a></span>
<div class="update_counts">15&nbsp;min&nbsp;of video</div>
`

func buildListingPage(base string, ids []int, nextPages string) []byte {
	var items string
	for _, id := range ids {
		items += fmt.Sprintf(itemTpl, id, base, id, id)
	}
	return []byte(fmt.Sprintf(listingTpl, nextPages, items))
}

func newTestServer(pages [][]int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")

		switch {
		case r.URL.Path == "/tour/categories/movies.html":
			nextPages := ""
			for p := 2; p <= len(pages); p++ {
				nextPages += fmt.Sprintf(`<a href="movies_%d_d.html">%d</a>`, p, p)
			}
			_, _ = w.Write(buildListingPage("", pages[0], nextPages))

		case len(r.URL.Path) > len("/tour/categories/movies_") && r.URL.Path[:len("/tour/categories/movies_")] == "/tour/categories/movies_":
			pageNum := 0
			_, _ = fmt.Sscanf(r.URL.Path, "/tour/categories/movies_%d_d.html", &pageNum)
			idx := pageNum - 1
			if idx >= 0 && idx < len(pages) {
				_, _ = w.Write(buildListingPage("", pages[idx], ""))
			} else {
				_, _ = fmt.Fprint(w, `<div>empty</div>`)
			}

		case r.URL.Path == "/tour/models/test-model.html":
			_, _ = w.Write(buildListingPage("", pages[0], ""))

		default:
			_, _ = fmt.Fprint(w, detailTpl)
		}
	}))
}

func TestListScenes(t *testing.T) {
	ts := newTestServer([][]int{{100, 200}})
	defer ts.Close()

	s := &Scraper{client: ts.Client(), base: ts.URL}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/tour/categories/movies.html", scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}

	results := testutil.CollectScenes(t, ch)
	if len(results) != 2 {
		t.Fatalf("got %d scenes, want 2", len(results))
	}
}

func TestListScenesPagination(t *testing.T) {
	page1 := make([]int, 24)
	for i := range page1 {
		page1[i] = i + 1
	}
	page2 := []int{25, 26, 27}

	ts := newTestServer([][]int{page1, page2})
	defer ts.Close()

	s := &Scraper{client: ts.Client(), base: ts.URL}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/tour/categories/movies.html", scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}

	results := testutil.CollectScenes(t, ch)
	if len(results) != 27 {
		t.Fatalf("got %d scenes, want 27", len(results))
	}
}

func TestListScenesKnownIDs(t *testing.T) {
	ts := newTestServer([][]int{{1, 2, 3, 4}})
	defer ts.Close()

	s := &Scraper{client: ts.Client(), base: ts.URL}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/tour/categories/movies.html", scraper.ListOpts{
		KnownIDs: map[string]bool{"3": true},
	})
	if err != nil {
		t.Fatal(err)
	}

	results, stoppedEarly := testutil.CollectScenesWithStop(t, ch)
	if !stoppedEarly {
		t.Error("expected StoppedEarly signal")
	}
	if len(results) != 2 {
		t.Fatalf("got %d scenes, want 2", len(results))
	}
}

func TestListScenesModelPage(t *testing.T) {
	ts := newTestServer([][]int{{10, 20, 30}})
	defer ts.Close()

	s := &Scraper{client: ts.Client(), base: ts.URL}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/tour/models/test-model.html", scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}

	results := testutil.CollectScenes(t, ch)
	if len(results) != 3 {
		t.Fatalf("got %d scenes, want 3", len(results))
	}
}
