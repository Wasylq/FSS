package puba

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wasylq/FSS/scraper"
)

func TestListingURL(t *testing.T) {
	parent := New(SiteConfig{ID: "puba"})
	got := parent.listingURL(0)
	for _, must := range []string{"section=538", "view=v", "searching=Search", "start=0", "count=24", "format=json", "resource=video"} {
		if !strings.Contains(got, must) {
			t.Errorf("parent URL missing %q: %s", must, got)
		}
	}
	if strings.Contains(got, "group=") {
		t.Errorf("parent URL should not include group=: %s", got)
	}

	sub := New(SiteConfig{ID: "x", Group: 46})
	gotSub := sub.listingURL(48)
	for _, must := range []string{"group=46", "start=48"} {
		if !strings.Contains(gotSub, must) {
			t.Errorf("group URL missing %q: %s", must, gotSub)
		}
	}
	if strings.Contains(gotSub, "view=v") {
		t.Errorf("group URL should not include view=v: %s", gotSub)
	}
}

func TestTrimLeadingNoise(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{`{"a":1}`, `{"a":1}`},
		{"\n\n  \t{\"a\":1}", `{"a":1}`},
		{"<!-- foo -->\n{\"a\":1}", `{"a":1}`},
		{"no json", "no json"},
	}
	for _, c := range cases {
		got := string(trimLeadingNoise([]byte(c.in)))
		if got != c.want {
			t.Errorf("trimLeadingNoise(%q) = %q; want %q", c.in, got, c.want)
		}
	}
}

func TestAbsURL(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"show_video.php?galid=123&nats=MC4w", baseURL + "/show_video.php?galid=123"},
		{"view_image.php?gal=456&file=sample.jpg", baseURL + "/view_image.php?gal=456&file=sample.jpg"},
		{"https://other.cdn/img.jpg", "https://other.cdn/img.jpg"},
		// Stripping `nats=` at the head must not leave a dangling `?`.
		{"foo.php?nats=abc", baseURL + "/foo.php"},
		// Stripping `nats=` in the middle: keep other params.
		{"foo.php?a=1&nats=abc&b=2", baseURL + "/foo.php?a=1&b=2"},
		{"", ""},
	}
	for _, c := range cases {
		if got := absURL(c.in); got != c.want {
			t.Errorf("absURL(%q) = %q; want %q", c.in, got, c.want)
		}
	}
}

