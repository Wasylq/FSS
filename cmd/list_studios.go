package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Wasylq/FSS/internal/store"
)

var listStudiosCmd = &cobra.Command{
	Use:   "list-studios",
	Short: "List all studios tracked in the SQLite database",
	RunE:  runListStudios,
}

var listStudiosDB string

func init() {
	rootCmd.AddCommand(listStudiosCmd)
	listStudiosCmd.Flags().StringVar(&listStudiosDB, "db", "", "path to SQLite database (required)")
	// MarkFlagRequired only errors if the flag is unregistered — a programming
	// bug we want to surface at startup, not silently swallow.
	if err := listStudiosCmd.MarkFlagRequired("db"); err != nil {
		panic(err)
	}
}

func runListStudios(cmd *cobra.Command, _ []string) error {
	db, err := store.NewSQLite(listStudiosDB)
	if err != nil {
		return fmt.Errorf("opening db: %w", err)
	}
	defer func() { _ = db.Close() }()

	studios, err := db.ListStudios()
	if err != nil {
		return err
	}
	if len(studios) == 0 {
		fmt.Println("No studios tracked yet. Scrape a studio with --db to add it.")
		return nil
	}
	for _, s := range studios {
		name := s.Name
		if name == "" {
			name = "(no name)"
		}
		last := "never"
		if s.LastScrapedAt != nil {
			last = s.LastScrapedAt.Format("2006-01-02")
		}
		fmt.Printf("%-30s  %-12s  last scraped: %s\n  %s\n", name, s.SiteID, last, s.URL)
	}
	return nil
}
