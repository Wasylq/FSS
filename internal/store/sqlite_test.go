package store

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Wasylq/FSS/models"
	_ "modernc.org/sqlite"
)

// ---- studios ----

func TestSQLiteUpsertListStudios(t *testing.T) {
	s := newTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)
	last := now.Add(time.Hour)

	st := models.Studio{
		URL:           testStudioURL,
		SiteID:        "manyvids",
		Name:          "Bettie Bondage",
		AddedAt:       now,
		LastScrapedAt: &last,
	}
	if err := s.UpsertStudio(st); err != nil {
		t.Fatalf("UpsertStudio: %v", err)
	}

	studios, err := s.ListStudios()
	if err != nil {
		t.Fatalf("ListStudios: %v", err)
	}
	if len(studios) != 1 {
		t.Fatalf("got %d studios, want 1", len(studios))
	}
	got := studios[0]
	if got.URL != testStudioURL {
		t.Errorf("URL = %q", got.URL)
	}
	if got.Name != "Bettie Bondage" {
		t.Errorf("Name = %q", got.Name)
	}
	if got.SiteID != "manyvids" {
		t.Errorf("SiteID = %q", got.SiteID)
	}
	if got.LastScrapedAt == nil || !got.LastScrapedAt.Equal(last) {
		t.Errorf("LastScrapedAt = %v, want %v", got.LastScrapedAt, last)
	}
}

func TestSQLiteUpsertStudioPreservesName(t *testing.T) {
	s := newTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)

	// First insert: with a name.
	if err := s.UpsertStudio(models.Studio{
		URL: testStudioURL, SiteID: "manyvids", Name: "Bettie Bondage", AddedAt: now,
	}); err != nil {
		t.Fatal(err)
	}

	// Second upsert: no name (e.g. scrape without --name flag) — should not clear the existing name.
	later := now.Add(time.Hour)
	if err := s.UpsertStudio(models.Studio{
		URL: testStudioURL, SiteID: "manyvids", Name: "", AddedAt: now, LastScrapedAt: &later,
	}); err != nil {
		t.Fatal(err)
	}

	studios, _ := s.ListStudios()
	if studios[0].Name != "Bettie Bondage" {
		t.Errorf("Name cleared by upsert without name, got %q", studios[0].Name)
	}
	if studios[0].LastScrapedAt == nil || !studios[0].LastScrapedAt.Equal(later) {
		t.Error("LastScrapedAt not updated")
	}
}

