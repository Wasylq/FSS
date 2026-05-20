package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/Wasylq/FSS/internal/config"
	"github.com/Wasylq/FSS/internal/httpx"
)

var cfg *config.Config

var rootCmd = &cobra.Command{
	Use:   "fss",
	Short: "FullStudioScraper — scrape all scenes and metadata from a studio URL",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		var err error
		cfg, err = config.Load()
		if err != nil {
			return err
		}
		httpx.SetDefaultUA(cfg.UserAgent)
		return nil
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
		os.Exit(1)
	}
}
