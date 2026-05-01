package purecfnm

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
		{"https://www.purecfnm.com/", true},
		{"https://purecfnm.com/", true},
		{"https://www.purecfnm.com/categories/movies_1_d.html", true},
		{"https://www.purecfnm.com/models/summer-foxy.html", true},
		{"https://www.purecfnm.com/sites/", true},
		{"https://example.com/purecfnm", false},
		{"", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.want {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.want)
		}
	}
}

func TestExtractSlug(t *testing.T) {
	cases := []struct {
		url  string
		want string
	}{
		{"https://www.purecfnm.com/categories/movies_1_d.html", "movies"},
		{"https://www.purecfnm.com/categories/ladyvoyeurs_3_d.html", "ladyvoyeurs"},
		{"https://www.purecfnm.com/", "movies"},
		{"https://www.purecfnm.com/models/summer-foxy.html", "movies"},
	}
	for _, c := range cases {
		if got := extractSlug(c.url); got != c.want {
			t.Errorf("extractSlug(%q) = %q, want %q", c.url, got, c.want)
		}
	}
}

func TestParseListingPage(t *testing.T) {
	body := []byte(`
<div class="update_details" data-setid="2622">
	<a title="" href="/join/">
		<img class="update_thumb thumbs stdimage" src="/content/thumbs/2622-1x.jpg" />
	</a>
	<a title="" href="/join/">
		Hero Fireman
	</a>
	<span class="update_models">
		<a href="/models/Robyn-Quinn.html">Robyn Quinn</a>,
		<a href="/models/Samantha-Jayne.html">Samantha Jayne</a>
	</span>
	<div class="update_counts">202&nbsp;Photos, 13&nbsp;minute(s)&nbsp;of video</div>
	<div class="cell update_date">May 1, 2026</div>
</div>
<div class="update_details" data-setid="2602">
	<a title="" href="/join/">
		<img class="update_thumb thumbs" src="/content/thumbs/2602-1x.jpg" />
	</a>
	<a title="" href="/join/">
		Straitjacket
	</a>
	<span class="update_models">
		<a href="/models/Babe-Ashton.html">Babe Ashton</a>
	</span>
	<div class="update_counts">153&nbsp;Photos, 6&nbsp;minute(s)&nbsp;of video</div>
	<div class="cell update_date">April 24, 2026</div>
</div>
`)
	scenes := parseListingPage(body, "https://test.local")
	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2", len(scenes))
	}

	s := scenes[0]
	if s.id != "2622" {
		t.Errorf("id = %q, want 2622", s.id)
	}
	if s.title != "Hero Fireman" {
		t.Errorf("title = %q", s.title)
	}
	if len(s.performers) != 2 || s.performers[0] != "Robyn Quinn" {
		t.Errorf("performers = %v", s.performers)
	}
	wantDate := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	if !s.date.Equal(wantDate) {
		t.Errorf("date = %v, want %v", s.date, wantDate)
	}
	if s.duration != 780 {
		t.Errorf("duration = %d, want 780", s.duration)
	}
	if s.thumb != "https://test.local/content/thumbs/2622-1x.jpg" {
		t.Errorf("thumb = %q", s.thumb)
	}

	s2 := scenes[1]
	if s2.id != "2602" {
		t.Errorf("id = %q, want 2602", s2.id)
	}
	if s2.title != "Straitjacket" {
		t.Errorf("title = %q", s2.title)
	}
	if len(s2.performers) != 1 || s2.performers[0] != "Babe Ashton" {
		t.Errorf("performers = %v", s2.performers)
	}
}

func TestParseModelPage(t *testing.T) {
	body := []byte(`
<div class="update_block">
<div class="update_block_info">
	<span class="update_title">Double Booking</span>
	<span class="tour_update_models">
		<a href="/models/Em-Yang.html">Em Yang</a>,
		<a href="/models/summer-foxy.html">Summer Foxy</a>
	</span>
	<span class="update_date">October 10, 2025</span>
	<span class="latest_update_description">A great scene description.</span>
	<span class="tour_update_tags" style="display:none;">
		Tags: <a href="/categories/group.html">Group</a>,
		<a href="/categories/handjob.html">Handjob</a>
	</span>
</div>
<img id="set-target-2470-12345" class="large_update_thumb left thumbs" src="/content/thumbs/2470-1x.jpg" />
<div class="update_counts_preview_table">96&nbsp;Photos, 7&nbsp;minute(s)&nbsp;of video</div>
</div>
`)
	scenes := parseModelPage(body, "https://test.local")
	if len(scenes) != 1 {
		t.Fatalf("got %d scenes, want 1", len(scenes))
	}

	s := scenes[0]
	if s.id != "2470" {
		t.Errorf("id = %q, want 2470", s.id)
	}
	if s.title != "Double Booking" {
		t.Errorf("title = %q", s.title)
	}
	if len(s.performers) != 2 || s.performers[0] != "Em Yang" {
		t.Errorf("performers = %v", s.performers)
	}
	wantDate := time.Date(2025, 10, 10, 0, 0, 0, 0, time.UTC)
	if !s.date.Equal(wantDate) {
		t.Errorf("date = %v, want %v", s.date, wantDate)
	}
	if s.description != "A great scene description." {
		t.Errorf("description = %q", s.description)
	}
	if len(s.tags) != 2 || s.tags[0] != "Group" {
		t.Errorf("tags = %v", s.tags)
	}
	if s.thumb != "https://test.local/content/thumbs/2470-1x.jpg" {
		t.Errorf("thumb = %q", s.thumb)
	}
	if s.duration != 420 {
		t.Errorf("duration = %d, want 420", s.duration)
	}
}

