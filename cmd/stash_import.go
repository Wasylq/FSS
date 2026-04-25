package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
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
	stashImportCmd.Flags().Bool("include-stashbox", false, "also process scenes that have StashDB data")
	stashImportCmd.Flags().String("stashbox-tag", "", "tag for stashbox overrides (default from config)")
	stashImportCmd.Flags().Bool("cover", false, "set cover image from FSS thumbnail (also implicitly enabled when 'cover' is in --fields)")
	stashImportCmd.Flags().Bool("cover-allow-private", false, "allow cover URLs that resolve to private/loopback IPs (for local media servers); disabled by default to prevent SSRF when importing third-party JSON")
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
	partial   int // UpdateScene succeeded but one or more Ensure*/cover calls failed
	failed    int // UpdateScene itself failed
}

// importFailure records a per-scene operation that did not complete.
// Ensure* failures inside a scene that still updates result in a "partial"
// stat; UpdateScene failures result in a "failed" stat.
type importFailure struct {
	SceneID  string
	Filename string
	Op       string // e.g. "tag", "performer", "studio", "cover", "update"
	Name     string // the name that failed (tag name, performer name, etc.); empty for "update"
	Err      error
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
	coverAllowPrivate, _ := cmd.Flags().GetBool("cover-allow-private")
	includeStashbox, _ := cmd.Flags().GetBool("include-stashbox")
	organized, _ := cmd.Flags().GetBool("organized")

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

	// Listing "cover" in --fields explicitly opts in to cover updates, so
	// don't also require --cover. The reverse (--cover with --fields not
	// listing "cover") still skips cover, since the fields filter is a hard
	// allowlist.
	setCover = resolveCoverEnabled(setCover, allowedFields)

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
	var failures []importFailure
	lookup := newEntityLookup(ctx, client)
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
			// Probe Stash for any entities this scene would add but that don't
			// yet exist. Cached across scenes so the same tag isn't queried twice.
			if fieldAllowed(allowedFields, "tags") {
				for _, t := range allTags {
					lookup.checkTag(t)
				}
			}
			if fieldAllowed(allowedFields, "performers") {
				for _, p := range merged.Performers {
					lookup.checkPerformer(p)
				}
			}
			if fieldAllowed(allowedFields, "studio") && merged.Studio != "" {
				lookup.checkStudio(merged.Studio)
			}
			fmt.Println()
			continue
		}

		// --- apply mode ---
		input := stash.SceneUpdateInput{ID: ss.ID}
		var sceneFailures []importFailure

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
					sceneFailures = append(sceneFailures, importFailure{
						SceneID: ss.ID, Filename: filename, Op: "tag (stashbox)", Name: stashboxTag, Err: tagErr,
					})
				} else {
					tagIDs = append(tagIDs, sbTagID)
				}
			}
			for _, t := range allTags {
				tid, tagErr := client.EnsureTag(ctx, t)
				if tagErr != nil {
					sceneFailures = append(sceneFailures, importFailure{
						SceneID: ss.ID, Filename: filename, Op: "tag", Name: t, Err: tagErr,
					})
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
					sceneFailures = append(sceneFailures, importFailure{
						SceneID: ss.ID, Filename: filename, Op: "performer", Name: p, Err: perfErr,
					})
					continue
				}
				perfIDs = append(perfIDs, pid)
			}
			input.PerformerIDs = dedup(perfIDs)
		}
		if fieldAllowed(allowedFields, "studio") && merged.Studio != "" {
			sid, studioErr := client.EnsureStudio(ctx, merged.Studio)
			if studioErr != nil {
				sceneFailures = append(sceneFailures, importFailure{
					SceneID: ss.ID, Filename: filename, Op: "studio", Name: merged.Studio, Err: studioErr,
				})
			} else {
				input.StudioID = &sid
			}
		}
		if organized {
			input.Organized = &organized
		}
		if fieldAllowed(allowedFields, "cover") && setCover && merged.Thumbnail != "" {
			coverData, coverErr := client.DownloadCoverImage(ctx, merged.Thumbnail, coverAllowPrivate)
			if coverErr != nil {
				sceneFailures = append(sceneFailures, importFailure{
					SceneID: ss.ID, Filename: filename, Op: "cover", Name: merged.Thumbnail, Err: coverErr,
				})
			} else {
				input.CoverImage = &coverData
			}
		}

		if err := client.UpdateScene(ctx, input); err != nil {
			stats.failed++
			failures = append(failures, importFailure{
				SceneID: ss.ID, Filename: filename, Op: "update", Err: err,
			})
			// Drop any sceneFailures collected before the failed update — the
			// scene wasn't written, so per-field ensure failures don't matter.
			continue
		}

		if len(sceneFailures) > 0 {
			stats.partial++
			failures = append(failures, sceneFailures...)
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

	if apply {
		printFailureSummary(failures)
	} else {
		printWouldCreateSummary(lookup)
	}

	// Summary.
	fmt.Println()
	if apply {
		fmt.Printf("Done: %d matched, %d updated, %d partial, %d failed, %d already up-to-date, %d skipped, %d ambiguous\n",
			stats.matched, stats.updated, stats.partial, stats.failed, stats.upToDate, stats.skipped, stats.ambiguous)
	} else {
		fmt.Printf("Dry-run: %d would match, %d already up-to-date, %d skipped, %d ambiguous\n",
			stats.matched, stats.upToDate, stats.skipped, stats.ambiguous)
	}
	return nil
}

