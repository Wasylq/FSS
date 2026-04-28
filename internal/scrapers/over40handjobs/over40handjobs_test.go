package over40handjobs

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
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
		{"https://www.over40handjobs.com/updates.htm", true},
		{"https://over40handjobs.com/updates.htm", true},
		{"https://www.over40handjobs.com/models/stacie-starr.html", true},
		{"https://www.over40handjobs.com/videos/some-scene.html", true},
		{"https://example.com/over40handjobs", false},
		{"", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.want {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.want)
		}
	}
}

func TestStripNATS(t *testing.T) {
	cases := []struct {
		input, want string
	}{
		{"/videos/foo.html?nats=MC4wLjUuOS4wLjAuMC4wLjA", "/videos/foo.html"},
		{"/videos/foo.html", "/videos/foo.html"},
		{"https://www.over40handjobs.com/models/bar.html?nats=abc", "https://www.over40handjobs.com/models/bar.html"},
	}
	for _, c := range cases {
		if got := stripNATS(c.input); got != c.want {
			t.Errorf("stripNATS(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

func TestParseDate(t *testing.T) {
	cases := []struct {
		input string
		want  time.Time
	}{
		{"March 19, 2026", time.Date(2026, 3, 19, 0, 0, 0, 0, time.UTC)},
		{"January 17, 2020,", time.Date(2020, 1, 17, 0, 0, 0, 0, time.UTC)},
		{"November 11, 2025", time.Date(2025, 11, 11, 0, 0, 0, 0, time.UTC)},
		{"  September 5, 2011, ", time.Date(2011, 9, 5, 0, 0, 0, 0, time.UTC)},
		{"", time.Time{}},
	}
	for _, c := range cases {
		if got := parseDate(c.input); !got.Equal(c.want) {
			t.Errorf("parseDate(%q) = %v, want %v", c.input, got, c.want)
		}
	}
}

func TestParseDuration(t *testing.T) {
	cases := []struct {
		input string
		want  int
	}{
		{"9:01", 541},
		{"22:24", 1344},
		{"0:30", 30},
		{"", 0},
	}
	for _, c := range cases {
		if got := parseDuration(c.input); got != c.want {
			t.Errorf("parseDuration(%q) = %d, want %d", c.input, got, c.want)
		}
	}
}

func TestParseListingPage(t *testing.T) {
	body := []byte(`<div class="updateArea">
<div class="updateBlock clear">
	<div class="updatePic"><a href="/videos/scene-one.html?nats=abc"><img src="/content/thumb1.jpg" alt="Scene One"></a></div>
	<div class="updateDetails clear">
		<h3><a href="/videos/scene-one.html?nats=abc" title="Watch Scene One">Scene One</a></h3>
		<h4>Date: March 19, 2026<br />9:01 HD Video / 69 Pictures</h4>
		<p>First scene description.</p>
		<div class="fullAccess"><a href="#">Watch</a></div>
	</div>
</div>
<div class="updateBlock clear">
	<div class="updatePic"><a href="/videos/scene-two.html?nats=abc"><img src="https://cdn.example.com/thumb2.jpg" alt="Scene Two"></a></div>
	<div class="updateDetails clear">
		<h3><a href="/videos/scene-two.html?nats=abc" title="Watch Scene Two">Scene Two</a></h3>
		<h4>Date: January 17, 2020,<br />06:51 HD Video / 50 Pictures</h4>
		<p>Second scene description.</p>
		<div class="fullAccess"><a href="#">Watch</a></div>
	</div>
</div>
</div>`)

	entries := parseListingPage(body)
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}

	e := entries[0]
	if e.slug != "scene-one" {
		t.Errorf("slug = %q, want scene-one", e.slug)
	}
	if e.title != "Scene One" {
		t.Errorf("title = %q", e.title)
	}
	wantDate := time.Date(2026, 3, 19, 0, 0, 0, 0, time.UTC)
	if !e.date.Equal(wantDate) {
		t.Errorf("date = %v, want %v", e.date, wantDate)
	}
	if e.duration != 541 {
		t.Errorf("duration = %d, want 541", e.duration)
	}
	if e.desc != "First scene description." {
		t.Errorf("desc = %q", e.desc)
	}
	if e.thumb != "/content/thumb1.jpg" {
		t.Errorf("thumb = %q", e.thumb)
	}

	e2 := entries[1]
	if e2.slug != "scene-two" {
		t.Errorf("slug = %q, want scene-two", e2.slug)
	}
	wantDate2 := time.Date(2020, 1, 17, 0, 0, 0, 0, time.UTC)
	if !e2.date.Equal(wantDate2) {
		t.Errorf("date = %v, want %v", e2.date, wantDate2)
	}
	if e2.duration != 411 {
		t.Errorf("duration = %d, want 411", e2.duration)
	}
	if e2.thumb != "https://cdn.example.com/thumb2.jpg" {
		t.Errorf("thumb = %q", e2.thumb)
	}
}

func TestParseDetailPageSinglePerformer(t *testing.T) {
	body := []byte(`<div class="featuringWrapper">Featuring <a href="/models/addyson-james.html?nats=abc">Addyson James</a></div>`)
	performers := parseDetailPage(body)
	if len(performers) != 1 || performers[0] != "Addyson James" {
		t.Errorf("performers = %v, want [Addyson James]", performers)
	}
}

func TestParseDetailPageMultiplePerformers(t *testing.T) {
	body := []byte(`<div class="featuringWrapper">Featuring <a href="/models/alice.html?nats=x">Alice</a> &amp; <a href="/models/bob.html?nats=x">Bob</a> &amp; <a href="/models/carol.html?nats=x">Carol</a></div>`)
	performers := parseDetailPage(body)
	if len(performers) != 3 {
		t.Fatalf("got %d performers, want 3", len(performers))
	}
	if performers[0] != "Alice" || performers[1] != "Bob" || performers[2] != "Carol" {
		t.Errorf("performers = %v", performers)
	}
}

func TestParseDetailPageNoPerformer(t *testing.T) {
	body := []byte(`<html><body><h2>Scene Title</h2></body></html>`)
	performers := parseDetailPage(body)
	if len(performers) != 0 {
		t.Errorf("performers = %v, want empty", performers)
	}
}

const listingTpl = `<div class="updateBlock clear">
	<div class="updatePic"><a href="/videos/scene-%d.html?nats=x"><img src="/content/thumb-%d.jpg" alt="Scene %d"></a></div>
	<div class="updateDetails clear">
		<h3><a href="/videos/scene-%d.html?nats=x">Scene %d</a></h3>
		<h4>Date: January %d, 2026<br />10:00 HD Video / 50 Pictures</h4>
		<p>Description for scene %d.</p>
		<div class="fullAccess"><a href="#">Watch</a></div>
	</div>
</div>`

const detailTpl = `<html><head><title>Scene %d - Over 40 Handjobs</title></head>
<body>
<h2 class="section__title">Scene %d</h2>
<div class="featuringWrapper">Featuring <a href="/models/model-%d.html?nats=x">Model %d</a></div>
</body></html>`

func buildListingPage(ids []int) []byte {
	var sb strings.Builder
	sb.WriteString(`<div class="updateArea">`)
	for _, id := range ids {
		fmt.Fprintf(&sb, listingTpl, id, id, id, id, id, id, id)
	}
	sb.WriteString(`</div>`)
	return []byte(sb.String())
}

func newTestServer(pages map[int][]int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")

		switch {
		case r.URL.Path == "/updates.htm":
			if ids, ok := pages[1]; ok {
				_, _ = w.Write(buildListingPage(ids))
			}
		case strings.HasPrefix(r.URL.Path, "/updates_"):
			var page int
			_, _ = fmt.Sscanf(r.URL.Path, "/updates_%d.html", &page)
			if ids, ok := pages[page]; ok {
				_, _ = w.Write(buildListingPage(ids))
			}
		case strings.HasPrefix(r.URL.Path, "/videos/scene-"):
			var id int
			_, _ = fmt.Sscanf(r.URL.Path, "/videos/scene-%d.html", &id)
			_, _ = fmt.Fprintf(w, detailTpl, id, id, id, id)
		case strings.HasPrefix(r.URL.Path, "/models/"):
			if ids, ok := pages[-1]; ok {
				_, _ = w.Write(buildListingPage(ids))
			}
		default:
			_, _ = fmt.Fprint(w, `<html><body></body></html>`)
		}
	}))
}

