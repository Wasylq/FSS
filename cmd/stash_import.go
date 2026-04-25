package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/Wasylq/FSS/internal/stash"
	"github.com/Wasylq/FSS/models"
)

var stashImportCmd = &cobra.Command{
	Use:   "import",
	Short: "Match FSS JSON scenes against Stash and push metadata",
	Long: `Match FSS JSON scenes against Stash scenes by filename, then push metadata.

Default is dry-run — shows what would change. Pass --apply to actually write to Stash.`,
	RunE: runStashImport,
}

func init() {
	stashCmd.AddCommand(stashImportCmd)

	stashImportCmd.Flags().String("dir", "", "directory containing FSS JSON files (default: config out_dir)")
	stashImportCmd.Flags().StringSlice("json", nil, "specific JSON files to load")
	stashImportCmd.Flags().String("tag", "", "import marker tag (default from config)")
	stashImportCmd.Flags().Bool("resolution-tags", false, "add resolution tags (4K/FHD/HD Available)")
	stashImportCmd.Flags().Bool("organized", false, "set organized flag on imported scenes")
	stashImportCmd.Flags().Bool("scrape", false, "call Stash scraper on first URL after import")
	stashImportCmd.Flags().Bool("include-stashbox", false, "also process scenes that have StashDB data")
	stashImportCmd.Flags().String("stashbox-tag", "", "tag for stashbox overrides (default from config)")
	stashImportCmd.Flags().Bool("cover", false, "set cover image from FSS thumbnail")
	stashImportCmd.Flags().Bool("apply", false, "actually write changes (default is dry-run)")
	stashImportCmd.Flags().String("performer", "", "filter Stash scenes by performer name")
	stashImportCmd.Flags().String("studio", "", "filter Stash scenes by studio name")
	stashImportCmd.Flags().Int("top", 0, "limit number of Stash scenes to process (0 = all)")
	stashImportCmd.Flags().StringSlice("fields", nil, "only update these fields (title,details,date,urls,tags,performers,studio,cover); default: all")
}

type importStats struct {
	total     int
	matched   int
	updated   int
	skipped   int
	ambiguous int
	upToDate  int
}

type changelogEntry struct {
	StashSceneID string                       `json:"stash_scene_id"`
	Timestamp    time.Time                    `json:"timestamp"`
	Filename     string                       `json:"filename"`
	MatchedTo    string                       `json:"matched_to"`
	Changes      map[string]changelogFieldDiff `json:"changes"`
}

type changelogFieldDiff struct {
	From  any      `json:"from,omitempty"`
	To    any      `json:"to,omitempty"`
	Added []string `json:"added,omitempty"`
}

