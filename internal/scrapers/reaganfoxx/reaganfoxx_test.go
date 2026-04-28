package reaganfoxx

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
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
		{"https://www.reaganfoxx.com/", true},
		{"https://reaganfoxx.com/", true},
		{"https://www.reaganfoxx.com/scenes/673608/reagan-foxx-streaming-pornstar-videos.html", true},
		{"https://www.reaganfoxx.com/1597175/reagan-foxx-stepmom-reagan-is-busted-streaming-scene-video.html", true},
		{"https://example.com/reaganfoxx", false},
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
<h4>174 Results</h4>
<div class="pagination d-flex">
<ul class="pagination"><li class="page-item active"><a href="?" class="page-link">1</a></li>
<li class="page-item"><a href="?page=2" class="page-link">2</a></li></ul>
</div>
<article class="scene-widget member-view"
	data-scene-id="1597175"
	data-master-id="4574744">
<div class="scene-preview-container">
<a class="scene-img" href="/1597175/stepmom-reagan-is-busted.html">
<img class="screenshot img-full-fluid" id="scene_1597175"
	data-src="https://caps1cdn.adultempire.com/200/4574744_300.jpg"
	src="https://caps1cdn.adultempire.com/10/4574744_300.jpg"/>
</a></div>
<div class="scene-info-container">
<a class="scene-title" href="/1597175/stepmom-reagan-is-busted.html">
<h6>Stepmom Reagan Is Busted</h6></a>
<p class="scene-performer-names">Reagan Foxx</p>
<p class="scene-length">11 min</p>
</div></article>
<article class="scene-widget member-view"
	data-scene-id="1597306"
	data-master-id="4575161">
<div class="scene-preview-container">
<a class="scene-img" href="/1597306/milf-gives-virgin-stepson.html">
<img class="screenshot" data-src="https://caps1cdn.adultempire.com/200/4575161_300.jpg"/>
</a></div>
<div class="scene-info-container">
<a class="scene-title" href="/1597306/milf-gives-virgin-stepson.html">
<h6>MILF Gives Virgin Stepson Sexy Advice</h6></a>
<p class="scene-performer-names">Reagan Foxx, Johnny The Kid</p>
<p class="scene-length">25 min</p>
</div></article>
`)

	scenes := parseListingPage(body, "https://test.local")

	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2", len(scenes))
	}

	s := scenes[0]
	if s.id != "1597175" {
		t.Errorf("id = %q", s.id)
	}
	if s.title != "Stepmom Reagan Is Busted" {
		t.Errorf("title = %q", s.title)
	}
	if s.url != "https://test.local/1597175/stepmom-reagan-is-busted.html" {
		t.Errorf("url = %q", s.url)
	}
	if len(s.performers) != 1 || s.performers[0] != "Reagan Foxx" {
		t.Errorf("performers = %v", s.performers)
	}
	if s.duration != 660 {
		t.Errorf("duration = %d, want 660", s.duration)
	}
	if s.thumb != "https://caps1cdn.adultempire.com/200/4574744_300.jpg" {
		t.Errorf("thumb = %q", s.thumb)
	}

	s2 := scenes[1]
	if len(s2.performers) != 2 {
		t.Errorf("performers = %v, want 2", s2.performers)
	}
	if s2.performers[0] != "Reagan Foxx" || s2.performers[1] != "Johnny The Kid" {
		t.Errorf("performers = %v", s2.performers)
	}
}

func TestExtractTotal(t *testing.T) {
	body := []byte(`<h4>174 Results</h4>`)
	if got := extractTotal(body); got != 174 {
		t.Errorf("extractTotal = %d, want 174", got)
	}
	if got := extractTotal([]byte(`no results`)); got != 0 {
		t.Errorf("extractTotal empty = %d, want 0", got)
	}
}

func TestHasPagination(t *testing.T) {
	with := []byte(`<nav class="pagination d-flex"><ul class="pagination"></ul></nav>`)
	without := []byte(`<div>no pagination here</div>`)
	if !hasPagination(with) {
		t.Error("expected true for page with pagination")
	}
	if hasPagination(without) {
		t.Error("expected false for page without pagination")
	}
}

func TestHasNextPage(t *testing.T) {
	body := []byte(`<a href="?page=2">2</a><a href="?page=3">3</a>`)
	if !hasNextPage(body, 1) {
		t.Error("expected next from page 1")
	}
	if !hasNextPage(body, 2) {
		t.Error("expected next from page 2")
	}
	if hasNextPage(body, 3) {
		t.Error("expected no next from page 3")
	}
}

func TestParseDetailPage(t *testing.T) {
	body := []byte(`
