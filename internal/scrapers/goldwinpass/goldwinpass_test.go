package goldwinpass

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wasylq/FSS/scraper"
)

const listingHTML = `<html><body>
<div class="thumbs_holder">

<div class="th" data-setid="19000">
  <a href="https://join.goldwinpass.com/signup/signup.php?nats=ABC&amp;step=2">
    <img id="tour_1371406862" class="stdimage update_thumb thumbs" src="content/gwp-p-josysarahblack_05/0.jpg" />
    <span class="time">
      <!-- Photo And Movie Totals -->
    </span>
  </a>
  <div class="tools">
    <p class="title">
      <a title="Visit Pictures Gallery" href="">Visit Pictures Gallery</a>
    </p>
    <p class="cat_list">
      <span class="rating">User Rating: <strong>100 %</strong></span>
      <br>
      <span class="date">Puplished on: <b>05/19/2026</b></span>
    </p>
  </div>
</div>

<div class="th" data-setid="23678">
  <a href="https://join.goldwinpass.com/signup/signup.php?nats=ABC&amp;step=2">
    <img id="tour_627244519" class="stdimage update_thumb thumbs" src="content/gwp_038_02/0.jpg" />
    <span class="time">28&nbsp;minute(s)&nbsp;Movie</span>
  </a>
  <div class="tools">
    <p class="title">
      <a title="My hot girlfriend" href="">My hot girlfriend</a>
    </p>
    <p class="cat_list">
      <span class="rating">User Rating: <strong>100 %</strong></span>
      <br>
      <span class="date">Puplished on: <b>05/18/2026</b></span>
    </p>
  </div>
</div>

</div>

<div class="pagintaion">
  <a href="https://www.goldwinpass.com/tour/updates/page_2.html">2</a>
  <a href="https://www.goldwinpass.com/tour/updates/page_10.html">10</a>
</div>
</body></html>`

const emptyHTML = `<html><body><div class="pagintaion">no more</div></body></html>`

func TestParseListing_extractsCards(t *testing.T) {
	items := parseListing([]byte(listingHTML))
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}

	first := items[0]
	if first.id != "19000" {
		t.Errorf("ID = %q, want 19000", first.id)
	}
	if first.title != "Visit Pictures Gallery" {
		t.Errorf("Title = %q", first.title)
	}
	if first.date.Year() != 2026 || first.date.Month() != 5 || first.date.Day() != 19 {
		t.Errorf("Date = %v, want 2026-05-19", first.date)
	}
	// First card has no duration string ("Photo And Movie Totals" is a comment placeholder).
	if first.duration != 0 {
		t.Errorf("Duration = %d, want 0 (no minutes line)", first.duration)
	}
	if first.thumb != "content/gwp-p-josysarahblack_05/0.jpg" {
		t.Errorf("Thumb = %q (raw relative URL expected at parse time)", first.thumb)
	}

	second := items[1]
	if second.id != "23678" {
		t.Errorf("Second ID = %q", second.id)
	}
	if second.title != "My hot girlfriend" {
		t.Errorf("Second title = %q", second.title)
	}
	// 28 minutes = 1680s
	if second.duration != 28*60 {
		t.Errorf("Second duration = %d, want 1680", second.duration)
	}
}

func TestParseListing_dedupes(t *testing.T) {
	doubled := listingHTML + listingHTML
	items := parseListing([]byte(doubled))
	if len(items) != 2 {
		t.Errorf("got %d items after dedup, want 2", len(items))
	}
}

func TestEstimateTotal(t *testing.T) {
	// 2 cards × max-page 10 = 20
	got := estimateTotal([]byte(listingHTML), 2)
	if got != 20 {
		t.Errorf("estimateTotal = %d, want 20", got)
	}
}

func TestListingURL(t *testing.T) {
	s := &Scraper{base: "https://example.com"}
	tests := []struct {
		page int
		want string
	}{
		{1, "https://example.com/tour/updates/page_1.html"},
		{2, "https://example.com/tour/updates/page_2.html"},
		{10, "https://example.com/tour/updates/page_10.html"},
	}
	for _, c := range tests {
		got := s.listingURL(c.page)
		if got != c.want {
			t.Errorf("page %d → %q, want %q", c.page, got, c.want)
		}
	}
}

func TestToScene_resolvesRelativeThumb(t *testing.T) {
	item := sceneItem{id: "1", title: "T", thumb: "content/foo/0.jpg"}
	scene := item.toScene("https://example.com", item.date)
	want := "https://example.com/tour/content/foo/0.jpg"
	if scene.Thumbnail != want {
		t.Errorf("Thumbnail = %q, want %q", scene.Thumbnail, want)
	}
	if !strings.HasPrefix(scene.URL, "https://example.com/tour/#scene-") {
		t.Errorf("URL = %q (expected synthesised anchor)", scene.URL)
	}
}

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url   string
		match bool
	}{
		{"https://www.goldwinpass.com/tour/", true},
		{"http://goldwinpass.com/tour/updates/page_5.html", true},
		{"https://example.com/", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.match {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.match)
		}
	}
}

func TestListScenes_endToEnd(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		switch r.URL.Path {
		case "/tour/updates/page_1.html":
			_, _ = fmt.Fprint(w, listingHTML)
		case "/tour/updates/page_2.html":
			_, _ = fmt.Fprint(w, emptyHTML)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	s := New()
	s.base = ts.URL

	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}

	var scenes int
	for r := range ch {
		switch r.Kind {
		case scraper.KindScene:
			scenes++
			if r.Scene.Studio != "GoldwinPass" {
				t.Errorf("Studio = %q", r.Scene.Studio)
			}
			if !strings.HasPrefix(r.Scene.URL, ts.URL+"/tour/#scene-") {
				t.Errorf("URL = %q", r.Scene.URL)
			}
		case scraper.KindError:
			t.Errorf("unexpected error: %v", r.Err)
		}
	}

	if scenes != 2 {
		t.Errorf("got %d scenes, want 2", scenes)
	}
}

func TestListScenes_knownIDsStopsEarly(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		if r.URL.Path == "/tour/updates/page_1.html" {
			_, _ = fmt.Fprint(w, listingHTML)
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	s := New()
	s.base = ts.URL

	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{
		KnownIDs: map[string]bool{"23678": true},
	})
	if err != nil {
		t.Fatal(err)
	}

	var (
		scenes       int
		stoppedEarly bool
	)
	for r := range ch {
		switch r.Kind {
		case scraper.KindScene:
			scenes++
		case scraper.KindStoppedEarly:
			stoppedEarly = true
		case scraper.KindError:
			t.Errorf("unexpected error: %v", r.Err)
		}
	}

	if scenes != 1 {
		t.Errorf("got %d scenes, want 1", scenes)
	}
	if !stoppedEarly {
		t.Error("expected StoppedEarly signal")
	}
}