func TestSQLiteUpsertStudioUpdatesName(t *testing.T) {
	s := newTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)

	if err := s.UpsertStudio(models.Studio{
		URL: testStudioURL, SiteID: "manyvids", Name: "Old Name", AddedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	// Passing a new non-empty name should overwrite.
	if err := s.UpsertStudio(models.Studio{
		URL: testStudioURL, SiteID: "manyvids", Name: "New Name", AddedAt: now,
	}); err != nil {
		t.Fatal(err)
	}

	studios, _ := s.ListStudios()
	if studios[0].Name != "New Name" {
		t.Errorf("Name not updated, got %q", studios[0].Name)
	}
}

const testStudioURL = "https://www.manyvids.com/Profile/123/test-creator/Store/Videos"

func newTestDB(t *testing.T) *SQLite {
	t.Helper()
	s, err := NewSQLite(":memory:")
	if err != nil {
		t.Fatalf("NewSQLite: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestSQLiteSaveLoad(t *testing.T) {
	s := newTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)

	scenes := []models.Scene{
		{
			ID:          "1",
			SiteID:      "manyvids",
			StudioURL:   testStudioURL,
			Title:       "Scene One",
			URL:         "https://www.manyvids.com/Video/1/scene-one",
			Date:        now.Add(-48 * time.Hour),
			Description: "A description",
			Thumbnail:   "https://cdn.example.com/thumb1.jpg",
			Preview:     "https://cdn.example.com/preview1.mp4",
			Performers:  []string{"Alice"},
			Studio:      "Alice Studio",
			Tags:        []string{"Tag1", "Tag2"},
			Categories:  []string{"Cat1"},
			Duration:    1200,
			Resolution:  "4K",
			Width:       3840,
			Height:      2160,
			Format:      "MP4",
			Views:       500,
			Likes:       42,
			Comments:    7,
			ScrapedAt:   now,
		},
		{
			ID:         "2",
			SiteID:     "manyvids",
			StudioURL:  testStudioURL,
			Title:      "Scene Two",
			URL:        "https://www.manyvids.com/Video/2/scene-two",
			Date:       now.Add(-24 * time.Hour),
			Performers: []string{"Alice"},
			ScrapedAt:  now,
		},
	}
	scenes[0].AddPrice(models.PriceSnapshot{
		Date: now, Regular: 29.99, Discounted: 14.99, IsOnSale: true, DiscountPercent: 50,
	})

	if err := s.Save(testStudioURL, scenes); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := s.Load(testStudioURL)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("Load returned %d scenes, want 2", len(loaded))
	}

	byID := map[string]models.Scene{}
	for _, sc := range loaded {
		byID[sc.ID] = sc
	}

	sc1 := byID["1"]
	check := func(field, got, want string) {
		t.Helper()
		if got != want {
			t.Errorf("%s = %q, want %q", field, got, want)
		}
	}
	check("Title", sc1.Title, "Scene One")
	check("Description", sc1.Description, "A description")
	check("Thumbnail", sc1.Thumbnail, "https://cdn.example.com/thumb1.jpg")
	check("Resolution", sc1.Resolution, "4K")
	check("Format", sc1.Format, "MP4")
	if sc1.Duration != 1200 {
		t.Errorf("Duration = %d, want 1200", sc1.Duration)
	}
	if sc1.Width != 3840 || sc1.Height != 2160 {
		t.Errorf("Width/Height = %d/%d", sc1.Width, sc1.Height)
	}
	if len(sc1.Tags) != 2 || sc1.Tags[0] != "Tag1" || sc1.Tags[1] != "Tag2" {
		t.Errorf("Tags = %v", sc1.Tags)
	}
	if len(sc1.Performers) != 1 || sc1.Performers[0] != "Alice" {
		t.Errorf("Performers = %v", sc1.Performers)
	}
	if len(sc1.Categories) != 1 || sc1.Categories[0] != "Cat1" {
		t.Errorf("Categories = %v", sc1.Categories)
	}
	if sc1.DeletedAt != nil {
		t.Error("DeletedAt should be nil")
	}

	// Price history
	if len(sc1.PriceHistory) != 1 {
		t.Fatalf("PriceHistory len = %d, want 1", len(sc1.PriceHistory))
	}
	p := sc1.PriceHistory[0]
	if p.Regular != 29.99 {
		t.Errorf("Regular = %v, want 29.99", p.Regular)
	}
	if p.Discounted != 14.99 {
		t.Errorf("Discounted = %v, want 14.99", p.Discounted)
	}
	if !p.IsOnSale {
		t.Error("IsOnSale should be true")
	}
	if p.DiscountPercent != 50 {
		t.Errorf("DiscountPercent = %d, want 50", p.DiscountPercent)
	}
	if sc1.LowestPrice != 14.99 {
		t.Errorf("LowestPrice = %v, want 14.99", sc1.LowestPrice)
	}

	// Scene 2 should have no price history
	if len(byID["2"].PriceHistory) != 0 {
		t.Errorf("scene 2 PriceHistory should be empty")
	}
}

func TestSQLiteSaveIdempotent(t *testing.T) {
	s := newTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)

	scene := models.Scene{
		ID: "1", SiteID: "manyvids", StudioURL: testStudioURL,
		Title: "Original", ScrapedAt: now,
	}
	if err := s.Save(testStudioURL, []models.Scene{scene}); err != nil {
		t.Fatal(err)
	}

	// Save again with updated title — should replace, not duplicate.
	scene.Title = "Updated"
	if err := s.Save(testStudioURL, []models.Scene{scene}); err != nil {
		t.Fatal(err)
	}

	loaded, err := s.Load(testStudioURL)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 scene after idempotent save, got %d", len(loaded))
	}
	if loaded[0].Title != "Updated" {
		t.Errorf("Title = %q, want Updated", loaded[0].Title)
	}
}

