package cmd

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Wasylq/FSS/scraper"
)

var checkCmd = &cobra.Command{
	Use:   "check <url>",
	Short: "Check whether a URL is supported and show the matching scraper",
	Args:  cobra.ExactArgs(1),
	RunE:  runCheck,
}

func init() {
	rootCmd.AddCommand(checkCmd)
}

func newScraperIssueURL(siteURL string) string {
	q := url.Values{}
	q.Set("template", "new_scraper.yml")
	q.Set("url", siteURL)
	return "https://github.com/Wasylq/FSS/issues/new?" + q.Encode()
}

func runCheck(cmd *cobra.Command, args []string) error {
	rawURL := args[0]
	w := cmd.OutOrStdout()

	s, err := scraper.ForURL(rawURL)
	if err != nil {
		_, _ = fmt.Fprintf(w, "Not supported: %s\n", rawURL)
		_, _ = fmt.Fprintf(w, "\nRequest support: %s\n", newScraperIssueURL(rawURL))
		return nil
	}

	_, _ = fmt.Fprintf(w, "Scraper:  %s\n", s.ID())
	_, _ = fmt.Fprintf(w, "Patterns: %s\n", strings.Join(s.Patterns(), ", "))
	return nil
}
