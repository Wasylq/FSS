package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Wasylq/FSS/models"
)

func TestSlugify(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"https://www.manyvids.com/Profile/590705/bettie-bondage/Store/Videos", "www-manyvids-com-profile-590705-bettie-bondage-store-videos"},
		{"https://www.kink.com/channel/hogtied", "www-kink-com-channel-hogtied"},
		{"https://tour.pissinghd.com/videos", "tour-pissinghd-com-videos"},
		{"https://www.karupsow.com/videos/", "www-karupsow-com-videos"},
		{"https://example.com", "example-com"},
		{"https://example.com/", "example-com"},
		{"https://www.example.com/path/to/thing", "www-example-com-path-to-thing"},
		{"not-a-url", "not-a-url"},
	}
	for _, tt := range tests {
		got := Slugify(tt.input)
		if got != tt.want {
			t.Errorf("Slugify(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSlugifySpecialChars(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"query string stripped", "https://example.com/videos?page=1&sort=date", "example-com-videos"},
		{"fragment stripped", "https://example.com/videos#top", "example-com-videos"},
		{"port stripped", "https://example.com:8080/videos", "example-com-videos"},
		{"unicode replaced", "https://example.com/vidéos/scène", "example-com-vid-os-sc-ne"},
		{"double hyphens collapsed", "https://example.com/a--b---c", "example-com-a-b-c"},
		{"trailing slash stripped", "https://example.com/path/", "example-com-path"},
		{"empty path", "https://example.com", "example-com"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Slugify(tt.input)
			if got != tt.want {
				t.Errorf("Slugify(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSlugifyEmpty(t *testing.T) {
	got := Slugify("")
	if got != "" {
		t.Errorf("Slugify('') = %q, want empty", got)
	}
}

func TestSlugifyPathTraversal(t *testing.T) {
	got := Slugify("https://evil.com/../../etc/passwd")
	if got != "evil-com-etc-passwd" {
		t.Errorf("Slugify path traversal = %q, want no dots/slashes", got)
	}
}

func newTestFlat(t *testing.T, formats ...string) *Flat {
	t.Helper()
	dir := t.TempDir()
	if len(formats) == 0 {
		formats = []string{"json"}
	}
	return NewFlat(dir, formats)
}

const flatTestURL = "https://www.example.com/videos"

func testScenes(now time.Time) []models.Scene {
	return []models.Scene{
		{
			ID:         "1",
			SiteID:     "example",
			StudioURL:  flatTestURL,
			Title:      "Scene One",
			URL:        "https://www.example.com/video/1",
			Date:       now.Add(-48 * time.Hour),
			Performers: []string{"Alice", "Bob"},
			Studio:     "Example Studio",
			Tags:       []string{"tag1", "tag2"},
			Duration:   1200,
			ScrapedAt:  now,
		},
		{
			ID:        "2",
			SiteID:    "example",
			StudioURL: flatTestURL,
			Title:     "Scene Two",
			URL:       "https://www.example.com/video/2",
			Date:      now.Add(-24 * time.Hour),
			Studio:    "Example Studio",
			ScrapedAt: now,
		},
	}
}

func TestFlatSaveLoad(t *testing.T) {
	f := newTestFlat(t)
	now := time.Now().UTC().Truncate(time.Second)
	scenes := testScenes(now)

	if err := f.Save(flatTestURL, scenes); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := f.Load(flatTestURL)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d scenes, want 2", len(got))
	}
	if got[0].ID != "1" || got[0].Title != "Scene One" {
		t.Errorf("scene 0: id=%q title=%q", got[0].ID, got[0].Title)
	}
	if len(got[0].Performers) != 2 || got[0].Performers[0] != "Alice" {
		t.Errorf("performers = %v", got[0].Performers)
	}
	if got[0].Duration != 1200 {
		t.Errorf("duration = %d", got[0].Duration)
	}
	if got[1].ID != "2" || got[1].Title != "Scene Two" {
		t.Errorf("scene 1: id=%q title=%q", got[1].ID, got[1].Title)
	}
}

func TestFlatLoadNonexistent(t *testing.T) {
	f := newTestFlat(t)
	got, err := f.Load("https://www.nonexistent.com/videos")
	if err != nil {
		t.Fatalf("Load nonexistent: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil, got %d scenes", len(got))
	}
}

func TestFlatLoadCorruptJSON(t *testing.T) {
	f := newTestFlat(t)
	path := f.jsonPath(flatTestURL)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{invalid json"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := f.Load(flatTestURL)
	if err == nil {
		t.Error("expected error for corrupt JSON")
	}
}

func TestFlatSaveCreatesDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "deep")
	f := NewFlat(dir, []string{"json"})
	now := time.Now().UTC().Truncate(time.Second)

	if err := f.Save(flatTestURL, testScenes(now)); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Errorf("dir not created: %v", err)
	}
}

func TestFlatSaveOverwrites(t *testing.T) {
	f := newTestFlat(t)
	now := time.Now().UTC().Truncate(time.Second)

	if err := f.Save(flatTestURL, testScenes(now)); err != nil {
		t.Fatal(err)
	}

	updated := []models.Scene{{
		ID: "3", SiteID: "example", StudioURL: flatTestURL,
		Title: "New Scene", URL: "https://www.example.com/video/3", ScrapedAt: now,
	}}
	if err := f.Save(flatTestURL, updated); err != nil {
		t.Fatal(err)
	}

	got, err := f.Load(flatTestURL)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "3" {
		t.Errorf("expected overwritten data, got %d scenes", len(got))
	}
}

func TestFlatMarkDeleted(t *testing.T) {
	f := newTestFlat(t)
	now := time.Now().UTC().Truncate(time.Second)

	if err := f.Save(flatTestURL, testScenes(now)); err != nil {
		t.Fatal(err)
	}

	if err := f.MarkDeleted(flatTestURL, "test", []string{"1"}); err != nil {
		t.Fatalf("MarkDeleted: %v", err)
	}

	got, err := f.Load(flatTestURL)
	if err != nil {
		t.Fatal(err)
	}
	if got[0].DeletedAt == nil {
		t.Error("scene 1 should be deleted")
	}
	if got[1].DeletedAt != nil {
		t.Error("scene 2 should not be deleted")
	}
}

func TestFlatMarkDeletedIdempotent(t *testing.T) {
	f := newTestFlat(t)
	now := time.Now().UTC().Truncate(time.Second)

	if err := f.Save(flatTestURL, testScenes(now)); err != nil {
		t.Fatal(err)
	}

	if err := f.MarkDeleted(flatTestURL, "test", []string{"1"}); err != nil {
		t.Fatal(err)
	}
	got1, _ := f.Load(flatTestURL)
	firstDeleted := *got1[0].DeletedAt

	if err := f.MarkDeleted(flatTestURL, "test", []string{"1"}); err != nil {
		t.Fatal(err)
	}
	got2, _ := f.Load(flatTestURL)
	if !got2[0].DeletedAt.Equal(firstDeleted) {
		t.Error("second MarkDeleted should not change the timestamp")
	}
}

func TestFlatMarkDeletedNonexistentID(t *testing.T) {
	f := newTestFlat(t)
	now := time.Now().UTC().Truncate(time.Second)

	if err := f.Save(flatTestURL, testScenes(now)); err != nil {
		t.Fatal(err)
	}
	if err := f.MarkDeleted(flatTestURL, "test", []string{"nonexistent"}); err != nil {
		t.Fatal(err)
	}

	got, _ := f.Load(flatTestURL)
	for _, s := range got {
		if s.DeletedAt != nil {
			t.Errorf("scene %s should not be deleted", s.ID)
		}
	}
}

func TestFlatSaveWithCSV(t *testing.T) {
	f := newTestFlat(t, "json", "csv")
	now := time.Now().UTC().Truncate(time.Second)

	if err := f.Save(flatTestURL, testScenes(now)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(f.jsonPath(flatTestURL)); err != nil {
		t.Errorf("JSON file missing: %v", err)
	}
	if _, err := os.Stat(f.csvPath(flatTestURL)); err != nil {
		t.Errorf("CSV file missing: %v", err)
	}
}

func TestFlatJSONStructure(t *testing.T) {
	f := newTestFlat(t)
	now := time.Now().UTC().Truncate(time.Second)

	if err := f.Save(flatTestURL, testScenes(now)); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(f.jsonPath(flatTestURL))
	if err != nil {
		t.Fatal(err)
	}
	var sf studioFile
	if err := json.Unmarshal(data, &sf); err != nil {
		t.Fatalf("JSON structure invalid: %v", err)
	}
	if sf.StudioURL != flatTestURL {
		t.Errorf("studioUrl = %q", sf.StudioURL)
	}
	if sf.SceneCount != 2 {
		t.Errorf("sceneCount = %d", sf.SceneCount)
	}
	if sf.ScrapedAt.IsZero() {
		t.Error("scrapedAt is zero")
	}
}

func TestFlatSaveReadOnlyDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "readonly")
	if err := os.MkdirAll(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(dir, "sub")
	f := NewFlat(nested, []string{"json"})
	now := time.Now().UTC().Truncate(time.Second)

	err := f.Save(flatTestURL, testScenes(now))
	if err == nil {
		t.Error("expected error saving to read-only directory")
	}
}

func TestFlatExportNoop(t *testing.T) {
	f := newTestFlat(t)
	if err := f.Export("json", "/tmp/out.json", flatTestURL); err != nil {
		t.Errorf("Export should be no-op, got: %v", err)
	}
}
