package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Wasylq/FSS/scraper"
)

var listScrapersCmd = &cobra.Command{
	Use:   "list-scrapers",
	Short: "List all registered scrapers and their URL patterns",
	RunE:  runListScrapers,
}

func init() {
	rootCmd.AddCommand(listScrapersCmd)
}

func runListScrapers(_ *cobra.Command, _ []string) error {
	all := scraper.All()
	if len(all) == 0 {
		fmt.Println("No scrapers registered.")
		return nil
	}
	for _, s := range all {
		fmt.Printf("%s:\n", s.ID())
		for _, p := range s.Patterns() {
			fmt.Printf("  %s\n", p)
		}
	}
	return nil
}
