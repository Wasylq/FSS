package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var stashCmd = &cobra.Command{
	Use:   "stash",
	Short: "Interact with a Stash instance",
}

func init() {
	rootCmd.AddCommand(stashCmd)

	stashCmd.PersistentFlags().String("url", "", "Stash server URL (default from config)")
	stashCmd.PersistentFlags().String("api-key", "", "Stash API key (env: FSS_STASH_API_KEY)")
}

func stashURL(cmd *cobra.Command) string {
	u, _ := cmd.Flags().GetString("url")
	if u != "" {
		return u
	}
	return cfg.Stash.URL
}

func stashAPIKey(cmd *cobra.Command) string {
	k, _ := cmd.Flags().GetString("api-key")
	if k != "" {
		return k
	}
	if env := os.Getenv("FSS_STASH_API_KEY"); env != "" {
		return env
	}
	return cfg.Stash.APIKey
}