func TestSQLitePriceHistoryAccumulates(t *testing.T) {
	s := newTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)

	scene := models.Scene{
		ID: "1", SiteID: "manyvids", StudioURL: testStudioURL, Title: "X", ScrapedAt: now,
	}
	scene.AddPrice(models.PriceSnapshot{Date: now, Regular: 29.99})

	if err := s.Save(testStudioURL, []models.Scene{scene}); err != nil {
		t.Fatal(err)
	}

	// Second scrape: load, add new price, save.
	loaded, err := s.Load(testStudioURL)
	if err != nil {
		t.Fatal(err)
	}
	loaded[0].AddPrice(models.PriceSnapshot{Date: now.Add(24 * time.Hour), Regular: 24.99, IsOnSale: true, Discounted: 24.99})
	if err := s.Save(testStudioURL, loaded); err != nil {
		t.Fatal(err)
	}

	final, err := s.Load(testStudioURL)
	if err != nil {
		t.Fatal(err)
	}
	if len(final[0].PriceHistory) != 2 {
		t.Fatalf("PriceHistory len = %d, want 2", len(final[0].PriceHistory))
	}
	if final[0].LowestPrice != 24.99 {
		t.Errorf("LowestPrice = %v, want 24.99", final[0].LowestPrice)
	}
}

func TestSQLiteMarkDeleted(t *testing.T) {
	s := newTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)

	scenes := []models.Scene{
		{ID: "1", SiteID: "manyvids", StudioURL: testStudioURL, Title: "A", ScrapedAt: now},
		{ID: "2", SiteID: "manyvids", StudioURL: testStudioURL, Title: "B", ScrapedAt: now},
	}
	if err := s.Save(testStudioURL, scenes); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkDeleted(testStudioURL, "manyvids", []string{"1"}); err != nil {
		t.Fatal(err)
	}

	loaded, err := s.Load(testStudioURL)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]models.Scene{}
	for _, sc := range loaded {
		byID[sc.ID] = sc
	}
	if byID["1"].DeletedAt == nil {
		t.Error("scene 1 DeletedAt should be set")
	}
	if byID["2"].DeletedAt != nil {
		t.Error("scene 2 DeletedAt should be nil")
	}

	// Mark deleted is idempotent — calling again should not change DeletedAt.
	firstDeletedAt := *byID["1"].DeletedAt
	if err := s.MarkDeleted(testStudioURL, "manyvids", []string{"1"}); err != nil {
		t.Fatal(err)
	}
	loaded2, _ := s.Load(testStudioURL)
	for _, sc := range loaded2 {
		if sc.ID == "1" && !sc.DeletedAt.Equal(firstDeletedAt) {
			t.Error("MarkDeleted should not update DeletedAt if already set")
		}
	}
}

// TestSQLiteRelationDiffAddRemove covers the syncRelation diff path: re-saving
// a scene with a different relation set should add new entries and drop removed
// ones, without re-touching unchanged rows.
func TestSQLiteRelationDiffAddRemove(t *testing.T) {
	s := newTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)

	scene := models.Scene{
		ID: "1", SiteID: "manyvids", StudioURL: testStudioURL, Title: "X", ScrapedAt: now,
		Performers: []string{"Alice", "Bob"},
		Tags:       []string{"red", "green"},
	}
	if err := s.Save(testStudioURL, []models.Scene{scene}); err != nil {
		t.Fatal(err)
	}

	// Drop Bob, add Carol; drop "green", add "blue".
	scene.Performers = []string{"Alice", "Carol"}
	scene.Tags = []string{"red", "blue"}
	if err := s.Save(testStudioURL, []models.Scene{scene}); err != nil {
		t.Fatal(err)
	}

	loaded, err := s.Load(testStudioURL)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 {
		t.Fatalf("got %d scenes, want 1", len(loaded))
	}
	got := loaded[0]
	wantPerformers := []string{"Alice", "Carol"}
	if len(got.Performers) != 2 || got.Performers[0] != wantPerformers[0] || got.Performers[1] != wantPerformers[1] {
		t.Errorf("Performers = %v, want %v", got.Performers, wantPerformers)
	}
	// Tags have no deterministic order in the schema; check as a set.
	gotTags := map[string]bool{}
	for _, t := range got.Tags {
		gotTags[t] = true
	}
	if !gotTags["red"] || !gotTags["blue"] || gotTags["green"] || len(gotTags) != 2 {
		t.Errorf("Tags = %v, want {red, blue}", got.Tags)
	}
}