func TestListScenes(t *testing.T) {
	ts := newTestServer(map[int][]int{
		1: {3, 2, 1},
	})
	defer ts.Close()

	s := &Scraper{client: ts.Client(), base: ts.URL}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/updates.htm", scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}

	results := testutil.CollectScenes(t, ch)
	if len(results) != 3 {
		t.Fatalf("got %d scenes, want 3", len(results))
	}

	for _, sc := range results {
		if sc.Studio != "Over 40 Handjobs" {
			t.Errorf("studio = %q", sc.Studio)
		}
		if len(sc.Performers) != 1 {
			t.Errorf("scene %s: performers = %v", sc.ID, sc.Performers)
		}
		if !strings.HasPrefix(sc.Thumbnail, ts.URL) {
			t.Errorf("scene %s: thumbnail = %q, want prefix %s", sc.ID, sc.Thumbnail, ts.URL)
		}
	}
}

func TestListScenesPagination(t *testing.T) {
	ts := newTestServer(map[int][]int{
		1: {5, 4},
		2: {3, 2},
		3: {1},
	})
	defer ts.Close()

	s := &Scraper{client: ts.Client(), base: ts.URL}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/updates.htm", scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}

	results := testutil.CollectScenes(t, ch)
	if len(results) != 5 {
		t.Fatalf("got %d scenes, want 5", len(results))
	}
}

func TestListScenesKnownIDs(t *testing.T) {
	ts := newTestServer(map[int][]int{
		1: {5, 4, 3, 2, 1},
	})
	defer ts.Close()

	s := &Scraper{client: ts.Client(), base: ts.URL}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/updates.htm", scraper.ListOpts{
		KnownIDs: map[string]bool{"scene-3": true},
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
	ts := newTestServer(map[int][]int{
		-1: {3, 2, 1},
	})
	defer ts.Close()

	s := &Scraper{client: ts.Client(), base: ts.URL}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/models/test-model.html", scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}

	results := testutil.CollectScenes(t, ch)
	if len(results) != 3 {
		t.Fatalf("got %d scenes, want 3", len(results))
	}
}
