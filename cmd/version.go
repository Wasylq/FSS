package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Wasylq/FSS/internal/httpx"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version and check for updates",
	Args:  cobra.NoArgs,
	RunE:  runVersion,
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

func runVersion(_ *cobra.Command, _ []string) error {
	fmt.Printf("fss %s (%s, %s)\n", buildVersion, buildCommit, buildDate)

	latest, err := fetchLatestRelease()
	if err != nil {
		fmt.Printf("Could not check for updates: %v\n", err)
		return nil
	}

	current := strings.TrimPrefix(buildVersion, "v")
	remote := strings.TrimPrefix(latest, "v")

	switch current {
	case "dev":
		fmt.Printf("Latest release: %s (running dev build)\n", latest)
	case remote:
		fmt.Println("You are running the latest version.")
	default:
		fmt.Printf("Update available: %s → %s\n", buildVersion, latest)
		fmt.Println("https://github.com/Wasylq/FSS/releases/latest")
	}

	return nil
}

func fetchLatestRelease() (string, error) {
	// Align ctx + client timeouts at 5s — the previous 1s ctx vs 5s
	// client mismatch meant ctx always tripped first, making the
	// transport-level deadline dead code.
	const timeout = 5 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	client := httpx.NewClient(timeout)
	// Single attempt — `version` is a best-effort check and shouldn't
	// stall the user with 0s/2s/4s retry backoff if GitHub is having
	// a bad day.
	resp, err := httpx.Do(ctx, client, httpx.Request{
		URL:         "https://api.github.com/repos/Wasylq/FSS/releases/latest",
		Headers:     map[string]string{"Accept": "application/vnd.github+json"},
		MaxAttempts: 1,
	})
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := httpx.DecodeJSON(resp.Body, &release); err != nil {
		return "", err
	}
	return release.TagName, nil
}
