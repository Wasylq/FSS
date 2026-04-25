package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

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
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/repos/Wasylq/FSS/releases/latest", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}
	return release.TagName, nil
}
