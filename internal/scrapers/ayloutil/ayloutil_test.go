package ayloutil

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestParseFilter(t *testing.T) {
	cases := []struct {
		name string
		url  string
		want Filter
	}{
		{"actor-pornstar", "https://www.babes.com/pornstar/12345/some-star", Filter{Type: FilterActor, ID: 12345}},
		{"actor-model", "https://www.babes.com/model/3911/alexis-fawx", Filter{Type: FilterActor, ID: 3911}},
		{"actor-modelprofile", "https://www.digitalplayground.com/modelprofile/153/anya-olsen", Filter{Type: FilterActor, ID: 153}},
		{"category", "https://www.babes.com/category/79/milf", Filter{Type: FilterTag, ID: 79}},
		{"site", "https://www.brazzers.com/site/12/brazzers-exxtra", Filter{Type: FilterCollection, ID: 12}},
		{"series", "https://www.brazzers.com/series/4567/something", Filter{Type: FilterSeries, ID: 4567}},
		{"all", "https://www.babes.com", Filter{Type: FilterAll}},
		{"unmatched-path", "https://www.babes.com/random/page", Filter{Type: FilterAll}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ParseFilter(c.url)
			if got != c.want {
				t.Errorf("ParseFilter(%q) = %+v, want %+v", c.url, got, c.want)
			}
		})
	}
}

