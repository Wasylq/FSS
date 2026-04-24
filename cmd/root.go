package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/Wasylq/FSS/internal/config"
)

var cfg *config.Config

var rootCmd = &cobra.Command{
	Use:   "fss",
	Short: "FullStudioScraper — scrape all scenes and metadata from a studio URL",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		var err error
		cfg, err = config.Load()
		return err
	},
}

var buildVersion, buildCommit, buildDate string

// SetVersion is called from main with values injected by -ldflags at build time.
func SetVersion(version, commit, date string) {
	buildVersion = version
	buildCommit = commit
	buildDate = date
	rootCmd.Version = version + " (" + commit + ", " + date + ")"
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
