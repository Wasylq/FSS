package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/Anastylosis/FSS/internal/store"
	"github.com/Anastylosis/FSS/models"
	"github.com/Anastylosis/FSS/output"
)

// exportCmdFor builds a standalone command carrying the same flags as the real
// one, so a test can set them without mutating the shared rootCmd.
func exportCmdFor(t *testing.T, args ...string) *cobra.Command {
	t.Helper()
	c := &cobra.Command{RunE: runExport}
	c.Flags().String("db", "", "")
	c.Flags().Lookup("db").NoOptDefVal = "default"
	c.Flags().StringP("output", "o", "", "")
	c.Flags().String("out-dir", "", "")
	c.SetOut(&strings.Builder{})
	if err := c.ParseFlags(args); err != nil {
		t.Fatalf("ParseFlags: %v", err)
	}
	return c
}

func seedDB(t *testing.T, studioURL string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fss.db")
	db, err := store.NewSQLite(path)
	if err != nil {
		t.Fatalf("NewSQLite: %v", err)
	}
	defer func() { _ = db.Close() }()

	now := time.Now().UTC().Truncate(time.Second)
	scenes := []models.Scene{{
		ID: "1", SiteID: "ex", StudioURL: studioURL, Title: "Exported Scene",
		URL: studioURL + "/v/1", Performers: []string{"Alice"},
		Tags: []string{"tag"}, Studio: "Example", ScrapedAt: now,
	}}
	if err := db.Save(studioURL, scenes); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := db.UpsertStudio(models.Studio{
		URL: studioURL, SiteID: "ex", Name: "Example", AddedAt: now,
	}); err != nil {
		t.Fatalf("UpsertStudio: %v", err)
	}
	return path
}

func TestExportWritesRequestedFormats(t *testing.T) {
	const studioURL = "https://export.example.com/studio"
	dbPath := seedDB(t, studioURL)
	outDir := t.TempDir()

	c := exportCmdFor(t, "--db="+dbPath, "--out-dir", outDir, "--output", "json,csv")
	if err := runExport(c, nil); err != nil {
		t.Fatalf("runExport: %v", err)
	}

	slug := output.Slugify(studioURL)
	for _, ext := range []string{"json", "csv"} {
		path := filepath.Join(outDir, slug+"."+ext)
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("expected %s: %v", path, err)
		}
		if info.Size() == 0 {
			t.Errorf("%s is empty", path)
		}
	}

	raw, err := os.ReadFile(filepath.Join(outDir, slug+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var file models.StudioFile
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatalf("exported JSON does not parse: %v", err)
	}
	if len(file.Scenes) != 1 || file.Scenes[0].Title != "Exported Scene" {
		t.Errorf("exported scenes = %+v", file.Scenes)
	}
}

// With no URL arguments the tracked studios are exported.
func TestExportDefaultsToEveryTrackedStudio(t *testing.T) {
	const studioURL = "https://export-all.example.com/studio"
	dbPath := seedDB(t, studioURL)
	outDir := t.TempDir()

	c := exportCmdFor(t, "--db="+dbPath, "--out-dir", outDir, "--output", "json")
	if err := runExport(c, nil); err != nil {
		t.Fatalf("runExport: %v", err)
	}

	if _, err := os.Stat(filepath.Join(outDir, output.Slugify(studioURL)+".json")); err != nil {
		t.Errorf("no file for the tracked studio: %v", err)
	}
}

func TestExportRequiresDB(t *testing.T) {
	c := exportCmdFor(t, "--out-dir", t.TempDir())
	err := runExport(c, nil)
	if err == nil {
		t.Fatal("expected an error without --db")
	}
	if !strings.Contains(err.Error(), "--db is required") {
		t.Errorf("err = %v", err)
	}
}

func TestExportEmptyDatabaseIsAnError(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "empty.db")
	db, err := store.NewSQLite(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	_ = db.Close()

	c := exportCmdFor(t, "--db="+dbPath, "--out-dir", t.TempDir())
	err = runExport(c, nil)
	if err == nil || !strings.Contains(err.Error(), "no studios tracked") {
		t.Errorf("err = %v, want the no-studios message", err)
	}
}