func TestSlugify(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"Hello World", "hello-world"},
		{"Mom's Friend!! 4K", "mom-s-friend-4k"},
		{"   leading and trailing   ", "leading-and-trailing"},
		{"---already-dashed---", "already-dashed"},
		{"NoSpecialChars", "nospecialchars"},
		{"", ""},
	}
	for _, c := range cases {
		if got := Slugify(c.in); got != c.want {
			t.Errorf("Slugify(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestParseDate(t *testing.T) {
	cases := []struct {
		in   string
		want time.Time
	}{
		{"2026-01-15T12:00:00+00:00", time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)},
		{"2026-01-15T08:00:00-04:00", time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)},
		{"", time.Time{}},
		{"not-a-date", time.Time{}},
	}
	for _, c := range cases {
		got := ParseDate(c.in)
		if !got.Equal(c.want) {
			t.Errorf("ParseDate(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestIsEmptyJSON(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{``, true},
		{`   `, true},
		{`[]`, true},
		{`{}`, true},
		{`null`, true},
		{` [] `, true},
		{`[1]`, false},
		{`{"a":1}`, false},
		{`"x"`, false},
	}
	for _, c := range cases {
		if got := isEmptyJSON(json.RawMessage(c.in)); got != c.want {
			t.Errorf("isEmptyJSON(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestThumbnailURL(t *testing.T) {
	t.Run("picks first available size in priority order", func(t *testing.T) {
		raw := json.RawMessage(`{
			"poster": {
				"0": {
					"md": {"urls": {"default": "https://cdn.example/md.jpg"}},
					"xl": {"urls": {"default": "https://cdn.example/xl.jpg"}}
				}
			}
		}`)
		got := ThumbnailURL(raw)
		if got != "https://cdn.example/xl.jpg" {
			t.Errorf("got %q, want xl URL", got)
		}
	})

	t.Run("falls back through sizes", func(t *testing.T) {
		raw := json.RawMessage(`{"poster":{"0":{"sm":{"urls":{"default":"https://cdn.example/sm.jpg"}}}}}`)
		if got := ThumbnailURL(raw); got != "https://cdn.example/sm.jpg" {
			t.Errorf("got %q, want sm URL", got)
		}
	})

	t.Run("returns empty on missing poster", func(t *testing.T) {
		if got := ThumbnailURL(json.RawMessage(`{"banner":{}}`)); got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})

	t.Run("returns empty on empty input", func(t *testing.T) {
		if got := ThumbnailURL(json.RawMessage(`[]`)); got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})

	t.Run("returns empty on malformed JSON", func(t *testing.T) {
		if got := ThumbnailURL(json.RawMessage(`{not json`)); got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})
}

func TestPreviewURL(t *testing.T) {
	t.Run("picks highest-resolution mediabook variant", func(t *testing.T) {
		raw := json.RawMessage(`{
			"mediabook": {
				"files": {
					"320p": {"urls": {"view": "https://cdn.example/320.mp4"}},
					"720p": {"urls": {"view": "https://cdn.example/720.mp4"}},
					"480p": {"urls": {"view": "https://cdn.example/480.mp4"}}
				}
			}
		}`)
		if got := PreviewURL(raw); got != "https://cdn.example/720.mp4" {
			t.Errorf("got %q, want 720p URL", got)
		}
	})

	t.Run("returns empty when input is array", func(t *testing.T) {
		if got := PreviewURL(json.RawMessage(`[]`)); got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})

	t.Run("returns empty when mediabook missing", func(t *testing.T) {
		if got := PreviewURL(json.RawMessage(`{"other":"thing"}`)); got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})

	t.Run("skips files with empty view URL", func(t *testing.T) {
		raw := json.RawMessage(`{
			"mediabook": {
				"files": {
					"720p": {"urls": {"view": ""}},
					"480p": {"urls": {"view": "https://cdn.example/480.mp4"}}
				}
			}
		}`)
		if got := PreviewURL(raw); got != "https://cdn.example/480.mp4" {
			t.Errorf("got %q, want 480p URL", got)
		}
	})
}

func TestMediaDuration(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want int
	}{
		{"valid", `{"mediabook":{"length":1234}}`, 1234},
		{"missing mediabook", `{"other":"x"}`, 0},
		{"empty input", `{}`, 0},
		{"array input", `[]`, 0},
		{"malformed", `{not json`, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := MediaDuration(json.RawMessage(c.raw))
			if got != c.want {
				t.Errorf("got %d, want %d", got, c.want)
			}
		})
	}
}

func TestParseHeight(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"720p", 720},
		{"1080p", 1080},
		{"320", 320},
		{"", 0},
		{"abc", 0},
	}
	for _, c := range cases {
		if got := parseHeight(c.in); got != c.want {
			t.Errorf("parseHeight(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestToScene(t *testing.T) {
	cfg := SiteConfig{
		SiteID:     "babes",
		SiteBase:   "https://www.babes.com",
		StudioName: "Babes",
	}
	rel := Release{
		ID:           42,
		Type:         "scene",
		Title:        "Sample Scene Title",
		Description:  "A description.",
		DateReleased: "2026-01-15T12:00:00+00:00",
		Actors: []Actor{
			{ID: 1, Name: "Alice"},
			{ID: 2, Name: "Bob"},
		},
		Tags: []Tag{
			{ID: 10, Name: "MILF"},
			{ID: 11, Name: "POV"},
		},
		Collections: []Collection{
			{ID: 100, Name: "Premium Series"},
		},
		Stats:     Stats{Likes: 50, Views: 1000},
		RawImages: json.RawMessage(`{"poster":{"0":{"xl":{"urls":{"default":"https://cdn.example/p.jpg"}}}}}`),
		RawVideos: json.RawMessage(`{"mediabook":{"length":1800,"files":{"720p":{"urls":{"view":"https://cdn.example/v.mp4"}}}}}`),
	}
	now := time.Date(2026, 4, 25, 10, 0, 0, 0, time.UTC)

	got := ToScene(cfg, "https://www.babes.com/pornstar/1/alice", rel, now)

	if got.ID != "42" {
		t.Errorf("ID = %q, want 42", got.ID)
	}
	if got.SiteID != "babes" {
		t.Errorf("SiteID = %q, want babes", got.SiteID)
	}
	if got.StudioURL != "https://www.babes.com/pornstar/1/alice" {
		t.Errorf("StudioURL = %q", got.StudioURL)
	}
	if got.Title != "Sample Scene Title" {
		t.Errorf("Title = %q", got.Title)
	}
	wantURL := "https://www.babes.com/video/42/sample-scene-title"
	if got.URL != wantURL {
		t.Errorf("URL = %q, want %q", got.URL, wantURL)
	}
	if got.Date.IsZero() {
		t.Error("Date is zero, expected 2026-01-15")
	}
	if got.Description != "A description." {
		t.Errorf("Description = %q", got.Description)
	}
	if got.Thumbnail != "https://cdn.example/p.jpg" {
		t.Errorf("Thumbnail = %q", got.Thumbnail)
	}
	if got.Preview != "https://cdn.example/v.mp4" {
		t.Errorf("Preview = %q", got.Preview)
	}
	if got.Duration != 1800 {
		t.Errorf("Duration = %d, want 1800", got.Duration)
	}
	if len(got.Performers) != 2 || got.Performers[0] != "Alice" || got.Performers[1] != "Bob" {
		t.Errorf("Performers = %v", got.Performers)
	}
	if got.Studio != "Babes" {
		t.Errorf("Studio = %q", got.Studio)
	}
	if len(got.Tags) != 2 || got.Tags[0] != "MILF" || got.Tags[1] != "POV" {
		t.Errorf("Tags = %v", got.Tags)
	}
	if got.Series != "Premium Series" {
		t.Errorf("Series = %q", got.Series)
	}
	if got.Likes != 50 || got.Views != 1000 {
		t.Errorf("Likes/Views = %d/%d", got.Likes, got.Views)
	}
	if !got.ScrapedAt.Equal(now) {
		t.Errorf("ScrapedAt = %v", got.ScrapedAt)
	}
}

func TestToScene_emptyCollections(t *testing.T) {
	cfg := SiteConfig{SiteID: "babes", SiteBase: "https://www.babes.com", StudioName: "Babes"}
	rel := Release{
		ID: 1, Title: "T", DateReleased: "2026-01-15T12:00:00+00:00",
		RawImages: json.RawMessage(`[]`), RawVideos: json.RawMessage(`[]`),
	}
	got := ToScene(cfg, "https://www.babes.com", rel, time.Now())
	if got.Series != "" {
		t.Errorf("Series should be empty for empty collections, got %q", got.Series)
	}
	if got.Thumbnail != "" || got.Preview != "" || got.Duration != 0 {
		t.Errorf("expected empty media fields, got thumb=%q preview=%q dur=%d", got.Thumbnail, got.Preview, got.Duration)
	}
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	defer func() { os.Stderr = orig }()

	fn()

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestWarnParseFailure_logsOncePerOnce(t *testing.T) {
	var once sync.Once
	out := captureStderr(t, func() {
		warnParseFailure(&once, "test-site", json.RawMessage(`first-payload`))
		warnParseFailure(&once, "test-site", json.RawMessage(`second-payload`))
		warnParseFailure(&once, "test-site", json.RawMessage(`third-payload`))
	})

	if got := strings.Count(out, "warning: ayloutil"); got != 1 {
		t.Errorf("expected exactly one warning, got %d:\n%s", got, out)
	}
	if !strings.Contains(out, "first-payload") {
		t.Errorf("expected first payload to be sampled, got: %s", out)
	}
	if strings.Contains(out, "second-payload") || strings.Contains(out, "third-payload") {
		t.Errorf("subsequent payloads must not be logged, got: %s", out)
	}
	if !strings.Contains(out, "test-site") {
		t.Errorf("expected location label in warning, got: %s", out)
	}
}

func TestWarnParseFailure_truncatesLongPayload(t *testing.T) {
	var once sync.Once
	big := bytes.Repeat([]byte("x"), 500)
	out := captureStderr(t, func() {
		warnParseFailure(&once, "big-site", json.RawMessage(big))
	})

	if !strings.Contains(out, "...(truncated)") {
		t.Errorf("expected truncation marker, got: %s", out)
	}
	// We sample 200 chars; the marker adds 14. Total stderr line is small.
	if len(out) > 600 {
		t.Errorf("warning line should be truncated, got %d bytes:\n%s", len(out), out)
	}
}

func TestWarnParseFailure_independentOnces(t *testing.T) {
	var a, b sync.Once
	out := captureStderr(t, func() {
		warnParseFailure(&a, "site-A", json.RawMessage(`{"a":1}`))
		warnParseFailure(&b, "site-B", json.RawMessage(`{"b":2}`))
	})
	if !strings.Contains(out, "site-A") || !strings.Contains(out, "site-B") {
		t.Errorf("each Once should fire independently, got: %s", out)
	}
}