func runStashImport(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()

	client := stash.NewClient(stashURL(cmd), stashAPIKey(cmd))
	if err := client.Ping(ctx); err != nil {
		return fmt.Errorf("connecting to stash: %w", err)
	}
	fmt.Println("Connected to Stash")

	// --- load FSS scenes ---
	jsonFiles, _ := cmd.Flags().GetStringSlice("json")
	dir, _ := cmd.Flags().GetString("dir")
	if dir == "" {
		dir = cfg.OutDir
	}

	fmt.Print("Loading FSS JSON files...")
	var fssScenes []models.Scene
	var err error
	if len(jsonFiles) > 0 {
		fmt.Printf(" %d file(s)...", len(jsonFiles))
		fssScenes, err = stash.LoadJSONFiles(jsonFiles)
	} else {
		fmt.Printf(" from %s...", dir)
		fssScenes, err = stash.LoadJSONDir(dir)
	}
	if err != nil {
		return fmt.Errorf("loading FSS data: %w", err)
	}
	fmt.Println()
	if len(fssScenes) == 0 {
		return fmt.Errorf("no FSS scenes found in %s", dir)
	}
	fmt.Printf("Loaded %d FSS scenes\n", len(fssScenes))

	idx := stash.BuildIndex(fssScenes)

	// --- resolve flags ---
	apply, _ := cmd.Flags().GetBool("apply")
	setCover, _ := cmd.Flags().GetBool("cover")
	includeStashbox, _ := cmd.Flags().GetBool("include-stashbox")
	organized, _ := cmd.Flags().GetBool("organized")
	scrapeFlag, _ := cmd.Flags().GetBool("scrape")
	if !cmd.Flags().Changed("scrape") {
		scrapeFlag = cfg.Stash.Scrape
	}

	resolutionTags, _ := cmd.Flags().GetBool("resolution-tags")
	if !cmd.Flags().Changed("resolution-tags") {
		resolutionTags = cfg.Stash.ResolutionTags
	}

	tagName, _ := cmd.Flags().GetString("tag")
	if tagName == "" {
		tagName = cfg.Stash.Tag
	}

	stashboxTag, _ := cmd.Flags().GetString("stashbox-tag")
	if stashboxTag == "" {
		stashboxTag = cfg.Stash.StashboxTag
	}

	fieldsList, _ := cmd.Flags().GetStringSlice("fields")
	allowedFields, err := parseFieldsFlag(fieldsList)
	if err != nil {
		return err
	}

	performer, _ := cmd.Flags().GetString("performer")
	studio, _ := cmd.Flags().GetString("studio")
	top, _ := cmd.Flags().GetInt("top")

	// --- query stash scenes ---
	zero := 0
	filter := stash.FindScenesFilter{
		PerformerName: performer,
		StudioName:    studio,
	}
	if !includeStashbox {
		filter.StashIDCount = &zero
	}

	fmt.Print("Querying Stash scenes...")
	var stashScenes []stash.StashScene
	if top > 0 {
		stashScenes, _, err = client.FindScenes(ctx, filter, 1, top)
	} else {
		stashScenes, err = client.FindAllScenes(ctx, filter, func(fetched, total int) {
			fmt.Printf("\rQuerying Stash scenes... %d / %d", fetched, total)
		})
	}
	fmt.Println()
	if err != nil {
		return fmt.Errorf("querying stash scenes: %w", err)
	}
	fmt.Printf("Found %d Stash scenes to process\n", len(stashScenes))

	if !apply {
		fmt.Print("\n--- DRY RUN (pass --apply to write changes) ---\n")
		if allowedFields != nil {
			fmt.Printf("Fields: %s\n", strings.Join(fieldsList, ", "))
		}
		fmt.Println()
	}

	// --- ensure import tag ---
	var importTagID string
	if apply && fieldAllowed(allowedFields, "tags") {
		importTagID, err = client.EnsureTag(ctx, tagName)
		if err != nil {
			return fmt.Errorf("ensuring import tag %q: %w", tagName, err)
		}
	}

	var changelog []changelogEntry
	stats := importStats{total: len(stashScenes)}

	for _, ss := range stashScenes {
		if ctx.Err() != nil {
			break
		}

		filename := ""
		if len(ss.Files) > 0 {
			filename = filepath.Base(ss.Files[0].Path)
		}
		if filename == "" {
			stats.skipped++
			continue
		}

		var fileDuration float64
		if len(ss.Files) > 0 {
			fileDuration = ss.Files[0].Duration
		}
		result := idx.Match(filename, fileDuration)

		switch result.Confidence {
		case stash.MatchNone:
			stats.skipped++
			continue
		case stash.MatchAmbiguous:
			stats.ambiguous++
			fmt.Printf("  AMBIGUOUS  %-50s  →  %d candidates, skipped\n\n", truncate(filename, 50), result.Candidates)
			continue
		}

		hasStashbox := len(ss.StashIDs) > 0

		// Parse existing stash date for earliest-date logic.
		var existingDate time.Time
		if ss.Date != "" {
			existingDate, _ = time.Parse("2006-01-02", ss.Date)
		}

		merged := stash.MergeScenes(result.Scenes, existingDate)
		sites := strings.Join(merged.Sites, " + ")
		stashBase := stashURL(cmd)
		fmt.Printf("  %-10s %-50s  →  %q (%s)\n", result.Confidence, truncate(filename, 50), merged.Title, sites)
		fmt.Printf("           %s/scenes/%s\n", stashBase, ss.ID)

		// Collect all tag names to add.
		allTags := merged.Tags
		allTags = append(allTags, merged.Categories...)
		allTags = append(allTags, tagName)
		if hasStashbox {
			allTags = append(allTags, stashboxTag)
		}
		if resolutionTags {
			allTags = append(allTags, stash.ResolutionTags(merged.Width)...)
		}

		// Merge URLs with existing.
		existingURLs := ss.URLs
		mergedURLs := stash.MergeURLs(existingURLs, merged.URLs)

		// Check if there's anything to change.
		changes := buildChanges(ss, merged, mergedURLs, allTags, setCover)
		if allowedFields != nil {
			for field := range changes {
				if !allowedFields[field] {
					delete(changes, field)
				}
			}
		}
		if len(changes) == 0 {
			stats.upToDate++
			continue
		}

		stats.matched++

		if !apply {
			for field, diff := range changes {
				if len(diff.Added) > 0 {
					fmt.Printf("    %s: +%v\n", field, diff.Added)
				} else {
					fmt.Printf("    %s: %v → %v\n", field, diff.From, diff.To)
				}
			}
			fmt.Println()
			continue
		}

		// --- apply mode ---
		input := stash.SceneUpdateInput{ID: ss.ID}

		if fieldAllowed(allowedFields, "title") {
			input.Title = strPtr(merged.Title)
		}
		if fieldAllowed(allowedFields, "details") && merged.Description != "" {
			input.Details = strPtr(merged.Description)
		}
		if fieldAllowed(allowedFields, "date") && !merged.Date.IsZero() {
			d := merged.Date.Format("2006-01-02")
			input.Date = &d
		}
		if fieldAllowed(allowedFields, "urls") {
			input.URLs = mergedURLs
		}
		if fieldAllowed(allowedFields, "tags") {
			tagIDs := extractTagIDs(ss.Tags)
			tagIDs = append(tagIDs, importTagID)
			if hasStashbox {
				sbTagID, tagErr := client.EnsureTag(ctx, stashboxTag)
				if tagErr != nil {
					return fmt.Errorf("ensuring stashbox override tag: %w", tagErr)
				}
				tagIDs = append(tagIDs, sbTagID)
			}
			for _, t := range allTags {
				tid, tagErr := client.EnsureTag(ctx, t)
				if tagErr != nil {
					fmt.Fprintf(os.Stderr, "warning: could not ensure tag %q: %v\n", t, tagErr)
					continue
				}
				tagIDs = append(tagIDs, tid)
			}
			input.TagIDs = dedup(tagIDs)
		}
		if fieldAllowed(allowedFields, "performers") {
			perfIDs := extractPerfIDs(ss.Performers)
			for _, p := range merged.Performers {
				pid, perfErr := client.EnsurePerformer(ctx, p)
				if perfErr != nil {
					fmt.Fprintf(os.Stderr, "warning: could not ensure performer %q: %v\n", p, perfErr)
					continue
				}
				perfIDs = append(perfIDs, pid)
			}
			input.PerformerIDs = dedup(perfIDs)
		}
		if fieldAllowed(allowedFields, "studio") && merged.Studio != "" {
			sid, studioErr := client.EnsureStudio(ctx, merged.Studio)
			if studioErr != nil {
				fmt.Fprintf(os.Stderr, "warning: could not ensure studio %q: %v\n", merged.Studio, studioErr)
			} else {
				input.StudioID = &sid
			}
		}
		if organized {
			input.Organized = &organized
		}
		if fieldAllowed(allowedFields, "cover") && setCover && merged.Thumbnail != "" {
			coverData, coverErr := client.DownloadCoverImage(ctx, merged.Thumbnail)
			if coverErr != nil {
				fmt.Fprintf(os.Stderr, "warning: could not download cover image: %v\n", coverErr)
			} else {
				input.CoverImage = &coverData
			}
		}

		if err := client.UpdateScene(ctx, input); err != nil {
			fmt.Fprintf(os.Stderr, "error updating scene %s: %v\n", ss.ID, err)
			continue
		}

		// Optional scrape from first URL.
		if scrapeFlag && len(merged.URLs) > 0 {
			_, _ = client.ScrapeSceneURL(ctx, merged.URLs[0])
		}

		// Changelog for stashbox overrides.
		if hasStashbox {
			changelog = append(changelog, changelogEntry{
				StashSceneID: ss.ID,
				Timestamp:    time.Now().UTC(),
				Filename:     filename,
				MatchedTo:    merged.Title,
				Changes:      changes,
			})
		}

		stats.updated++
	}

	// Write stashbox changelog if we modified any scenes with StashDB data.
	if len(changelog) > 0 {
		if err := appendChangelog(dir, changelog); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not write stashbox changelog: %v\n", err)
		}
	}

	// Summary.
	fmt.Println()
	if apply {
		fmt.Printf("Done: %d matched, %d updated, %d already up-to-date, %d skipped, %d ambiguous\n",
			stats.matched, stats.updated, stats.upToDate, stats.skipped, stats.ambiguous)
	} else {
		fmt.Printf("Dry-run: %d would match, %d already up-to-date, %d skipped, %d ambiguous\n",
			stats.matched, stats.upToDate, stats.skipped, stats.ambiguous)
	}
	return nil
}