func TestEstimateTotal(t *testing.T) {
	body := []byte(`<a href="movies_5_d.html">5</a><a href="movies_37_d.html">37</a>`)
	if got := estimateTotal(body, 28); got != 1036 {
		t.Errorf("estimateTotal = %d, want 1036", got)
	}
}

const listingItemTpl = `<div class="update_details" data-setid="%d">
<a title="" href="/join/"><img class="update_thumb thumbs" src="/content/thumbs/%d-1x.jpg" /></a>
<a title="" href="/join/">Scene %d</a>
<span class="update_models"><a href="/models/test.html">Test Model</a></span>
<div class="update_counts">10&nbsp;minute(s)&nbsp;of video</div>
<div class="cell update_date">January 15, 2026</div>
</div>`

const modelItemTpl = `<div class="update_block">
<div class="update_block_info">
<span class="update_title">Model Scene %d</span>
<span class="tour_update_models"><a href="/models/test.html">Test Model</a></span>
<span class="update_date">January 15, 2026</span>
<span class="latest_update_description">Description %d</span>
<span class="tour_update_tags" style="display:none;">Tags: <a href="#">CFNM</a></span>
</div>
<img id="set-target-%d-999" class="large_update_thumb left thumbs" src="/content/thumbs/%d-1x.jpg" />
<div class="update_counts_preview_table">10&nbsp;minute(s)&nbsp;of video</div>
</div>`

func buildListingPage(ids []int, maxPage int) []byte {
	var sb string
	for _, id := range ids {
		sb += fmt.Sprintf(listingItemTpl, id, id, id)
	}
	pager := ""
	for p := 2; p <= maxPage; p++ {
		pager += fmt.Sprintf(`<a href="movies_%d_d.html">%d</a>`, p, p)
	}
	return []byte(pager + sb)
}

func buildModelPage(ids []int) []byte {
	var sb string
	for _, id := range ids {
		sb += fmt.Sprintf(modelItemTpl, id, id, id, id)
	}
	return []byte(sb)
}

func newTestServer(pages [][]int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")

		switch r.URL.Path {
		case "/models/test-model.html":
			_, _ = w.Write(buildModelPage(pages[0]))

		default:
			pageNum := 0
			_, _ = fmt.Sscanf(r.URL.Path, "/categories/movies_%d_d.html", &pageNum)
			if pageNum == 0 {
				pageNum = 1
			}
			idx := pageNum - 1
			if idx >= 0 && idx < len(pages) {
				_, _ = w.Write(buildListingPage(pages[idx], len(pages)))
			} else {
				_, _ = fmt.Fprint(w, `<div>empty</div>`)
			}
		}
	}))
}

func TestListScenes(t *testing.T) {
	ts := newTestServer([][]int{{100, 200}})
	defer ts.Close()

	s := &Scraper{client: ts.Client(), base: ts.URL}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/categories/movies_1_d.html", scraper.ListOpts{
		Delay: time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}

	results := testutil.CollectScenes(t, ch)
	if len(results) != 2 {
		t.Fatalf("got %d scenes, want 2", len(results))
	}
	if results[0].Title != "Scene 100" {
		t.Errorf("title = %q, want Scene 100", results[0].Title)
	}
}

func TestListScenesKnownIDs(t *testing.T) {
	ts := newTestServer([][]int{{1, 2, 3, 4}})
	defer ts.Close()

	s := &Scraper{client: ts.Client(), base: ts.URL}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/categories/movies_1_d.html", scraper.ListOpts{
		KnownIDs: map[string]bool{"3": true},
		Delay:    time.Millisecond,
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

func TestListScenesPagination(t *testing.T) {
	page1 := make([]int, 28)
	for i := range page1 {
		page1[i] = i + 1
	}
	page2 := []int{29, 30, 31}

	ts := newTestServer([][]int{page1, page2})
	defer ts.Close()

	s := &Scraper{client: ts.Client(), base: ts.URL}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/categories/movies_1_d.html", scraper.ListOpts{
		Delay: time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}

	results := testutil.CollectScenes(t, ch)
	if len(results) != 31 {
		t.Fatalf("got %d scenes, want 31", len(results))
	}
}

func TestListScenesModelPage(t *testing.T) {
	ts := newTestServer([][]int{{10, 20, 30}})
	defer ts.Close()

	s := &Scraper{client: ts.Client(), base: ts.URL}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/models/test-model.html", scraper.ListOpts{
		Delay: time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}

	results := testutil.CollectScenes(t, ch)
	if len(results) != 3 {
		t.Fatalf("got %d scenes, want 3", len(results))
	}
	if results[0].Description != "Description 10" {
		t.Errorf("description = %q, want Description 10", results[0].Description)
	}
	if len(results[0].Tags) != 1 || results[0].Tags[0] != "CFNM" {
		t.Errorf("tags = %v", results[0].Tags)
	}
}

func TestListScenesRootURL(t *testing.T) {
	ts := newTestServer([][]int{{500, 501}})
	defer ts.Close()

	s := &Scraper{client: ts.Client(), base: ts.URL}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/", scraper.ListOpts{
		Delay: time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}

	results := testutil.CollectScenes(t, ch)
	if len(results) != 2 {
		t.Fatalf("got %d scenes, want 2", len(results))
	}
}
