package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/Wasylq/FSS/internal/stash"
)

var stashUnmatchedCmd = &cobra.Command{
	Use:   "unmatched",
	Short: "List Stash scenes with no StashDB metadata",
	RunE:  runStashUnmatched,
}

func init() {
	stashCmd.AddCommand(stashUnmatchedCmd)

	stashUnmatchedCmd.Flags().String("performer", "", "filter by performer name")
	stashUnmatchedCmd.Flags().String("studio", "", "filter by studio name")
	stashUnmatchedCmd.Flags().Int("top", 10, "limit number of results (0 = all)")
}

func runStashUnmatched(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()

	client := stash.NewClient(stashURL(cmd), stashAPIKey(cmd))
	if err := client.Ping(ctx); err != nil {
		return fmt.Errorf("connecting to stash: %w", err)
	}
	fmt.Println("Connected to Stash")

	performer, _ := cmd.Flags().GetString("performer")
	studio, _ := cmd.Flags().GetString("studio")

	top, _ := cmd.Flags().GetInt("top")

	zero := 0
	filter := stash.FindScenesFilter{
		StashIDCount:  &zero,
		PerformerName: performer,
		StudioName:    studio,
	}

	fmt.Print("Querying unmatched scenes...")
	var scenes []stash.StashScene
	var total int
	var err error
	if top > 0 {
		scenes, total, err = client.FindScenes(ctx, filter, 1, top)
	} else {
		scenes, err = client.FindAllScenes(ctx, filter, func(fetched, count int) {
			fmt.Printf("\rQuerying unmatched scenes... %d / %d", fetched, count)
		})
		total = len(scenes)
	}
	fmt.Println()
	if err != nil {
		return fmt.Errorf("querying stash: %w", err)
	}

	if len(scenes) == 0 {
		fmt.Println("No unmatched scenes found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tFilename\tTitle\tPerformers")
	fmt.Fprintln(w, "--\t--------\t-----\t----------")
	for _, s := range scenes {
		filename := ""
		if len(s.Files) > 0 {
			filename = filepath.Base(s.Files[0].Path)
		}
		perfs := make([]string, len(s.Performers))
		for i, p := range s.Performers {
			perfs[i] = p.Name
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", s.ID, filename, s.Title, strings.Join(perfs, ", "))
	}
	_ = w.Flush()

	if top > 0 && total > len(scenes) {
		fmt.Printf("\nShowing %d of %d unmatched scene(s) (use --top 0 for all)\n", len(scenes), total)
	} else {
		fmt.Printf("\n%d unmatched scene(s)\n", total)
	}
	return nil
}
