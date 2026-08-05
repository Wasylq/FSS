package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Wasylq/FSS/internal/config"
	"github.com/Wasylq/FSS/internal/store"
	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/scraper"
)

var importCmd = &cobra.Command{
	Use:   "import <file-or-dir> [file-or-dir ...]",
	Short: "Load studio JSON files into the SQLite database",
	Long: `Load one or more studio JSON files (or every *.json in a directory) into the
SQLite store.

The database schema is a superset of the JSON layout, so nothing is lost. Each
file's own "studioUrl" decides which studio it belongs to — filenames are not
parsed.

Existing database scenes are merged with the file by default: a scene present in
both takes the file's values, price history is carried forward, and scenes only
in the database are kept. Pass --replace to make the file authoritative instead,
which deletes database scenes absent from it.`,
	Args: cobra.MinimumNArgs(1),
	RunE: runImport,
}

func init() {
	rootCmd.AddCommand(importCmd)
	importCmd.Flags().String("db", "", "path to SQLite database (no value = default location)")
	importCmd.Flags().Lookup("db").NoOptDefVal = "default"
	importCmd.Flags().Bool("replace", false, "make each file authoritative: delete stored scenes it does not contain")
	importCmd.Flags().Bool("dry-run", false, "report what would be imported without writing")
}

func runImport(cmd *cobra.Command, args []string) error {
	dbFlag, _ := cmd.Flags().GetString("db")
	if dbFlag == "" && cfg != nil {
		dbFlag = cfg.DB
	}
	dbPath := config.ResolveDBPath(dbFlag)
	if dbPath == "" {
		return fmt.Errorf("--db is required (pass --db for the default location, or --db /path/to/file.db)")
	}
	replace, _ := cmd.Flags().GetBool("replace")
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	files, err := collectJSONFiles(args)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("no .json files found in %s", strings.Join(args, ", "))
	}

	if !dryRun {
		if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
			return fmt.Errorf("creating database directory: %w", err)
		}
	}
	db, err := store.NewSQLite(dbPath)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer func() { _ = db.Close() }()

	var imported, skipped int
	for _, path := range files {
		n, err := importFile(db, path, replace, dryRun)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: %s: %v\n", path, err)
			skipped++
			continue
		}
		imported += n
	}

	verb := "imported"
	if dryRun {
		verb = "would import"
	}
	fmt.Printf("Done: %s %d scene(s) from %d file(s)", verb, imported, len(files)-skipped)
	if skipped > 0 {
		fmt.Printf(", %d file(s) skipped", skipped)
	}
	fmt.Println()
	return nil
}

// collectJSONFiles expands each argument: a directory contributes its *.json
// entries (not recursive), a file contributes itself.
func collectJSONFiles(args []string) ([]string, error) {
	var out []string
	seen := map[string]bool{}
	for _, arg := range args {
		info, err := os.Stat(arg)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", arg, err)
		}
		if !info.IsDir() {
			if !seen[arg] {
				seen[arg] = true
				out = append(out, arg)
			}
			continue
		}
		matches, err := filepath.Glob(filepath.Join(arg, "*.json"))
		if err != nil {
			return nil, err
		}
		for _, m := range matches {
			if !seen[m] {
				seen[m] = true
				out = append(out, m)
			}
		}
	}
	sort.Strings(out)
	return out, nil
}

func importFile(db *store.SQLite, path string, replace, dryRun bool) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	var sf models.StudioFile
	if err := json.Unmarshal(data, &sf); err != nil {
		return 0, fmt.Errorf("parsing: %w", err)
	}
	if sf.SchemaVersion > models.StoreSchemaVersion {
		return 0, fmt.Errorf("store schema v%d is newer than this build's v%d — upgrade fss",
			sf.SchemaVersion, models.StoreSchemaVersion)
	}
	// The studio URL comes from the file, never the filename: Slugify is lossy
	// and its hash suffix is not reversible.
	if sf.StudioURL == "" {
		return 0, fmt.Errorf("no studioUrl in file — cannot tell which studio these scenes belong to")
	}
	if len(sf.Scenes) == 0 {
		return 0, nil
	}

	unlock, err := db.Lock(sf.StudioURL)
	if err != nil {
		return 0, fmt.Errorf("locking studio: %w", err)
	}
	defer func() { _ = unlock.Close() }()

	existing, err := db.Load(sf.StudioURL)
	if err != nil {
		return 0, fmt.Errorf("loading stored scenes: %w", err)
	}
	existingByKey := make(map[sceneKey]models.Scene, len(existing))
	for _, s := range existing {
		existingByKey[keyOf(s)] = s
	}

	incoming := make([]models.Scene, 0, len(sf.Scenes))
	seen := make(map[sceneKey]bool, len(sf.Scenes))
	for _, s := range sf.Scenes {
		s.StudioURL = sf.StudioURL
		k := keyOf(s)
		if seen[k] {
			continue
		}
		seen[k] = true
		if prev, ok := existingByKey[k]; ok {
			s = carryOver(s, prev, true)
		}
		incoming = append(incoming, s)
	}
	if !replace {
		for _, s := range existing {
			if !seen[keyOf(s)] {
				incoming = append(incoming, s)
			}
		}
	}

	scraper.Debugf(1, "import: %s → %s (%d in file, %d stored, %d after merge)",
		path, sf.StudioURL, len(sf.Scenes), len(existing), len(incoming))

	if dryRun {
		fmt.Printf("  %-50s  →  %s (%d scene(s))\n", truncate(filepath.Base(path), 50), sf.StudioURL, len(incoming))
		return len(incoming), nil
	}

	if err := db.Save(sf.StudioURL, incoming); err != nil {
		return 0, fmt.Errorf("saving: %w", err)
	}
	if err := db.UpsertStudio(studioFromFile(sf, incoming)); err != nil {
		fmt.Fprintf(os.Stderr, "warning: %s: could not update studio record: %v\n", path, err)
	}
	fmt.Printf("  %-50s  →  %s (%d scene(s))\n", truncate(filepath.Base(path), 50), sf.StudioURL, len(incoming))
	return len(incoming), nil
}

// studioFromFile derives the studios-table row the JSON does not carry: site ID
// and display name come from the scenes, AddedAt from the earliest first-seen
// timestamp available.
func studioFromFile(sf models.StudioFile, scenes []models.Scene) models.Studio {
	st := models.Studio{URL: sf.StudioURL}
	for _, s := range scenes {
		if st.SiteID == "" {
			st.SiteID = s.SiteID
		}
		if st.Name == "" {
			st.Name = s.Studio
		}
		if t := s.FirstSeenAt; !t.IsZero() && (st.AddedAt.IsZero() || t.Before(st.AddedAt)) {
			st.AddedAt = t
		}
	}
	if st.AddedAt.IsZero() {
		st.AddedAt = sf.ScrapedAt
	}
	if !sf.ScrapedAt.IsZero() {
		last := sf.ScrapedAt
		st.LastScrapedAt = &last
	}
	return st
}
