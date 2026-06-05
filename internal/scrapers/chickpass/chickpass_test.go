package chickpass

import (
	"testing"
)

func TestFindSetListBlockID(t *testing.T) {
	page := &pageResponse{Blocks: []pageBlock{
		{CMSBlockID: "100001", Settings: blockSetting{Type: "navigation"}},
		{CMSBlockID: "100002", Settings: blockSetting{Type: "set_list"}},
	}}
	if got := findSetListBlockID(page); got != "100002" {
		t.Errorf("got %q, want 100002", got)
	}
	if got := findSetListBlockID(&pageResponse{}); got != "" {
		t.Errorf("empty page should yield empty, got %q", got)
	}
}

func TestExtractDataTypes(t *testing.T) {
	dts := []dataType{
		{Type: "Models", Values: []dataValue{{Name: "Alice", Slug: "alice"}, {Name: "Bob", Slug: "bob"}}},
		{Type: "Category", Values: []dataValue{{Name: "Amateur", Slug: "amateur"}, {Name: "Blonde", Slug: "blonde"}}},
		{Type: "Other", Values: []dataValue{{Name: "Ignored"}}},
	}
	performers, tags := extractDataTypes(dts)
	if len(performers) != 2 || performers[0] != "Alice" || performers[1] != "Bob" {
		t.Errorf("performers = %v", performers)
	}
	if len(tags) != 2 || tags[0] != "Amateur" || tags[1] != "Blonde" {
		t.Errorf("tags = %v", tags)
	}
}

func TestExtractDataTypesEmpty(t *testing.T) {
	performers, tags := extractDataTypes(nil)
	if len(performers) != 0 || len(tags) != 0 {
		t.Errorf("expected empty, got performers=%v, tags=%v", performers, tags)
	}
}

func TestPickThumbnail(t *testing.T) {
	servers := map[string]string{
		"5": "https://cdn.example.com",
	}
	p := previewBlob{Thumb: map[string][]previewItem{
		"200-112":   {{CMSContentServerID: "5", FileURI: "/small.webp", Signature: "t=1"}},
		"1920-1077": {{CMSContentServerID: "5", FileURI: "/large.webp", Signature: "t=2"}},
	}}
	got := pickThumbnail(p, servers)
	want := "https://cdn.example.com/large.webp?t=2"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPickThumbnailNilServers(t *testing.T) {
	p := previewBlob{Thumb: map[string][]previewItem{
		"200-112": {{CMSContentServerID: "5", FileURI: "/x.webp"}},
	}}
	if got := pickThumbnail(p, nil); got != "" {
		t.Errorf("nil servers should yield empty, got %q", got)
	}
}

func TestCleanHTML(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"plain text", "plain text"},
		{"<p>hello <b>world</b></p>", "hello world"},
		{"&amp; &quot;quoted&quot;", "& \"quoted\""},
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
	if len(sites) != 10 {
		t.Errorf("expected 10 sites, got %d", len(sites))
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
		{"chickpass", "https://www.chickpass.com/", true},
		{"chickpass", "https://chickpass.com/video/some-slug", true},
		{"chickpass", "https://www.chickpassnetwork.com/", true},
		{"chickpass", "https://chickpassnetwork.com/", true},
		{"bouncychicks", "https://www.bouncychicks.com/", true},
		{"bouncychicks", "https://bouncychicks.com/", true},
		{"fuckthegeek", "https://www.fuckthegeek.com/", true},
		{"minimuff", "https://www.minimuff.com/", true},
		{"xxxnj", "https://www.xxxnj.com/", true},
		{"chickpass", "https://example.com/", false},
		{"chickpass", "https://www.bouncychicks.com/", false},
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
