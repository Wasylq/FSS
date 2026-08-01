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

	for _, line := range updateLines(buildVersion, latest) {
		fmt.Println(line)
	}
	return nil
}

// updateLines renders the update notice comparing the built-in version against
// the latest release tag.
//
// Split out from runVersion so the comparison is testable without reaching
// GitHub — the network call is best-effort and its own concern.
//
// An empty tag is treated as "could not determine": GitHub returning a body
// without tag_name (or any 200 that is not the expected shape) otherwise fell
// into the default branch and printed "Update available: v1.2.3 → " with nothing
// after the arrow, pointing the user at a release that was never identified.
func updateLines(current, latest string) []string {
	if strings.TrimSpace(latest) == "" {
		return []string{"Could not determine the latest release."}
	}

	cur := strings.TrimPrefix(current, "v")
	remote := strings.TrimPrefix(latest, "v")

	switch cur {
	case "dev":
		return []string{fmt.Sprintf("Latest release: %s (running dev build)", latest)}
	case remote:
		return []string{"You are running the latest version."}
	default:
		return []string{
			fmt.Sprintf("Update available: %s → %s", current, latest),
			"https://github.com/Wasylq/FSS/releases/latest",
		}
	}
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
