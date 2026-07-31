package aziani

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/scraper"
)

func TestStringOrInt(t *testing.T) {
	cases := []struct {
		raw  string
		want int
	}{
		{`3321`, 3321},
		{`"3321"`, 3321},
		{`""`, 0},
		{`null`, 0},
		{`"abc"`, 0},
		{`0`, 0},
	}
	for _, c := range cases {
		var v stringOrInt
		if err := json.Unmarshal([]byte(c.raw), &v); err != nil {
			t.Errorf("Unmarshal(%q): %v", c.raw, err)
			continue
		}
		if int(v) != c.want {
			t.Errorf("Unmarshal(%q) = %d, want %d", c.raw, int(v), c.want)
		}
	}
}

func TestFindSetListBlockID(t *testing.T) {
	page := &pageResponse{Blocks: []pageBlock{
		{CMSBlockID: "114001", Settings: blockSetting{Type: "navigation"}},
		{CMSBlockID: "114002", Settings: blockSetting{Type: "html"}},
		{CMSBlockID: "114370", Settings: blockSetting{Type: "set_list"}},
		{CMSBlockID: "114003", Settings: blockSetting{Type: "model_list"}},
	}}
	if got := findSetListBlockID(page); got != "114370" {
		t.Errorf("got %q, want 114370", got)
	}
	if got := findSetListBlockID(&pageResponse{}); got != "" {
		t.Errorf("empty page should yield empty, got %q", got)
	}
}