// TestSQLiteRelationDiffPositionUpdate covers the positioned-relation case:
// reordering performers should update positions in place, not duplicate rows.
func TestSQLiteRelationDiffPositionUpdate(t *testing.T) {
	s := newTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)

	scene := models.Scene{
		ID: "1", SiteID: "manyvids", StudioURL: testStudioURL, Title: "X", ScrapedAt: now,
		Performers: []string{"Alice", "Bob"},
	}
	if err := s.Save(testStudioURL, []models.Scene{scene}); err != nil {
		t.Fatal(err)
	}

	scene.Performers = []string{"Bob", "Alice"}
	if err := s.Save(testStudioURL, []models.Scene{scene}); err != nil {
		t.Fatal(err)
	}

	loaded, _ := s.Load(testStudioURL)
	got := loaded[0].Performers
	if len(got) != 2 || got[0] != "Bob" || got[1] != "Alice" {
		t.Errorf("Performers after reorder = %v, want [Bob, Alice]", got)
	}

	var rowCount int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM scene_performers WHERE scene_id = '1'`).Scan(&rowCount); err != nil {
		t.Fatal(err)
	}
	if rowCount != 2 {
		t.Errorf("scene_performers row count = %d, want 2", rowCount)
	}
}

// TestSQLitePriceHistoryDiff verifies that re-saving with the same history is
// a no-op (no duplicate inserts) and that adding one snapshot inserts only
// the new row.
func TestSQLitePriceHistoryDiff(t *testing.T) {
	s := newTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)

	scene := models.Scene{
		ID: "1", SiteID: "manyvids", StudioURL: testStudioURL, Title: "X", ScrapedAt: now,
	}
	scene.AddPrice(models.PriceSnapshot{Date: now, Regular: 29.99})
	if err := s.Save(testStudioURL, []models.Scene{scene}); err != nil {
		t.Fatal(err)
	}

	var firstID int64
	if err := s.db.QueryRow(`SELECT id FROM price_history WHERE scene_id = '1'`).Scan(&firstID); err != nil {
		t.Fatal(err)
	}

	// Re-save with no change: row id must be preserved (no DELETE+reinsert churn).
	if err := s.Save(testStudioURL, []models.Scene{scene}); err != nil {
		t.Fatal(err)
	}
	var afterResaveID int64
	if err := s.db.QueryRow(`SELECT id FROM price_history WHERE scene_id = '1'`).Scan(&afterResaveID); err != nil {
		t.Fatal(err)
	}
	if afterResaveID != firstID {
		t.Errorf("re-save churned row id: %d -> %d (should be unchanged)", firstID, afterResaveID)
	}

	// Add a new snapshot. Original row id must still be preserved.
	scene.AddPrice(models.PriceSnapshot{Date: now.Add(24 * time.Hour), Regular: 24.99, IsOnSale: true, Discounted: 24.99})
	if err := s.Save(testStudioURL, []models.Scene{scene}); err != nil {
		t.Fatal(err)
	}
	var rowCount int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM price_history WHERE scene_id = '1'`).Scan(&rowCount); err != nil {
		t.Fatal(err)
	}
	if rowCount != 2 {
		t.Fatalf("price_history row count after add = %d, want 2", rowCount)
	}
	var stillFirst int64
	if err := s.db.QueryRow(`SELECT id FROM price_history WHERE scene_id = '1' AND regular = 29.99`).Scan(&stillFirst); err != nil {
		t.Fatal(err)
	}
	if stillFirst != firstID {
		t.Errorf("original snapshot row id changed: %d -> %d (diff path should not delete unchanged rows)", firstID, stillFirst)
	}
}

