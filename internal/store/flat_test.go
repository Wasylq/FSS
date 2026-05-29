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

	if err := f.MarkDeleted(flatTestURL, "example", []string{"1"}); err != nil {
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

	if err := f.MarkDeleted(flatTestURL, "example", []string{"1"}); err != nil {
		t.Fatal(err)
	}
	got1, _ := f.Load(flatTestURL)
	firstDeleted := *got1[0].DeletedAt

	if err := f.MarkDeleted(flatTestURL, "example", []string{"1"}); err != nil {
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
	if err := f.MarkDeleted(flatTestURL, "example", []string{"nonexistent"}); err != nil {
		t.Fatal(err)
	}

	got, _ := f.Load(flatTestURL)
	for _, s := range got {
		if s.DeletedAt != nil {
			t.Errorf("scene %s should not be deleted", s.ID)
		}
	}
}

// TestFlatSaveRejectsEmptyKeyFields mirrors the SQLite store guard —
// catching the missing required fields at the store boundary instead
// of writing a JSON file whose entries have empty (ID, SiteID) pairs
// that would never round-trip correctly.
func TestFlatSaveRejectsEmptyKeyFields(t *testing.T) {
	f := newTestFlat(t)
	now := time.Now().UTC()

	emptyID := models.Scene{ID: "", SiteID: "example", StudioURL: flatTestURL, Title: "x", ScrapedAt: now}
	if err := f.Save(flatTestURL, []models.Scene{emptyID}); err == nil {
		t.Errorf("Save with empty ID should error")
	}

	emptySite := models.Scene{ID: "1", SiteID: "", StudioURL: flatTestURL, Title: "y", ScrapedAt: now}
	if err := f.Save(flatTestURL, []models.Scene{emptySite}); err == nil {
		t.Errorf("Save with empty SiteID should error")
	}

	// No JSON file should have been created.
	loaded, _ := f.Load(flatTestURL)
	if len(loaded) != 0 {
		t.Errorf("rejected Save still wrote: got %d scenes", len(loaded))
	}
}

// TestFlatSaveAutoRevives locks in the documented Save contract: a
// re-emitted scene with DeletedAt == nil clears any prior soft-delete.
// Matches SQLite — see the cross-store contract documented on
// `store.Store.Save`.
func TestFlatSaveAutoRevives(t *testing.T) {
	f := newTestFlat(t)
	now := time.Now().UTC().Truncate(time.Second)

	scene := models.Scene{
		ID: "1", SiteID: "example", StudioURL: flatTestURL,
		Title: "A", ScrapedAt: now,
	}
	if err := f.Save(flatTestURL, []models.Scene{scene}); err != nil {
		t.Fatal(err)
	}
	if err := f.MarkDeleted(flatTestURL, "example", []string{"1"}); err != nil {
		t.Fatal(err)
	}
	loaded, _ := f.Load(flatTestURL)
	if loaded[0].DeletedAt == nil {
		t.Fatal("setup: scene should be soft-deleted after MarkDeleted")
	}

	// Re-emit the same scene with DeletedAt == nil — auto-revive.
	revived := scene
	revived.Title = "A (back)"
	if err := f.Save(flatTestURL, []models.Scene{revived}); err != nil {
		t.Fatal(err)
	}
	loaded, _ = f.Load(flatTestURL)
	if len(loaded) != 1 || loaded[0].DeletedAt != nil {
		t.Errorf("Save with DeletedAt=nil should auto-revive, got %+v", loaded)
	}
	if loaded[0].Title != "A (back)" {
		t.Errorf("Title not updated: %q", loaded[0].Title)
	}
}

// TestFlatMarkDeletedSiteIDScoped guards against the previous bug where
// `MarkDeleted` ignored its `siteID` argument and would soft-delete
// every scene with a matching ID, including scenes from a different
// site that happened to share the ID. Studio files produced by
// cross-site stash merges can hold overlapping IDs across SiteIDs.
func TestFlatMarkDeletedSiteIDScoped(t *testing.T) {
	f := newTestFlat(t)
	now := time.Now().UTC().Truncate(time.Second)

	mixed := []models.Scene{
		{ID: "1", SiteID: "example", StudioURL: flatTestURL, Title: "from example", ScrapedAt: now},
		{ID: "1", SiteID: "other", StudioURL: flatTestURL, Title: "from other", ScrapedAt: now},
	}
	if err := f.Save(flatTestURL, mixed); err != nil {
		t.Fatal(err)
	}

	if err := f.MarkDeleted(flatTestURL, "example", []string{"1"}); err != nil {
		t.Fatal(err)
	}

	got, err := f.Load(flatTestURL)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d scenes after MarkDeleted, want 2", len(got))
	}
	for _, s := range got {
		switch s.SiteID {
		case "example":
			if s.DeletedAt == nil {
				t.Errorf("example/%s should be soft-deleted", s.ID)
			}
		case "other":
			if s.DeletedAt != nil {
				t.Errorf("other/%s should NOT be soft-deleted — different SiteID", s.ID)
			}
		}
	}

	// MarkDeleted with a SiteID nobody owns must be a no-op.
	if err := f.MarkDeleted(flatTestURL, "nobody", []string{"1"}); err != nil {
		t.Fatal(err)
	}
	got, _ = f.Load(flatTestURL)
	for _, s := range got {
		if s.SiteID == "other" && s.DeletedAt != nil {
			t.Errorf("other/%s should still NOT be deleted after MarkDeleted(nobody)", s.ID)
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
	var sf models.StudioFile
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

// TestFlatSlugCollision pins AUDIT.md §Store #4: two distinct studio URLs
// can sanitize to the same slug. The pre-fix store silently overwrote one
// studio's data with the other on Save and silently returned the wrong
// scenes on Load. The fix detects the mismatch via the stored StudioURL
// and errors out — non-breaking for users with non-colliding URLs.
func TestFlatSlugCollision(t *testing.T) {
	collisionPairs := [][2]string{
		// hyphen vs slash
		{"https://example.com/foo-bar", "https://example.com/foo/bar"},
		// case difference
		{"https://example.com/Foo", "https://example.com/foo"},
		// query string ignored by Slugify
		{"https://example.com/videos?page=1", "https://example.com/videos?page=2"},
	}

	for _, pair := range collisionPairs {
		urlA, urlB := pair[0], pair[1]
		t.Run(urlA+"_vs_"+urlB, func(t *testing.T) {
			// Sanity: confirm these still collide under the current Slugify.
			if Slugify(urlA) != Slugify(urlB) {
				t.Fatalf("test fixture no longer collides: %q vs %q", urlA, urlB)
			}

			f := newTestFlat(t)
			now := time.Now().UTC().Truncate(time.Second)

			// First studio saves cleanly.
			scenesA := []models.Scene{{ID: "A", SiteID: "a", StudioURL: urlA, Title: "from A", ScrapedAt: now}}
			if err := f.Save(urlA, scenesA); err != nil {
				t.Fatalf("first Save: %v", err)
			}

			// Second studio attempts to write to the same slug — must error.
			scenesB := []models.Scene{{ID: "B", SiteID: "b", StudioURL: urlB, Title: "from B", ScrapedAt: now}}
			if err := f.Save(urlB, scenesB); err == nil {
				t.Fatal("Save with colliding URL should error, got nil")
			}

			// First studio's data must survive.
			got, err := f.Load(urlA)
			if err != nil {
				t.Fatalf("Load(urlA): %v", err)
			}
			if len(got) != 1 || got[0].ID != "A" {
				t.Errorf("urlA data clobbered: got %+v", got)
			}

			// Load from the colliding URL must also error.
			if _, err := f.Load(urlB); err == nil {
				t.Error("Load with colliding URL should error, got nil")
			}
		})
	}
}

// TestFlatSaveSameURLOverwrites confirms the collision check does NOT block
// the normal incremental-update case where the same URL is saved repeatedly.
func TestFlatSaveSameURLOverwrites(t *testing.T) {
	f := newTestFlat(t)
	now := time.Now().UTC().Truncate(time.Second)

	first := []models.Scene{{ID: "1", SiteID: "x", StudioURL: flatTestURL, Title: "v1", ScrapedAt: now}}
	if err := f.Save(flatTestURL, first); err != nil {
		t.Fatalf("first Save: %v", err)
	}
	second := []models.Scene{{ID: "1", SiteID: "x", StudioURL: flatTestURL, Title: "v2", ScrapedAt: now}}
	if err := f.Save(flatTestURL, second); err != nil {
		t.Fatalf("second Save: %v", err)
	}
	got, err := f.Load(flatTestURL)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got) != 1 || got[0].Title != "v2" {
		t.Errorf("expected v2 after overwrite, got %+v", got)
	}
}
