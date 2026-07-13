package natscmsutil

import (
	"encoding/json"
	"regexp"
	"testing"
	"time"
)

func TestStringOrInt(t *testing.T) {
	cases := []struct {
		raw  string
		want int
	}{
		{`1473`, 1473},
		{`"1473"`, 1473},
		{`""`, 0},
		{`null`, 0},
		{`"abc"`, 0}, // non-numeric strings -> 0, no error
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
		{CMSBlockID: "108854", Settings: blockSetting{Type: "navigation"}},
		{CMSBlockID: "108874", Settings: blockSetting{Type: "carousel"}},
		{CMSBlockID: "108983", Settings: blockSetting{Type: "set_list"}},
		{CMSBlockID: "111044", Settings: blockSetting{Type: "carousel"}},
	}}
	if got := findSetListBlockID(page); got != "108983" {
		t.Errorf("got %q, want 108983", got)
	}
	if got := findSetListBlockID(&pageResponse{}); got != "" {
		t.Errorf("empty page should yield empty, got %q", got)
	}
}

func TestPickThumbnail(t *testing.T) {
	servers := map[string]string{
		"1": "https://c76161b613.mjedge.net", // trailing slash already stripped
	}
	p := previewBlob{Thumb: map[string][]previewItem{
		"200-112": {{CMSContentServerID: "1", FileURI: "/path/.thumb-small.webp", Signature: "expires=1&token=a"}},
		"800-450": {{CMSContentServerID: "1", FileURI: "/path/.thumb-large.webp", Signature: "expires=1&token=b"}},
		"400-225": {{CMSContentServerID: "1", FileURI: "/path/.thumb-mid.webp", Signature: "expires=1&token=c"}},
	}}
	// Largest ratio (800*450 = 360000) should win.
	got := pickThumbnail(p, servers)
	want := "https://c76161b613.mjedge.net/path/.thumb-large.webp?expires=1&token=b"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPickThumbnail_unknownServer(t *testing.T) {
	servers := map[string]string{"2": "https://other.cdn/"}
	p := previewBlob{Thumb: map[string][]previewItem{
		"200-112": {{CMSContentServerID: "1", FileURI: "/x.webp", Signature: "s"}},
	}}
	if got := pickThumbnail(p, servers); got != "" {
		t.Errorf("unknown server should yield empty, got %q", got)
	}
}

func TestPickThumbnail_nilServers(t *testing.T) {
	p := previewBlob{Thumb: map[string][]previewItem{
		"200-112": {{CMSContentServerID: "1", FileURI: "/x.webp"}},
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
		{"1920-1080", 1920, 1080, true},
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

// TestToScene checks the full listing→Scene mapping, including the
// per-config StudioName/SiteName/NatsAPIBase fields and the synthesised
// trailer URL.
func TestToScene(t *testing.T) {
	s := New(SiteConfig{
		ID:         "cosplayground",
		SiteBase:   "https://cosplayground.com",
		SiteName:   "Cosplayground",
		StudioName: "Cosplayground",
		MatchRe:    regexp.MustCompile(`cosplayground\.com`),
	})
	e := setEntry{
		CMSSetID:    "305",
		Name:        "The Odyssey XXX &amp; More",
		Slug:        "the-odyssey-xxx",
		AddedNice:   "2026-07-10",
		MemberViews: 119,
	}
	got := s.toScene(e, "https://cosplayground.com/", nil, time.Now().UTC())
	if got.ID != "305" || got.SiteID != "cosplayground" {
		t.Errorf("id/site mismatch: %+v", got)
	}
	if got.Title != "The Odyssey XXX & More" {
		t.Errorf("title = %q", got.Title)
	}
	if got.Studio != "Cosplayground" || got.Series != "Cosplayground" {
		t.Errorf("studio/series = %q/%q", got.Studio, got.Series)
	}
	if got.URL != "https://cosplayground.com/tour/trailer/the-odyssey-xxx/" {
		t.Errorf("url = %q", got.URL)
	}
	if got.Views != 119 {
		t.Errorf("views = %d", got.Views)
	}
	if got.Date.Year() != 2026 || got.Date.Month() != 7 || got.Date.Day() != 10 {
		t.Errorf("date = %v", got.Date)
	}
}