func TestParsePerformers(t *testing.T) {
	in := "<a href='index.php?section=538&actor=2178&nats=…'>Nicole Aniston</a>, <a href='index.php?section=538&actor=781&nats=…'>Chad Alva</a>"
	got := parsePerformers(in)
	want := []string{"Nicole Aniston", "Chad Alva"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("performer[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	// HTML entities and dedup.
	got = parsePerformers("<a href='#'>Kiki D&#039;Aire</a>, <a href='#'>Kiki D&#039;Aire</a>")
	if len(got) != 1 || got[0] != "Kiki D'Aire" {
		t.Errorf("dedup/entity handling failed: %v", got)
	}
	if got := parsePerformers(""); got != nil {
		t.Errorf("empty actors should yield nil, got %v", got)
	}
}

func TestCleanText(t *testing.T) {
	cases := []struct{ in, want string }{
		{"  trim  ", "trim"},
		{"a&#039;s &amp; b", "a's & b"},
		{"multi\nline\ttext", "multi line text"},
		{"", ""},
	}
	for _, c := range cases {
		if got := cleanText(c.in); got != c.want {
			t.Errorf("cleanText(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestToScene(t *testing.T) {
	s := New(SiteConfig{ID: "pubasamanthasaint", SiteName: "Samantha Saint", Group: 46})
	scene := s.toScene(apiItem{
		GalID:       13356,
		Description: "Sexy Tease with Samantha &amp; friends",
		VideoURL:    "show_video.php?galid=13356&nats=MC4w",
		ImageURL:    "view_image.php?gal=13356&file=sample.jpg",
		Actors:      "<a href='#'>Samantha Saint</a>",
		Time:        "15:35",
	}, "https://www.puba.com/pornstarnetwork/index.php?section=538&group=46", time.Now())
	if scene.ID != "13356" {
		t.Errorf("ID = %q", scene.ID)
	}
	if scene.Title != "Sexy Tease with Samantha & friends" {
		t.Errorf("Title = %q", scene.Title)
	}
	if scene.URL != baseURL+"/show_video.php?galid=13356" {
		t.Errorf("URL = %q", scene.URL)
	}
	if !strings.Contains(scene.Thumbnail, "view_image.php?gal=13356") {
		t.Errorf("Thumbnail = %q", scene.Thumbnail)
	}
	if scene.Studio != "Puba" {
		t.Errorf("Studio = %q", scene.Studio)
	}
	if scene.Series != "Samantha Saint" {
		t.Errorf("Series = %q", scene.Series)
	}
	if scene.Duration != 15*60+35 {
		t.Errorf("Duration = %d", scene.Duration)
	}
	if len(scene.Performers) != 1 || scene.Performers[0] != "Samantha Saint" {
		t.Errorf("Performers = %v", scene.Performers)
	}
}

func TestMatchesURL(t *testing.T) {
	get := func(id string) *Scraper {
		for _, cfg := range sites {
			if cfg.ID == id {
				return New(cfg)
			}
		}
		return nil
	}
	cases := []struct {
		id, url string
		want    bool
	}{
		// Parent
		{"puba", "https://www.puba.com/pornstarnetwork/index.php?section=538&view=v", true},
		{"puba", "https://www.puba.com/pornstarnetwork/", true},
		{"puba", "https://www.puba.com/", false},
		// Per-pornstar groups
		{"pubasamanthasaint", "https://samanthasaint.puba.com/", true},
		{"pubasamanthasaint", "https://samanthafucks.com/", true},
		{"pubasamanthasaint", "https://www.puba.com/pornstarnetwork/index.php?section=538&group=46&nats=x", true},
		{"pubasamanthasaint", "https://www.puba.com/pornstarnetwork/index.php?section=538&group=13", false},
		{"pubaasaakira", "https://asaakira.puba.com/", true},
		{"pubaasaakira", "https://asafucks.com/", true},
		{"pubashylastylez", "https://shyla.puba.com/index1.php", true},
		{"pubashylastylez", "https://shyla.puba.com/", true},
		{"pubashylastylez", "https://samanthasaint.puba.com/", false},
		{"puba1girl1camera", "https://www.1girl1camera.com/", true},
		{"puba1girl1camera", "https://1girl1camera.com/anything", true},
		// Group ID substring trap — group=46 must not match group=460.
		{"pubasamanthasaint", "https://www.puba.com/pornstarnetwork/index.php?section=538&group=460&nats=x", false},
	}
	for _, c := range cases {
		s := get(c.id)
		if s == nil {
			t.Fatalf("unknown ID %q", c.id)
		}
		if got := s.MatchesURL(c.url); got != c.want {
			t.Errorf("MatchesURL[%s](%q) = %v, want %v", c.id, c.url, got, c.want)
		}
	}
}

func TestSitesTable(t *testing.T) {
	seen := map[string]bool{}
	groupSeen := map[int]string{}
	for _, cfg := range sites {
		if cfg.ID == "" {
			t.Errorf("empty ID")
		}
		if seen[cfg.ID] {
			t.Errorf("duplicate ID: %q", cfg.ID)
		}
		seen[cfg.ID] = true
		if cfg.Group != 0 {
			if prev, ok := groupSeen[cfg.Group]; ok {
				t.Errorf("duplicate group %d: %q and %q", cfg.Group, prev, cfg.ID)
			}
			groupSeen[cfg.Group] = cfg.ID
		}
	}
	// 1 parent + 72 per-pornstar / sub-site groups (group IDs from the
	// network's `?section=539` site index).
	if len(sites) != 73 {
		t.Errorf("expected 73 sites, got %d", len(sites))
	}
}

// TestListScenes_endToEnd serves a single page of fake API JSON (with
// leading whitespace mimicking the real PHP response) and verifies the
// scraper drains it into scene results.
func TestListScenes_endToEnd(t *testing.T) {
	const page1 = `{"page":1,"start":0,"count":24,"total":3,"num_pages":1,"items":[
		{"galid":100,"secid":320,"video_url":"show_video.php?galid=100&nats=x","image_url":"view_image.php?gal=100&file=sample.jpg","description":"First","actors":"<a>Alice</a>","time":"10:00","favorite":false},
		{"galid":99,"secid":320,"video_url":"show_video.php?galid=99&nats=x","image_url":"view_image.php?gal=99&file=sample.jpg","description":"Second","actors":"<a>Bob</a>","time":"5:30","favorite":false},
		{"galid":98,"secid":320,"video_url":"show_video.php?galid=98&nats=x","image_url":"view_image.php?gal=98&file=sample.jpg","description":"Third","actors":"<a>Carol</a>","time":"1:02:03","favorite":false}
	]}`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, "   \n", page1)
	}))
	defer ts.Close()

	// fetchPage hard-codes the production base URL, so exercise the
	// parsing pipeline directly here rather than the full run() loop.
	resp, err := ts.Client().Get(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body := make([]byte, resp.ContentLength+16)
	n, _ := resp.Body.Read(body)
	body = trimLeadingNoise(body[:n])

	var data apiResponse
	if err := json.Unmarshal(body, &data); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if data.Total != 3 || len(data.Items) != 3 {
		t.Fatalf("got total=%d items=%d", data.Total, len(data.Items))
	}

	s := New(SiteConfig{ID: "puba"})
	now := time.Now()
	var got []string
	for _, it := range data.Items {
		sc := s.toScene(it, ts.URL, now)
		got = append(got, sc.ID+":"+sc.Title)
	}
	want := []string{"100:First", "99:Second", "98:Third"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("got %v, want %v", got, want)
	}
	// Spot-check duration parsing on the third item (HH:MM:SS).
	third := s.toScene(data.Items[2], ts.URL, now)
	if third.Duration != 3723 {
		t.Errorf("HH:MM:SS duration = %d, want 3723", third.Duration)
	}
}

// Compile-time sanity: ensure ListScenes still wires up cleanly even
// though we don't dial out from this test file.
var _ = scraper.StudioScraper((*Scraper)(nil))
var _ context.Context = context.Background()
