package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Anastylosis/FSS/internal/store"
	"github.com/Anastylosis/FSS/models"
)

const importURL = "https://example.com/studio/import"

func writeImportStudioFile(t *testing.T, dir, name string, sf models.StudioFile) string {
	t.Helper()
	data, err := json.Marshal(sf)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func importScene(id, title string, now time.Time) models.Scene {
	return models.Scene{
		ID: id, SiteID: "imp", StudioURL: importURL,
		Title: title, URL: "https://example.com/" + id,
		Tags: []string{"tag-" + id}, Performers: []string{"P" + id},
		ScrapedAt: now,
	}
}

func newImportDB(t *testing.T) *store.SQLite {
	t.Helper()
	db, err := store.NewSQLite(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestImportFileRoundTrip(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	dir := t.TempDir()
	path := writeImportStudioFile(t, dir, "studio.json", models.StudioFile{
		SchemaVersion: models.StoreSchemaVersion,
		StudioURL:     importURL,
		ScrapedAt:     now,
		SceneCount:    2,
		Scenes:        []models.Scene{importScene("1", "One", now), importScene("2", "Two", now)},
	})

	db := newImportDB(t)
	n, _, err := importFile(db, path, false, false)
	if err != nil {
		t.Fatalf("importFile: %v", err)
	}
	if n != 2 {
		t.Errorf("imported %d, want 2", n)
	}

	got, err := db.Load(importURL)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("loaded %d scenes, want 2", len(got))
	}
	for _, sc := range got {
		if len(sc.Tags) != 1 || len(sc.Performers) != 1 {
			t.Errorf("scene %q lost relations: %+v", sc.ID, sc)
		}
		if sc.FirstSeenAt.IsZero() {
			t.Errorf("scene %q has no firstSeenAt", sc.ID)
		}
	}

	// The studios row is derived from the file, since JSON does not carry one.
	studios, err := db.ListStudios()
	if err != nil {
		t.Fatal(err)
	}
	if len(studios) != 1 || studios[0].URL != importURL || studios[0].SiteID != "imp" {
		t.Errorf("studios = %+v", studios)
	}
}

// Default is a merge: scenes only in the database survive an import that does
// not mention them.
func TestImportMergesByDefault(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	db := newImportDB(t)
	if err := db.Save(importURL, []models.Scene{importScene("1", "One", now), importScene("9", "Nine", now)}); err != nil {
		t.Fatal(err)
	}

	path := writeImportStudioFile(t, t.TempDir(), "studio.json", models.StudioFile{
		StudioURL: importURL,
		Scenes:    []models.Scene{importScene("2", "Two", now)},
	})
	if _, _, err := importFile(db, path, false, false); err != nil {
		t.Fatal(err)
	}

	got, _ := db.Load(importURL)
	if len(got) != 3 {
		t.Fatalf("got %d scenes, want 3 (2 stored + 1 imported)", len(got))
	}
}

func TestImportReplaceIsAuthoritative(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	db := newImportDB(t)
	if err := db.Save(importURL, []models.Scene{importScene("1", "One", now), importScene("9", "Nine", now)}); err != nil {
		t.Fatal(err)
	}

	path := writeImportStudioFile(t, t.TempDir(), "studio.json", models.StudioFile{
		StudioURL: importURL,
		Scenes:    []models.Scene{importScene("1", "One", now)},
	})
	if _, _, err := importFile(db, path, true, false); err != nil {
		t.Fatal(err)
	}

	got, _ := db.Load(importURL)
	if len(got) != 1 || got[0].ID != "1" {
		t.Fatalf("got %+v, want only scene 1", got)
	}
}

// An import must not blank stored metadata the file happens to omit — the same
// guarantee a re-scrape gives.
func TestImportPreservesEnrichment(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	db := newImportDB(t)
	rich := importScene("1", "One", now)
	rich.Description = "detailed"
	if err := db.Save(importURL, []models.Scene{rich}); err != nil {
		t.Fatal(err)
	}

	bare := models.Scene{ID: "1", SiteID: "imp", StudioURL: importURL, Title: "One", ScrapedAt: now}
	path := writeImportStudioFile(t, t.TempDir(), "studio.json", models.StudioFile{
		StudioURL: importURL,
		Scenes:    []models.Scene{bare},
	})
	if _, _, err := importFile(db, path, true, false); err != nil {
		t.Fatal(err)
	}

	got, _ := db.Load(importURL)
	if len(got) != 1 || got[0].Description != "detailed" || len(got[0].Tags) != 1 {
		t.Errorf("enrichment lost: %+v", got)
	}
}

func TestImportDryRunWritesNothing(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	db := newImportDB(t)
	path := writeImportStudioFile(t, t.TempDir(), "studio.json", models.StudioFile{
		StudioURL: importURL,
		Scenes:    []models.Scene{importScene("1", "One", now)},
	})
	if _, _, err := importFile(db, path, false, true); err != nil {
		t.Fatal(err)
	}
	if got, _ := db.Load(importURL); len(got) != 0 {
		t.Errorf("dry run wrote %d scenes", len(got))
	}
}

func TestImportRejectsBadFiles(t *testing.T) {
	dir := t.TempDir()
	db := newImportDB(t)

	noURL := writeImportStudioFile(t, dir, "nourl.json", models.StudioFile{
		Scenes: []models.Scene{{ID: "1", SiteID: "imp", Title: "x"}},
	})
	if _, _, err := importFile(db, noURL, false, false); err == nil {
		t.Error("expected an error for a file with no studioUrl")
	}

	future := writeImportStudioFile(t, dir, "future.json", models.StudioFile{
		SchemaVersion: models.StoreSchemaVersion + 1,
		StudioURL:     importURL,
		Scenes:        []models.Scene{{ID: "1", SiteID: "imp", Title: "x"}},
	})
	if _, _, err := importFile(db, future, false, false); err == nil {
		t.Error("expected an error for a newer schema version")
	}

	bad := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(bad, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := importFile(db, bad, false, false); err == nil {
		t.Error("expected an error for malformed JSON")
	}
}

func TestCollectJSONFiles(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"a.json", "b.json", "notes.txt"} {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	// Directory expansion picks up only .json, sorted.
	got, err := collectJSONFiles([]string{dir})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || filepath.Base(got[0]) != "a.json" || filepath.Base(got[1]) != "b.json" {
		t.Fatalf("got %v", got)
	}

	// A file named twice, and a file also covered by its directory, appear once.
	explicit := filepath.Join(dir, "a.json")
	got, err = collectJSONFiles([]string{explicit, dir, explicit})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %v, want 2 deduplicated entries", got)
	}

	if _, err := collectJSONFiles([]string{filepath.Join(dir, "missing")}); err == nil {
		t.Error("expected an error for a nonexistent path")
	}
}

func TestStudioFromFile(t *testing.T) {
	early := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	late := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	s1 := importScene("1", "One", late)
	s1.FirstSeenAt = late
	s1.Studio = "Example Studio"
	s2 := importScene("2", "Two", late)
	s2.FirstSeenAt = early

	got := studioFromFile(models.StudioFile{StudioURL: importURL, ScrapedAt: late}, []models.Scene{s1, s2})
	if got.SiteID != "imp" || got.Name != "Example Studio" {
		t.Errorf("got %+v", got)
	}
	if !got.AddedAt.Equal(early) {
		t.Errorf("AddedAt = %v, want the earliest firstSeenAt %v", got.AddedAt, early)
	}
	if got.LastScrapedAt == nil || !got.LastScrapedAt.Equal(late) {
		t.Errorf("LastScrapedAt = %v, want %v", got.LastScrapedAt, late)
	}

	// With no firstSeenAt anywhere, AddedAt falls back to the file's scrapedAt.
	bare := studioFromFile(models.StudioFile{StudioURL: importURL, ScrapedAt: late},
		[]models.Scene{importScene("1", "One", late)})
	if !bare.AddedAt.Equal(late) {
		t.Errorf("AddedAt = %v, want fallback %v", bare.AddedAt, late)
	}
}

// TestCollectJSONFilesOrdersByModTime pins the ordering that decides which
// version of a re-scraped studio survives.
//
// Merging is last-write-wins per field, so processing order is data-affecting.
// Sorting by name made it depend on filenames, and a browser saving a second
// download as `x (1).json` sorts it *before* `x.json` — so the newer file was
// applied first and the older one overwrote it.
func TestCollectJSONFilesOrdersByModTime(t *testing.T) {
	dir := t.TempDir()
	older := filepath.Join(dir, "studio (1).json") // sorts FIRST by name
	newer := filepath.Join(dir, "studio.json")     // sorts second by name

	for _, p := range []string{older, newer} {
		if err := os.WriteFile(p, []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	base := time.Now().Add(-time.Hour)
	if err := os.Chtimes(older, base, base); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(newer, base.Add(time.Hour), base.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}

	got, err := collectJSONFiles([]string{dir})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d files, want 2", len(got))
	}
	if got[0] != older || got[1] != newer {
		t.Errorf("order = %v, want oldest first (%s then %s)",
			[]string{filepath.Base(got[0]), filepath.Base(got[1])},
			filepath.Base(older), filepath.Base(newer))
	}
}

// The newest file's values must survive when two files describe one studio.
func TestImportNewerFileWinsRegardlessOfName(t *testing.T) {
	dir := t.TempDir()
	old := time.Date(2026, 4, 28, 0, 0, 0, 0, time.UTC)
	recent := time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC)

	// The alphabetically-first file holds the OLDER data, as it did in practice.
	oldPath := writeImportStudioFile(t, dir, "studio (1).json", models.StudioFile{
		StudioURL: importURL,
		Scenes:    []models.Scene{{ID: "1", SiteID: "imp", Title: "April title", ScrapedAt: old}},
	})
	newPath := writeImportStudioFile(t, dir, "studio.json", models.StudioFile{
		StudioURL: importURL,
		Scenes:    []models.Scene{{ID: "1", SiteID: "imp", Title: "May title", ScrapedAt: recent}},
	})
	if err := os.Chtimes(oldPath, old, old); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(newPath, recent, recent); err != nil {
		t.Fatal(err)
	}

	db := newImportDB(t)
	files, err := collectJSONFiles([]string{dir})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if _, _, err := importFile(db, f, false, false); err != nil {
			t.Fatal(err)
		}
	}

	got, _ := db.Load(importURL)
	if len(got) != 1 {
		t.Fatalf("got %d scenes, want 1", len(got))
	}
	if got[0].Title != "May title" {
		t.Errorf("title = %q, want the newer file's value", got[0].Title)
	}
}

// A dry run must not create the database. It used to skip only the MkdirAll and
// still open the file, so --dry-run failed outright before a first import —
// exactly when it is most useful.
func TestImportDryRunDoesNotCreateDatabase(t *testing.T) {
	dir := t.TempDir()
	writeImportStudioFile(t, dir, "studio.json", models.StudioFile{
		StudioURL: importURL,
		Scenes:    []models.Scene{{ID: "1", SiteID: "imp", Title: "One", ScrapedAt: time.Now().UTC()}},
	})
	dbPath := filepath.Join(dir, "nested", "fss.db")

	// importFile tolerates a nil database: nothing is stored, so nothing merges.
	n, url, err := importFile(nil, filepath.Join(dir, "studio.json"), false, true)
	if err != nil {
		t.Fatalf("dry-run importFile with no database: %v", err)
	}
	if n != 1 || url != importURL {
		t.Errorf("got (%d, %q), want (1, %q)", n, url, importURL)
	}
	if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
		t.Errorf("dry run created %s", dbPath)
	}
	if _, err := os.Stat(filepath.Dir(dbPath)); !os.IsNotExist(err) {
		t.Error("dry run created the database directory")
	}
}