<div class="release-date">
<span class="font-weight-bold mr-2">Released:</span>May 19, 2023
</div>
<div class="studio">
<span class="font-weight-bold mr-2">Studio:</span>
<span>Reagan Foxx</span>
</div>
<div class="tags">
<span class="font-weight-bold mr-2">Tags:</span>
<a href="/join">Family Roleplaying</a>,
<a href="/join">MILF</a>,
<a href="/join">Brunette</a>
</div>
<h3 class="price mb-1">$7.9900</h3>
<h6 class="card-title mb-1">Buy This Scene</h6>
`)
	d := parseDetailPage(body)

	wantDate := time.Date(2023, 5, 19, 0, 0, 0, 0, time.UTC)
	if !d.date.Equal(wantDate) {
		t.Errorf("date = %v, want %v", d.date, wantDate)
	}
	if d.studio != "Reagan Foxx" {
		t.Errorf("studio = %q", d.studio)
	}
	if len(d.tags) != 3 || d.tags[0] != "Family Roleplaying" || d.tags[1] != "MILF" {
		t.Errorf("tags = %v", d.tags)
	}
	if d.price != 7.99 {
		t.Errorf("price = %f, want 7.99", d.price)
	}
}

func TestParseDetailPageShortMonth(t *testing.T) {
	body := []byte(`
<div class="release-date">
<span class="font-weight-bold mr-2">Released:</span>Jan 13, 2023
</div>
`)
	d := parseDetailPage(body)
	wantDate := time.Date(2023, 1, 13, 0, 0, 0, 0, time.UTC)
	if !d.date.Equal(wantDate) {
		t.Errorf("date = %v, want %v", d.date, wantDate)
	}
}

func TestParseDetailPageNoPrice(t *testing.T) {
	body := []byte(`
<div class="release-date">
<span class="font-weight-bold mr-2">Released:</span>Jan 1, 2024
</div>
`)
	d := parseDetailPage(body)
	if d.price != 0 {
		t.Errorf("price = %f, want 0", d.price)
	}
}

const listingHTML = `<h4>%d Results</h4>
<div class="pagination d-flex"><ul class="pagination">
<li class="page-item active"><a class="page-link">1</a></li>
%s
</ul></div>
%s`

const sceneHTML = `<article class="scene-widget member-view"
	data-scene-id="%d" data-master-id="999">
<a class="scene-img" href="/%d/title.html">
<img class="screenshot" data-src="https://caps1cdn.adultempire.com/200/999_300.jpg"/>
</a>
<a class="scene-title" href="/%d/title.html"><h6>Scene %d</h6></a>
<p class="scene-performer-names">Reagan Foxx</p>
<p class="scene-length">10 min</p>
</article>`

const detailHTML = `
<div class="release-date"><span class="font-weight-bold mr-2">Released:</span>Jan 1, 2024</div>
<div class="studio"><span class="font-weight-bold mr-2">Studio:</span><span>Reagan Foxx</span></div>
<div class="tags"><span class="font-weight-bold mr-2">Tags:</span><a href="#">MILF</a></div>
<h3 class="price mb-1">$5.99</h3>
<h6 class="card-title mb-1">Buy This Scene</h6>
`

func buildListingPage(ids []int, total int, nextPages string) []byte {
	var scenes string
	for _, id := range ids {
		scenes += fmt.Sprintf(sceneHTML, id, id, id, id)
	}
	return []byte(fmt.Sprintf(listingHTML, total, nextPages, scenes))
}

func newTestServer(pages [][]int, total int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")

		if r.URL.Path == "/scenes/1/listing.html" {
			pageStr := r.URL.Query().Get("page")
			pageNum := 1
			if pageStr != "" {
				pageNum, _ = strconv.Atoi(pageStr)
			}
			idx := pageNum - 1
			if idx >= len(pages) {
				_, _ = fmt.Fprint(w, `<div>no scenes</div>`)
				return
			}
			nextPages := ""
			for p := 2; p <= len(pages); p++ {
				nextPages += fmt.Sprintf(`<li class="page-item"><a href="?page=%d" class="page-link">%d</a></li>`, p, p)
			}
			_, _ = w.Write(buildListingPage(pages[idx], total, nextPages))
			return
		}

		_, _ = fmt.Fprint(w, detailHTML)
	}))
}

func TestListScenes(t *testing.T) {
	ts := newTestServer([][]int{{100, 200}}, 2)
	defer ts.Close()

	s := &Scraper{client: ts.Client(), base: ts.URL}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/scenes/1/listing.html", scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}

	results := testutil.CollectScenes(t, ch)
	if len(results) != 2 {
		t.Fatalf("got %d scenes, want 2", len(results))
	}
}

func TestListScenesPagination(t *testing.T) {
	page1 := make([]int, 52)
	for i := range page1 {
		page1[i] = i + 1
	}
	page2 := []int{53, 54, 55}

	ts := newTestServer([][]int{page1, page2}, 55)
	defer ts.Close()

	s := &Scraper{client: ts.Client(), base: ts.URL}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/scenes/1/listing.html", scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}

	results := testutil.CollectScenes(t, ch)
	if len(results) != 55 {
		t.Fatalf("got %d scenes, want 55", len(results))
	}
}

func TestListScenesKnownIDs(t *testing.T) {
	ts := newTestServer([][]int{{1, 2, 3, 4}}, 4)
	defer ts.Close()

	s := &Scraper{client: ts.Client(), base: ts.URL}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/scenes/1/listing.html", scraper.ListOpts{
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
