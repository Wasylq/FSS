package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/Wasylq/FSS/identify"
	"github.com/Wasylq/FSS/match"
	"github.com/Wasylq/FSS/models"
)

var identifyCmd = &cobra.Command{
	Use:   "identify <video-dir>",
	Short: "Match video files against FSS metadata and write NFO sidecar files",
	Long: `Walk a directory of video files, match them against FSS scene metadata,
and write .nfo sidecar files alongside each matched video.

The .nfo files can be picked up by Stash via the community NFO scraper.

Default is dry-run — shows what would be written. Pass --apply to write.`,
	Args: cobra.ExactArgs(1),
	RunE: runIdentify,
}

func init() {
	rootCmd.AddCommand(identifyCmd)

	identifyCmd.Flags().StringSlice("json", nil, "FSS JSON files to load")
	identifyCmd.Flags().String("dir", "", "directory containing FSS JSON files (default: config out_dir)")
	identifyCmd.Flags().Bool("apply", false, "actually write .nfo files (default is dry-run)")
	identifyCmd.Flags().Bool("force", false, "overwrite existing .nfo files")
	identifyCmd.Flags().Bool("no-report", false, "do not write fss-report.txt")
}

func runIdentify(cmd *cobra.Command, args []string) error {
	videoDir := args[0]

	absDir, err := filepath.Abs(videoDir)
	if err != nil {
		return fmt.Errorf("resolving video directory: %w", err)
	}

	// --- load FSS scenes ---
	jsonFiles, _ := cmd.Flags().GetStringSlice("json")
	dir, _ := cmd.Flags().GetString("dir")
	if dir == "" && len(jsonFiles) == 0 {
		dir = cfg.OutDir
	}

	fmt.Print("Loading FSS JSON files...")
	var fssScenes []models.Scene
	if len(jsonFiles) > 0 {
		fmt.Printf(" %d file(s)...", len(jsonFiles))
		fssScenes, err = match.LoadJSONFiles(jsonFiles)
	} else {
		fmt.Printf(" from %s...", dir)
		fssScenes, err = match.LoadJSONDir(dir)
	}
	if err != nil {
		return fmt.Errorf("loading FSS data: %w", err)
	}
	fmt.Println()
	if len(fssScenes) == 0 {
		return fmt.Errorf("no FSS scenes found")
	}
	fmt.Printf("Loaded %d FSS scenes\n", len(fssScenes))

	idx := match.BuildIndex(fssScenes)

	// --- find videos ---
	fmt.Printf("Scanning %s for video files...\n", absDir)
	videos, err := identify.FindVideos(absDir)
	if err != nil {
		return fmt.Errorf("scanning video directory: %w", err)
	}
	if len(videos) == 0 {
		return fmt.Errorf("no video files found in %s", absDir)
	}
	fmt.Printf("Found %d video files\n", len(videos))

	// --- match and write ---
	apply, _ := cmd.Flags().GetBool("apply")
	force, _ := cmd.Flags().GetBool("force")
	noReport, _ := cmd.Flags().GetBool("no-report")

	results := identify.Run(videos, idx, identify.Options{
		Apply: apply,
		Force: force,
	})

	// --- print results ---
	if !apply {
		fmt.Println("\n--- DRY RUN (pass --apply to write .nfo files) ---")
	}
	fmt.Println()

	for _, r := range results {
		rel, _ := filepath.Rel(absDir, r.VideoPath)
		if rel == "" {
			rel = r.VideoPath
		}

		if r.Skipped {
			fmt.Printf("  SKIP       %-50s  (%s)\n", rel, r.SkipReason)
			continue
		}

		if r.Scene == nil {
			continue
		}

		fmt.Printf("  %-10s %-50s  →  %q", r.Confidence, rel, r.Scene.Title)
		if len(r.Scene.Sites) > 0 {
			fmt.Printf(" (%s)", r.Scene.Sites[0])
		}
		fmt.Println()
	}

	stats := identify.Summarize(results)
	fmt.Println()
	if apply {
		fmt.Printf("%d matched and written, %d unmatched, %d skipped, %d ambiguous\n",
			stats.Matched, stats.Unmatched, stats.Skipped, stats.Ambiguous)
	} else {
		fmt.Printf("Dry-run: %d would match, %d unmatched, %d skipped, %d ambiguous\n",
			stats.Matched, stats.Unmatched, stats.Skipped, stats.Ambiguous)
	}

	// --- write report ---
	if !noReport {
		if err := identify.WriteReport(absDir, results); err != nil {
			return fmt.Errorf("writing report: %w", err)
		}
	}

	return nil
}