func TestPickThumbnail(t *testing.T) {
	servers := map[string]string{
		"2": "https://c75c0c3063.mjedge.net",
	}
	p := previewBlob{Thumb: map[string][]previewItem{
		"200-112":   {{CMSContentServerID: "2", FileURI: "/path/.thumb-small.webp", Signature: "expires=1&token=a"}},
		"1920-1077": {{CMSContentServerID: "2", FileURI: "/path/.thumb-large.webp", Signature: "expires=1&token=b"}},
		"400-224":   {{CMSContentServerID: "2", FileURI: "/path/.thumb-mid.webp", Signature: "expires=1&token=c"}},
	}}
	got := pickThumbnail(p, servers)
	want := "https://c75c0c3063.mjedge.net/path/.thumb-large.webp?expires=1&token=b"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPickThumbnail_nilServers(t *testing.T) {
	p := previewBlob{Thumb: map[string][]previewItem{
		"200-112": {{CMSContentServerID: "2", FileURI: "/x.webp"}},
	}}
	if got := pickThumbnail(p, nil); got != "" {
		t.Errorf("nil servers should yield empty, got %q", got)
	}
}

func TestParseRatio(t *testing.T) {
	cases := []struct {
		in   string
		w, h int
		ok   bool
	}{
		{"200-112", 200, 112, true},
		{"1920-1077", 1920, 1077, true},
		{"bad", 0, 0, false},
		{"100-", 100, 0, false},
		{"", 0, 0, false},
	}
	for _, c := range cases {
		w, h, ok := parseRatio(c.in)
		if ok != c.ok || w != c.w || h != c.h {
			t.Errorf("parseRatio(%q) = (%d, %d, %v); want (%d, %d, %v)", c.in, w, h, ok, c.w, c.h, c.ok)
		}
	}
}

func TestCleanHTML(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"plain text", "plain text"},
		{"<p>hello <b>world</b></p>", "hello world"},
		{"&amp; &quot;quoted&quot;", "& \"quoted\""},
		{"  multi\nspace\t\t", "multi space"},
		{"", ""},
	}
	for _, c := range cases {
		if got := cleanHTML(c.in); got != c.want {
			t.Errorf("cleanHTML(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSitesTable(t *testing.T) {
	seen := map[string]bool{}
	for _, cfg := range sites {
		if cfg.ID == "" {
			t.Errorf("empty ID in sites table")
		}
		if seen[cfg.ID] {
			t.Errorf("duplicate ID: %q", cfg.ID)
		}
		seen[cfg.ID] = true
		if cfg.CMSAreaID == "" {
			t.Errorf("site %q missing CMSAreaID", cfg.ID)
		}
		if cfg.SiteBase == "" {
			t.Errorf("site %q missing SiteBase", cfg.ID)
		}
		if cfg.MatchRe == nil {
			t.Errorf("site %q missing MatchRe", cfg.ID)
		}
	}
	if len(sites) != 4 {
		t.Errorf("expected 4 sites, got %d", len(sites))
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
		{"aziani", "https://www.aziani.com/", true},
		{"aziani", "https://aziani.com/video/some-slug", true},
		{"aziani", "http://aziani.com/anything", true},
		{"aziani", "https://www.2poles1hole.com/", false},
		{"2poles1hole", "https://2poles1hole.com/", true},
		{"2poles1hole", "https://www.2poles1hole.com/", true},
		{"creampiled", "https://creampiled.com/", true},
		{"popuporgies", "https://popuporgies.com/", true},
		{"aziani", "https://azianifake.com/", false},
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

// --- end to end ---------------------------------------------------------------
//
// The tests above exercise the helpers directly, which left the whole NATS walk
// — ListScenes, run, fetchPageConfig, fetchSets, fetchServers, fetchAPI — at 0%.
// The tour_api endpoint is now a field, so the three-step discovery
// (page config -> set list -> servers) runs offline.
func TestListScenesEndToEnd(t *testing.T) {
	// Handlers run on separate goroutines (Workers > 1), so the request log needs
	// a mutex — a bare append here is a real data race.
	var mu sync.Mutex
	var seen []string
	record := func(p string) {
		mu.Lock()
		defer mu.Unlock()
		seen = append(seen, p)
	}
	snapshot := func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), seen...)
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		record(r.URL.Path + "?" + r.URL.RawQuery)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.RawQuery, "slug=/"):
			_, _ = fmt.Fprint(w, `{"success":true,"slug":"/","name":"Home","blocks":[`+
				`{"cms_block_id":"99","settings":{"type":"other"}},`+
				`{"cms_block_id":"42","settings":{"type":"set_list"}}]}`)
		case strings.Contains(r.URL.Path, "/content/sets"):
			// total_count and member_views arrive as *strings* (hence stringOrInt), and
			// thumb keys are W-H ratios (hyphen, not "x") that parseRatio ranks by area.
			_, _ = fmt.Fprint(w, `{"success":true,"total_count":"2","sets":[`+
				`{"cms_set_id":"1001","name":"First Scene","description":"<p>Desc &amp; more</p>",`+
				`"slug":"first-scene","added_nice":"2026-01-05","member_views":"1234",`+
				`"preview_formatted":{"thumb":{"320-180":[{"cms_content_server_id":"7","fileuri":"/t/1s.jpg","signature":"sig"}],"1280-720":[{"cms_content_server_id":"7","fileuri":"/t/1.jpg","signature":"sig"}]}}},`+
				`{"cms_set_id":"1002","name":"Second Scene","description":"","slug":"second-scene",`+
				`"added_nice":"2026-01-06","member_views":"5",`+
				`"preview_formatted":{"thumb":{"1280-720":[{"cms_content_server_id":"7","fileuri":"/t/2.jpg","signature":"sig"}]}}}]}`)
		case strings.Contains(r.URL.Path, "/content/servers"):
			_, _ = fmt.Fprint(w, `{"success":true,"servers":[{"cms_content_server_id":"7","settings":{"url":"https://cdn.example/"}}]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	cfg := SiteConfig{ID: "aziani", SiteBase: "https://azianistudios.com", SiteName: "Aziani", CMSAreaID: "1"}
	s := New(cfg)
	s.client = ts.Client()
	s.apiBase = ts.URL

	ch, err := s.ListScenes(context.Background(), cfg.SiteBase, scraper.ListOpts{Workers: 2})
	if err != nil {
		t.Fatal(err)
	}
	scenes := testutil.CollectScenes(t, ch)
	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2 (requests: %v)", len(scenes), snapshot())
	}

	// All three discovery steps must have run.
	joined := strings.Join(snapshot(), " ")
	for _, want := range []string{"slug=/", "/content/sets", "/content/servers"} {
		if !strings.Contains(joined, want) {
			t.Errorf("never requested %q; requests were %v", want, snapshot())
		}
	}

	var first *models.Scene
	for i := range scenes {
		if scenes[i].Title == "First Scene" {
			first = &scenes[i]
		}
	}
	if first == nil {
		t.Fatalf("scene \"First Scene\" missing; got %+v", scenes)
	}
	if first.ID != "1001" {
		t.Errorf("ID = %q, want 1001", first.ID)
	}
	if first.Date.Format("2006-01-02") != "2026-01-05" {
		t.Errorf("Date = %v, want 2026-01-05 (added_nice)", first.Date)
	}
	// member_views is a JSON string; stringOrInt must decode it.
	if first.Views != 1234 {
		t.Errorf("Views = %d, want 1234 (member_views as a string)", first.Views)
	}
	// Description arrives as HTML and is cleaned.
	if strings.Contains(first.Description, "<p>") {
		t.Errorf("Description still contains HTML: %q", first.Description)
	}
	// The thumbnail is assembled from the servers response.
	if first.Thumbnail == "" {
		t.Error("Thumbnail is empty — the servers lookup did not feed through")
	}
	if !strings.HasPrefix(first.URL, cfg.SiteBase) {
		t.Errorf("URL = %q, want it under SiteBase", first.URL)
	}
}

// Config table integrity — see testutil.CheckSiteTable for what this catches and
// why a config-only table needs it even when the *util carries the parsing tests.
func TestSiteTableIntegrity(t *testing.T) {
	rows := make([]testutil.SiteRow, 0, len(sites))
	for _, c := range sites {
		rows = append(rows, testutil.SiteRow{
			ID: c.ID, Base: c.SiteBase, Studio: c.SiteName,
			Patterns: c.Patterns, MatchRe: c.MatchRe,
		})
	}
	testutil.CheckSiteTable(t, rows)
}