// TestSQLiteRelationFastPathPreservesEntities verifies the no-op case: re-saving
// with identical relations should not churn the entity table or junction rows.
func TestSQLiteRelationFastPathPreservesEntities(t *testing.T) {
	s := newTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)

	scene := models.Scene{
		ID: "1", SiteID: "manyvids", StudioURL: testStudioURL, Title: "X", ScrapedAt: now,
		Performers: []string{"Alice"},
	}
	if err := s.Save(testStudioURL, []models.Scene{scene}); err != nil {
		t.Fatal(err)
	}
	var firstAliceID int64
	if err := s.db.QueryRow(`SELECT id FROM performers WHERE name = 'Alice'`).Scan(&firstAliceID); err != nil {
		t.Fatal(err)
	}

	// Re-save unchanged: Alice's id and her junction row must be untouched.
	if err := s.Save(testStudioURL, []models.Scene{scene}); err != nil {
		t.Fatal(err)
	}
	var afterAliceID int64
	if err := s.db.QueryRow(`SELECT id FROM performers WHERE name = 'Alice'`).Scan(&afterAliceID); err != nil {
		t.Fatal(err)
	}
	if afterAliceID != firstAliceID {
		t.Errorf("Alice id churned: %d -> %d", firstAliceID, afterAliceID)
	}
	var perfCount int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM scene_performers WHERE scene_id = '1'`).Scan(&perfCount); err != nil {
		t.Fatal(err)
	}
	if perfCount != 1 {
		t.Errorf("scene_performers row count after no-op resave = %d, want 1", perfCount)
	}
}

// ---- Export ----

func TestSQLiteExportJSON(t *testing.T) {
	s := newTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)

	scenes := []models.Scene{{
		ID: "1", SiteID: "test", StudioURL: testStudioURL,
		Title: "Export Me", URL: "https://example.com/1",
		Performers: []string{"Alice"}, Tags: []string{"tag1"},
		Duration: 600, ScrapedAt: now,
	}}
	if err := s.Save(testStudioURL, scenes); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(t.TempDir(), "export.json")
	if err := s.Export("json", path, testStudioURL); err != nil {
		t.Fatalf("Export JSON: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var sf studioFile
	if err := json.Unmarshal(data, &sf); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if sf.StudioURL != testStudioURL {
		t.Errorf("studioUrl = %q", sf.StudioURL)
	}
	if len(sf.Scenes) != 1 || sf.Scenes[0].Title != "Export Me" {
		t.Errorf("scenes = %v", sf.Scenes)
	}
	if len(sf.Scenes[0].Performers) != 1 || sf.Scenes[0].Performers[0] != "Alice" {
		t.Errorf("performers = %v", sf.Scenes[0].Performers)
	}
}

func TestSQLiteExportCSV(t *testing.T) {
	s := newTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)

	scenes := []models.Scene{{
		ID: "1", SiteID: "test", StudioURL: testStudioURL,
		Title: "CSV Scene", Performers: []string{"Bob", "Carol"},
		Tags: []string{"t1", "t2"}, Duration: 1200, ScrapedAt: now,
	}}
	if err := s.Save(testStudioURL, scenes); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(t.TempDir(), "export.csv")
	if err := s.Export("csv", path, testStudioURL); err != nil {
		t.Fatalf("Export CSV: %v", err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	r := csv.NewReader(f)
	records, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 {
		t.Fatalf("got %d rows, want 2 (header + 1)", len(records))
	}
	if records[1][3] != "CSV Scene" {
		t.Errorf("title = %q", records[1][3])
	}
}

func TestSQLiteExportUnknownFormat(t *testing.T) {
	s := newTestDB(t)
	err := s.Export("xml", "/tmp/nope.xml", testStudioURL)
	if err == nil {
		t.Error("expected error for unknown format")
	}
}

// ---- Migration ----

// newV0DB creates a SQLite database at schema v0 (no junction tables).
// Scenes have JSON arrays in the performers/tags/categories TEXT columns,
// just like the original schema before migration 1.
func newV0DB(t *testing.T) *SQLite {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })

	if _, err := db.Exec(baseSchema); err != nil {
		t.Fatalf("baseSchema: %v", err)
	}
	// Explicitly set version to 0 so migration 1 will run.
	if _, err := db.Exec(`INSERT INTO schema_version (version) VALUES (0)`); err != nil {
		t.Fatal(err)
	}
	return &SQLite{db: db}
}

func TestSQLiteMigration1(t *testing.T) {
	s := newV0DB(t)
	now := timeStr(time.Now().UTC().Truncate(time.Second))

	// Insert v0-style scenes with JSON arrays in text columns.
	_, err := s.db.Exec(`
		INSERT INTO scenes (id, site_id, studio_url, title, url, date,
			performers, tags, categories, scraped_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"s1", "test", testStudioURL, "Scene One", "https://example.com/1", now,
		`["Alice","Bob"]`, `["blowjob","anal"]`, `["premium"]`, now,
	)
	if err != nil {
		t.Fatalf("insert scene 1: %v", err)
	}
	_, err = s.db.Exec(`
		INSERT INTO scenes (id, site_id, studio_url, title, url, date,
			performers, tags, categories, scraped_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"s2", "test", testStudioURL, "Scene Two", "https://example.com/2", now,
		`[]`, `["solo"]`, `[]`, now,
	)
	if err != nil {
		t.Fatalf("insert scene 2: %v", err)
	}

	// Run migration 1.
	if err := s.applyMigration1(); err != nil {
		t.Fatalf("applyMigration1: %v", err)
	}

	// Verify schema version updated.
	var version int
	if err := s.db.QueryRow(`SELECT MAX(version) FROM schema_version`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != 1 {
		t.Errorf("schema version = %d, want 1", version)
	}

	// Verify junction table data via Load.
	scenes, err := s.Load(testStudioURL)
	if err != nil {
		t.Fatalf("Load after migration: %v", err)
	}
	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2", len(scenes))
	}

	byID := map[string]models.Scene{}
	for _, sc := range scenes {
		byID[sc.ID] = sc
	}

	sc1 := byID["s1"]
	if len(sc1.Performers) != 2 || sc1.Performers[0] != "Alice" || sc1.Performers[1] != "Bob" {
		t.Errorf("s1 performers = %v, want [Alice Bob]", sc1.Performers)
	}
	if len(sc1.Tags) != 2 {
		t.Errorf("s1 tags = %v, want 2 tags", sc1.Tags)
	}
	if len(sc1.Categories) != 1 || sc1.Categories[0] != "premium" {
		t.Errorf("s1 categories = %v, want [premium]", sc1.Categories)
	}

	sc2 := byID["s2"]
	if len(sc2.Performers) != 0 {
		t.Errorf("s2 performers = %v, want empty", sc2.Performers)
	}
	if len(sc2.Tags) != 1 || sc2.Tags[0] != "solo" {
		t.Errorf("s2 tags = %v, want [solo]", sc2.Tags)
	}
	if len(sc2.Categories) != 0 {
		t.Errorf("s2 categories = %v, want empty", sc2.Categories)
	}

	// Verify entity tables were populated.
	var perfCount int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM performers`).Scan(&perfCount); err != nil {
		t.Fatal(err)
	}
	if perfCount != 2 {
		t.Errorf("performers table has %d rows, want 2", perfCount)
	}
}

