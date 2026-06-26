package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/output"
)

var slugHashSuffixRe = regexp.MustCompile(`^[0-9a-f]{8}$`)

// assertSlugBase checks Slugify(input) == wantBase + "-" + <8 hex hash>.
// Slugify appends a short hash of the raw URL so distinct URLs never collide.
func assertSlugBase(t *testing.T, input, wantBase string) {
	t.Helper()
	got := Slugify(input)
	prefix := wantBase + "-"
	if !strings.HasPrefix(got, prefix) {
		t.Errorf("Slugify(%q) = %q, want prefix %q", input, got, prefix)
		return
	}
	if suf := got[len(prefix):]; !slugHashSuffixRe.MatchString(suf) {
		t.Errorf("Slugify(%q) = %q, suffix %q is not an 8-hex hash", input, got, suf)
	}
}

func TestSlugify(t *testing.T) {
	assertSlugBase(t, "https://www.manyvids.com/Profile/590705/bettie-bondage/Store/Videos", "www-manyvids-com-profile-590705-bettie-bondage-store-videos")
	assertSlugBase(t, "https://www.kink.com/channel/hogtied", "www-kink-com-channel-hogtied")
	assertSlugBase(t, "https://tour.pissinghd.com/videos", "tour-pissinghd-com-videos")
	assertSlugBase(t, "https://www.karupsow.com/videos/", "www-karupsow-com-videos")
	assertSlugBase(t, "https://example.com", "example-com")
	assertSlugBase(t, "https://www.example.com/path/to/thing", "www-example-com-path-to-thing")
	assertSlugBase(t, "not-a-url", "not-a-url")
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
			assertSlugBase(t, tt.input, tt.want)
		})
	}
}

func TestSlugifyEmpty(t *testing.T) {
	// Empty input has no base, so the slug is the bare 8-hex hash — non-empty.
	got := Slugify("")
	if !slugHashSuffixRe.MatchString(got) {
		t.Errorf("Slugify('') = %q, want bare 8-hex hash", got)
	}
}

func TestSlugifyPathTraversal(t *testing.T) {
	got := Slugify("https://evil.com/../../etc/passwd")
	assertSlugBase(t, "https://evil.com/../../etc/passwd", "evil-com-etc-passwd")
	if strings.ContainsAny(got, "./\\") {
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

func TestFlatLock(t *testing.T) {
	f := newTestFlat(t)

	unlock, err := f.Lock(flatTestURL)
	if err != nil {
		t.Fatalf("Lock: %v", err)
	}
	if err := unlock.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Re-acquire after release.
	unlock2, err := f.Lock(flatTestURL)
	if err != nil {
		t.Fatalf("Lock after release: %v", err)
	}
	_ = unlock2.Close()
}

func TestFlatLockCreatesDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "deep")
	f := NewFlat(dir, []string{"json"})

	unlock, err := f.Lock(flatTestURL)
	if err != nil {
		t.Fatalf("Lock: %v", err)
	}
	_ = unlock.Close()

	if _, err := os.Stat(dir); err != nil {
		t.Errorf("Lock should create dir: %v", err)
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
// TestFlatSlugCollision is the A4 regression: URL pairs that share an identical
// sanitized base used to collide on one file (and the collision guard rejected
// the second). The hash suffix now gives each a distinct slug, so both studios
// save and load independently with no error and no clobbering.
func TestFlatSlugCollision(t *testing.T) {
	pairs := [][2]string{
		{"https://example.com/foo-bar", "https://example.com/foo/bar"},             // hyphen vs slash
		{"https://example.com/Foo", "https://example.com/foo"},                     // case difference
		{"https://example.com/videos?page=1", "https://example.com/videos?page=2"}, // query ignored by base
	}

	for _, pair := range pairs {
		urlA, urlB := pair[0], pair[1]
		t.Run(urlA+"_vs_"+urlB, func(t *testing.T) {
			// Same sanitized base, but distinct slugs thanks to the hash.
			if output.LegacySlugify(urlA) != output.LegacySlugify(urlB) {
				t.Fatalf("test fixture no longer shares a base: %q vs %q", urlA, urlB)
			}
			if Slugify(urlA) == Slugify(urlB) {
				t.Fatalf("hashed slugs still collide: %q vs %q", urlA, urlB)
			}

			f := newTestFlat(t)
			now := time.Now().UTC().Truncate(time.Second)

			scenesA := []models.Scene{{ID: "A", SiteID: "a", StudioURL: urlA, Title: "from A", ScrapedAt: now}}
			if err := f.Save(urlA, scenesA); err != nil {
				t.Fatalf("Save(urlA): %v", err)
			}
			scenesB := []models.Scene{{ID: "B", SiteID: "b", StudioURL: urlB, Title: "from B", ScrapedAt: now}}
			if err := f.Save(urlB, scenesB); err != nil {
				t.Fatalf("Save(urlB) should succeed now, got: %v", err)
			}

			gotA, err := f.Load(urlA)
			if err != nil {
				t.Fatalf("Load(urlA): %v", err)
			}
			if len(gotA) != 1 || gotA[0].ID != "A" {
				t.Errorf("urlA data wrong: got %+v", gotA)
			}
			gotB, err := f.Load(urlB)
			if err != nil {
				t.Fatalf("Load(urlB): %v", err)
			}
			if len(gotB) != 1 || gotB[0].ID != "B" {
				t.Errorf("urlB data wrong: got %+v", gotB)
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

// TestFlatMigratesLegacyFile covers the A4 backward-compat path: a file written
// under the pre-hash (legacy) slug is renamed to the new hashed name on Load,
// so existing incremental state survives the Slugify change.
func TestFlatMigratesLegacyFile(t *testing.T) {
	dir := t.TempDir()
	f := NewFlat(dir, []string{"json"})
	url := "https://www.example.com/videos"
	now := time.Now().UTC().Truncate(time.Second)

	// Write a file under the legacy (un-hashed) name directly.
	legacyPath := filepath.Join(dir, output.LegacySlugify(url)+".json")
	sf := models.StudioFile{StudioURL: url, ScrapedAt: now, SceneCount: 1,
		Scenes: []models.Scene{{ID: "1", SiteID: "x", StudioURL: url, Title: "legacy", ScrapedAt: now}}}
	data, _ := json.Marshal(sf)
	if err := os.WriteFile(legacyPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := f.Load(url)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got) != 1 || got[0].Title != "legacy" {
		t.Fatalf("legacy data not loaded: %+v", got)
	}
	// The legacy file is gone and the hashed file now exists.
	if _, err := os.Stat(legacyPath); !os.IsNotExist(err) {
		t.Errorf("legacy file should have been renamed away")
	}
	newPath := filepath.Join(dir, Slugify(url)+".json")
	if _, err := os.Stat(newPath); err != nil {
		t.Errorf("hashed file missing after migration: %v", err)
	}
}
