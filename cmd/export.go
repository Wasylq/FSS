package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/Anastylosis/FSS/internal/store"
	"github.com/Anastylosis/FSS/output"
)

var exportCmd = &cobra.Command{
	Use:   "export [studio-url ...]",
	Short: "Write studio JSON/CSV files out of the SQLite database",
	Long: `Export studios from the SQLite store as JSON and/or CSV files, one per studio,
named the same way the flat store names them.

With no arguments every tracked studio is exported.`,
	RunE: runExport,
}

func init() {
	rootCmd.AddCommand(exportCmd)
	exportCmd.Flags().String("db", "", "path to SQLite database (no value = the configured db, or the default location)")
	exportCmd.Flags().Lookup("db").NoOptDefVal = "default"
	exportCmd.Flags().StringP("output", "o", "", "export formats: json, csv, or json,csv (default from config)")
	exportCmd.Flags().String("out-dir", "", "output directory (default from config)")
}

func runExport(cmd *cobra.Command, args []string) error {
	dbPath := resolveDBPath(cmd)
	if dbPath == "" {
		return fmt.Errorf("--db is required (pass --db for the default location, or --db /path/to/file.db)")
	}

	outputFlag, _ := cmd.Flags().GetString("output")
	outputStr := outputFlag
	if outputStr == "" && cfg != nil {
		outputStr = cfg.Output
	}
	formats, err := parseFormats(outputStr)
	if err != nil {
		return err
	}
	if len(formats) == 0 {
		formats = []string{"json"}
	}

	outDir, _ := cmd.Flags().GetString("out-dir")
	if outDir == "" && cfg != nil {
		outDir = cfg.OutDir
	}
	if outDir == "" {
		outDir = "."
	}

	db, err := store.NewSQLite(dbPath)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer func() { _ = db.Close() }()

	urls := args
	if len(urls) == 0 {
		studios, err := db.ListStudios()
		if err != nil {
			return err
		}
		for _, s := range studios {
			urls = append(urls, s.URL)
		}
		if len(urls) == 0 {
			return fmt.Errorf("no studios tracked in %s — pass a studio URL explicitly, or scrape one with --db first", dbPath)
		}
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("creating output dir: %w", err)
	}

	written := 0
	var firstErr error
	for _, studioURL := range urls {
		slug := output.Slugify(studioURL)
		for _, format := range formats {
			path := filepath.Join(outDir, slug+"."+format)
			if err := db.Export(format, path, studioURL); err != nil {
				fmt.Fprintf(os.Stderr, "error exporting %s as %s: %v\n", studioURL, format, err)
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			fmt.Printf("  %s\n", path)
			written++
		}
	}
	fmt.Printf("Done: %d file(s) written to %s\n", written, outDir)
	return firstErr
}
