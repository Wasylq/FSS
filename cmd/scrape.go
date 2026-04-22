package cmd

import (
	"github.com/spf13/cobra"
)

var scrapeCmd = &cobra.Command{
	Use:   "scrape <studio-url>",
	Short: "Scrape all scenes from a studio URL",
	Args:  cobra.ExactArgs(1),
	RunE:  runScrape,
}

func init() {
	rootCmd.AddCommand(scrapeCmd)

	scrapeCmd.Flags().IntP("workers", "w", 0, "max parallel fetchers (0 = use config/default)")
	scrapeCmd.Flags().Bool("full", false, "ignore existing data, scrape everything from scratch")
	scrapeCmd.Flags().Bool("refresh", false, "re-fetch metadata for all known scenes, soft-delete missing")
	scrapeCmd.Flags().StringP("output", "o", "", "export formats: json, csv, or json,csv (default from config)")
	scrapeCmd.Flags().String("out", "", "output directory (default from config)")
	scrapeCmd.Flags().String("db", "", "enable SQLite store at this path")
}

func runScrape(cmd *cobra.Command, args []string) error {
	// TODO: Phase 2+ — resolve flags against cfg, pick store, pick scraper, run.
	return nil
}