func buildChanges(ss stash.StashScene, merged stash.MergedScene, mergedURLs []string, newTags []string, setCover bool) map[string]changelogFieldDiff {
	changes := map[string]changelogFieldDiff{}

	if merged.Title != "" && merged.Title != ss.Title {
		changes["title"] = changelogFieldDiff{From: ss.Title, To: merged.Title}
	}
	if merged.Description != "" && merged.Description != ss.Details {
		changes["details"] = changelogFieldDiff{From: truncate(ss.Details, 60), To: truncate(merged.Description, 60)}
	}

	if !merged.Date.IsZero() {
		newDate := merged.Date.Format("2006-01-02")
		if newDate != ss.Date {
			changes["date"] = changelogFieldDiff{From: ss.Date, To: newDate}
		}
	}

	addedURLs := diffStrings(ss.URLs, mergedURLs)
	if len(addedURLs) > 0 {
		changes["urls"] = changelogFieldDiff{Added: addedURLs}
	}

	if setCover && merged.Thumbnail != "" {
		changes["cover"] = changelogFieldDiff{To: truncate(merged.Thumbnail, 60)}
	}

	existingTagNames := make(map[string]bool, len(ss.Tags))
	for _, t := range ss.Tags {
		existingTagNames[t.Name] = true
	}
	var addedTags []string
	for _, t := range newTags {
		if !existingTagNames[t] {
			addedTags = append(addedTags, t)
		}
	}
	if len(addedTags) > 0 {
		changes["tags"] = changelogFieldDiff{Added: addedTags}
	}

	existingPerfNames := make(map[string]bool, len(ss.Performers))
	for _, p := range ss.Performers {
		existingPerfNames[p.Name] = true
	}
	var addedPerfs []string
	for _, p := range merged.Performers {
		if !existingPerfNames[p] {
			addedPerfs = append(addedPerfs, p)
		}
	}
	if len(addedPerfs) > 0 {
		changes["performers"] = changelogFieldDiff{Added: addedPerfs}
	}

	return changes
}