func TestSQLiteMigration1EmptyDB(t *testing.T) {
	s := newV0DB(t)

	if err := s.applyMigration1(); err != nil {
		t.Fatalf("applyMigration1 on empty DB: %v", err)
	}

	var version int
	if err := s.db.QueryRow(`SELECT MAX(version) FROM schema_version`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != 1 {
		t.Errorf("schema version = %d, want 1", version)
	}
}

func TestSQLiteMigration1NullJSON(t *testing.T) {
	s := newV0DB(t)
	now := timeStr(time.Now().UTC().Truncate(time.Second))

	// Insert a scene where JSON columns are empty strings or null.
	_, err := s.db.Exec(`
		INSERT INTO scenes (id, site_id, studio_url, title, url, date,
			performers, tags, categories, scraped_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"s1", "test", testStudioURL, "Null Scene", "https://example.com/1", now,
		`null`, ``, `[]`, now,
	)
	if err != nil {
		t.Fatal(err)
	}

	if err := s.applyMigration1(); err != nil {
		t.Fatalf("applyMigration1 with null JSON: %v", err)
	}

	scenes, err := s.Load(testStudioURL)
	if err != nil {
		t.Fatal(err)
	}
	if len(scenes) != 1 {
		t.Fatalf("got %d scenes, want 1", len(scenes))
	}
	if len(scenes[0].Performers) != 0 {
		t.Errorf("performers = %v, want empty", scenes[0].Performers)
	}
}

// ---- unmarshalStrings ----

func TestUnmarshalStrings(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  []string
	}{
		{"empty string", "", nil},
		{"empty array", "[]", nil},
		{"null", "null", nil},
		{"single", `["alice"]`, []string{"alice"}},
		{"multiple", `["alice","bob","carol"]`, []string{"alice", "bob", "carol"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := unmarshalStrings(c.input)
			if err != nil {
				t.Fatalf("unmarshalStrings(%q): %v", c.input, err)
			}
			if len(got) != len(c.want) {
				t.Fatalf("got %v, want %v", got, c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Errorf("got[%d] = %q, want %q", i, got[i], c.want[i])
				}
			}
		})
	}
}

func TestUnmarshalStringsInvalid(t *testing.T) {
	_, err := unmarshalStrings(`{not json}`)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}