// entityLookup caches Stash existence checks across scenes so the same tag /
// performer / studio name isn't queried multiple times during a dry run. Used
// to populate the "would create on apply" summary.
//
// Map values: true = exists in Stash, false = does not exist (would be created).
// A name is absent from the map until checkX is called for it.
type entityLookup struct {
	ctx    context.Context
	client *stash.Client

	tags       map[string]bool
	performers map[string]bool
	studios    map[string]bool
}

func newEntityLookup(ctx context.Context, c *stash.Client) *entityLookup {
	return &entityLookup{
		ctx:        ctx,
		client:     c,
		tags:       map[string]bool{},
		performers: map[string]bool{},
		studios:    map[string]bool{},
	}
}

func (l *entityLookup) checkTag(name string) {
	if _, seen := l.tags[name]; seen {
		return
	}
	_, found, err := l.client.FindTagByName(l.ctx, name)
	if err == nil && !found {
		_, found, err = l.client.FindTagByAlias(l.ctx, name)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: looking up tag %q: %v\n", name, err)
		l.tags[name] = true // treat as existing so it doesn't appear in the would-create list
		return
	}
	l.tags[name] = found
}

func (l *entityLookup) checkPerformer(name string) {
	if _, seen := l.performers[name]; seen {
		return
	}
	_, found, err := l.client.FindPerformerByName(l.ctx, name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: looking up performer %q: %v\n", name, err)
		l.performers[name] = true
		return
	}
	l.performers[name] = found
}

func (l *entityLookup) checkStudio(name string) {
	if _, seen := l.studios[name]; seen {
		return
	}
	_, found, err := l.client.FindStudioByName(l.ctx, name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: looking up studio %q: %v\n", name, err)
		l.studios[name] = true
		return
	}
	l.studios[name] = found
}

// printWouldCreateSummary writes a sorted, grouped list of entities that
// would be created in Stash on `--apply`. No-op if nothing would be created.
func printWouldCreateSummary(l *entityLookup) {
	tags := wouldCreateNames(l.tags)
	perfs := wouldCreateNames(l.performers)
	studios := wouldCreateNames(l.studios)

	if len(tags) == 0 && len(perfs) == 0 && len(studios) == 0 {
		return
	}

	fmt.Println()
	fmt.Println("Would create on apply:")
	for _, t := range tags {
		fmt.Printf("  + tag       %q\n", t)
	}
	for _, p := range perfs {
		fmt.Printf("  + performer %q\n", p)
	}
	for _, s := range studios {
		fmt.Printf("  + studio    %q\n", s)
	}
}

func wouldCreateNames(m map[string]bool) []string {
	var out []string
	for name, exists := range m {
		if !exists {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

// printFailureSummary writes a grouped, scene-by-scene report of all per-scene
// failures collected during apply mode. No-op if there are no failures.
// Output goes to stderr so it stays out of the way when piping.
func printFailureSummary(failures []importFailure) {
	if len(failures) == 0 {
		return
	}

	bySceneID := make(map[string][]importFailure)
	var sceneOrder []string
	for _, f := range failures {
		if _, seen := bySceneID[f.SceneID]; !seen {
			sceneOrder = append(sceneOrder, f.SceneID)
		}
		bySceneID[f.SceneID] = append(bySceneID[f.SceneID], f)
	}

	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "Failures (%d operations across %d scenes):\n", len(failures), len(bySceneID))
	for _, id := range sceneOrder {
		fs := bySceneID[id]
		fmt.Fprintf(os.Stderr, "  scene %s (%s):\n", id, fs[0].Filename)
		for _, f := range fs {
			if f.Name != "" {
				fmt.Fprintf(os.Stderr, "    - %s %q: %v\n", f.Op, f.Name, f.Err)
			} else {
				fmt.Fprintf(os.Stderr, "    - %s: %v\n", f.Op, f.Err)
			}
		}
	}
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
	switch {
	case errors.Is(err, fs.ErrNotExist):
		// First run — no existing changelog, start fresh.
	case err != nil:
		return fmt.Errorf("reading changelog %s: %w", path, err)
	default:
		if err := json.Unmarshal(data, &existing); err != nil {
			backup := filepath.Join(dir, fmt.Sprintf("fss-stashbox-changelog.corrupt-%s.json", time.Now().UTC().Format("20060102-150405")))
			if renameErr := os.Rename(path, backup); renameErr != nil {
				return fmt.Errorf("changelog %s is corrupt and could not be backed up to %s: %w", path, backup, renameErr)
			}
			fmt.Fprintf(os.Stderr, "warning: changelog %s was corrupt (%v); backed up to %s and starting fresh\n", path, err, backup)
			existing = nil
		}
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

// resolveCoverEnabled returns the effective value for the cover-update toggle,
// implicitly enabling it when --fields explicitly lists "cover" so the user
// doesn't have to pass both --cover and --fields cover. The reverse case
// (--cover with --fields not listing cover) still skips cover, because the
// fields filter is a hard allowlist.
func resolveCoverEnabled(setCoverFlag bool, allowedFields map[string]bool) bool {
	if setCoverFlag {
		return true
	}
	return allowedFields != nil && allowedFields["cover"]
}