func diffStrings(existing, merged []string) []string {
	set := make(map[string]bool, len(existing))
	for _, s := range existing {
		set[s] = true
	}
	var added []string
	for _, s := range merged {
		if !set[s] {
			added = append(added, s)
		}
	}
	return added
}

func appendChangelog(dir string, entries []changelogEntry) error {
	path := filepath.Join(dir, "fss-stashbox-changelog.json")

	var existing []changelogEntry
	data, err := os.ReadFile(path)
	if err == nil {
		_ = json.Unmarshal(data, &existing)
	}

	existing = append(existing, entries...)
	out, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o644)
}

func extractTagIDs(tags []stash.StashTag) []string {
	ids := make([]string, len(tags))
	for i, t := range tags {
		ids[i] = t.ID
	}
	return ids
}

func extractPerfIDs(perfs []stash.StashPerf) []string {
	ids := make([]string, len(perfs))
	for i, p := range perfs {
		ids[i] = p.ID
	}
	return ids
}

func dedup(ss []string) []string {
	seen := make(map[string]bool, len(ss))
	out := make([]string, 0, len(ss))
	for _, s := range ss {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func strPtr(s string) *string { return &s }

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

var validImportFields = map[string]bool{
	"title": true, "details": true, "date": true, "urls": true,
	"tags": true, "performers": true, "studio": true, "cover": true,
}

func parseFieldsFlag(fields []string) (map[string]bool, error) {
	if len(fields) == 0 {
		return nil, nil
	}
	m := make(map[string]bool, len(fields))
	for _, f := range fields {
		f = strings.TrimSpace(f)
		if !validImportFields[f] {
			return nil, fmt.Errorf("unknown field %q (valid: title,details,date,urls,tags,performers,studio,cover)", f)
		}
		m[f] = true
	}
	return m, nil
}

func fieldAllowed(allowed map[string]bool, field string) bool {
	return allowed == nil || allowed[field]
}
