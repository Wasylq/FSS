package aziani

import (
	"encoding/json"
	"testing"
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
