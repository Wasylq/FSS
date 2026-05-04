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

	"github.com/spf13/cobra"

	"github.com/Wasylq/FSS/stash"
)

var stashRevertCmd = &cobra.Command{
	Use:   "revert <stash-scene-id> [<stash-scene-id> ...]",
	Short: "Undo a prior --include-stashbox import using the changelog",
	Long: `Undo changes recorded in fss-stashbox-changelog.json for one or more scenes.

Reverts: title, date, urls (the added ones), tags (the added ones), performers
(the added ones).

NOT reverted (limitations of what the changelog stores):
  - details: only a 60-char preview is recorded; can't restore the original.
  - cover:   the original cover URL was never recorded.

Default is dry-run — pass --apply to actually write changes.`,
	Args: cobra.MinimumNArgs(1),
	RunE: runStashRevert,
}

func init() {
	stashCmd.AddCommand(stashRevertCmd)

	stashRevertCmd.Flags().String("dir", "", "directory containing fss-stashbox-changelog.json (default: config out_dir)")
	stashRevertCmd.Flags().Bool("apply", false, "actually write changes (default is dry-run)")
	stashRevertCmd.Flags().Bool("all", false, "revert every changelog entry for the scene (default: only the most recent)")
	stashRevertCmd.Flags().StringSlice("fields", nil, "only revert these fields (title,date,urls,tags,performers); default: all revertible")
}

// revertableFields is the subset of validImportFields that revert can actually
// restore. details/cover are excluded — see stashRevertCmd.Long.
var revertableFields = map[string]bool{
	"title": true, "date": true, "urls": true, "tags": true, "performers": true,
}

func runStashRevert(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	dir, _ := cmd.Flags().GetString("dir")
	if dir == "" {
		dir = cfg.OutDir
	}
	apply, _ := cmd.Flags().GetBool("apply")
	all, _ := cmd.Flags().GetBool("all")
	fieldsList, _ := cmd.Flags().GetStringSlice("fields")
	allowed, err := parseRevertFields(fieldsList)
	if err != nil {
		return err
	}

	entries, err := loadChangelog(dir)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return fmt.Errorf("changelog at %s is empty", filepath.Join(dir, "fss-stashbox-changelog.json"))
	}

	client := stash.NewClient(stashURL(cmd), stashAPIKey(cmd))
	if err := client.Ping(ctx); err != nil {
		return fmt.Errorf("connecting to stash: %w", err)
	}
	fmt.Println("Connected to Stash")

	if !apply {
		fmt.Print("\n--- DRY RUN (pass --apply to write changes) ---\n\n")
	}

	var anyApplied, anyMatched int
	for _, sceneID := range args {
		matches := entriesForScene(entries, sceneID)
		if len(matches) == 0 {
			fmt.Fprintf(os.Stderr, "warning: no changelog entries for scene %s\n", sceneID)
			continue
		}
		if !all {
			matches = matches[len(matches)-1:] // most recent only
		}
		anyMatched += len(matches)

		current, found, err := client.FindSceneByID(ctx, sceneID)
		if err != nil {
			return fmt.Errorf("fetching scene %s: %w", sceneID, err)
		}
		if !found {
			fmt.Fprintf(os.Stderr, "warning: scene %s not found in Stash, skipping\n", sceneID)
			continue
		}

		// Apply the entries oldest-first so multi-entry reverts compose correctly.
		// Each entry is computed against the current (possibly already partially
		// reverted) state.
		for _, entry := range matches {
			input, plan, skipped := computeRevert(entry, *current, allowed)
			fmt.Printf("scene %s (%s) [entry %s]:\n", sceneID, entry.Filename, entry.Timestamp.Format("2006-01-02T15:04:05Z"))
			if len(plan) == 0 && len(skipped) == 0 {
				fmt.Println("  nothing to revert")
				continue
			}
			for _, line := range plan {
				fmt.Printf("  %s\n", line)
			}
			for _, s := range skipped {
				fmt.Printf("  (skipped) %s\n", s)
			}

			if !apply {
				continue
			}

			// Resolve tag/performer NAMES → IDs, drop them from the current
			// id list, and feed the remainder back into the input.
			if len(input.removeTagNames) > 0 {
				ids, err := resolveExistingTagIDs(ctx, client, input.removeTagNames)
				if err != nil {
					return err
				}
				input.scene.TagIDs = subtractIDs(currentTagIDs(*current), ids)
			}
			if len(input.removePerfNames) > 0 {
				ids, err := resolveExistingPerfIDs(ctx, client, input.removePerfNames)
				if err != nil {
					return err
				}
				input.scene.PerformerIDs = subtractIDs(currentPerfIDs(*current), ids)
			}

			if err := client.UpdateScene(ctx, input.scene); err != nil {
				fmt.Fprintf(os.Stderr, "error reverting scene %s: %v\n", sceneID, err)
				continue
			}
			anyApplied++

			// Refresh the in-memory current state for subsequent --all entries.
			refreshed, _, err := client.FindSceneByID(ctx, sceneID)
			if err == nil && refreshed != nil {
				current = refreshed
			}
		}
	}

	fmt.Println()
	if apply {
		fmt.Printf("Done: %d entries matched, %d applied\n", anyMatched, anyApplied)
	} else {
		fmt.Printf("Dry-run: %d entries would be reverted\n", anyMatched)
	}
	return nil
}

// parseRevertFields validates the --fields list against the revertable set.
// Returns nil to mean "all revertable fields".
func parseRevertFields(fields []string) (map[string]bool, error) {
	if len(fields) == 0 {
		return nil, nil
	}
	out := make(map[string]bool, len(fields))
	for _, f := range fields {
		f = strings.TrimSpace(f)
		if !revertableFields[f] {
			valid := make([]string, 0, len(revertableFields))
			for k := range revertableFields {
				valid = append(valid, k)
			}
			sort.Strings(valid)
			return nil, fmt.Errorf("--fields: %q is not revertable (valid: %s)", f, strings.Join(valid, ","))
		}
		out[f] = true
	}
	return out, nil
}

// loadChangelog returns the changelog entries from the given directory, or an
// error if the file is missing/unreadable. An empty slice is returned for an
// empty file.
func loadChangelog(dir string) ([]changelogEntry, error) {
	path := filepath.Join(dir, "fss-stashbox-changelog.json")
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("no changelog at %s — was --include-stashbox ever used here?", path)
	}
	if err != nil {
		return nil, fmt.Errorf("reading changelog %s: %w", path, err)
	}
	if len(data) == 0 {
		return nil, nil
	}
	var entries []changelogEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("parsing changelog %s: %w", path, err)
	}
	return entries, nil
}

func entriesForScene(entries []changelogEntry, sceneID string) []changelogEntry {
	var out []changelogEntry
	for _, e := range entries {
		if e.StashSceneID == sceneID {
			out = append(out, e)
		}
	}
	return out
}

// revertInput packages the scene update plus the name-keyed deletion lists
// that need ID resolution before the update can be issued.
type revertInput struct {
	scene           stash.SceneUpdateInput
	removeTagNames  []string
	removePerfNames []string
}

// computeRevert builds the scene update for a single changelog entry.
// Returns the input, a human-readable plan (one line per field), and a list
// of skipped fields (with reason) that the caller should print as warnings.
func computeRevert(entry changelogEntry, current stash.StashScene, allowed map[string]bool) (revertInput, []string, []string) {
	in := revertInput{scene: stash.SceneUpdateInput{ID: current.ID}}
	var plan, skipped []string

	allow := func(field string) bool {
		return allowed == nil || allowed[field]
	}

	for field, diff := range entry.Changes {
		switch field {
		case "title":
			if !allow(field) {
				continue
			}
			from := stringValue(diff.From)
			if from == "" {
				skipped = append(skipped, "title: original was empty, leaving current")
				continue
			}
			s := from
			in.scene.Title = &s
			plan = append(plan, fmt.Sprintf("title: %q → %q", current.Title, from))

		case "date":
			if !allow(field) {
				continue
			}
			from := stringValue(diff.From)
			if from == "" {
				skipped = append(skipped, "date: original was empty, leaving current")
				continue
			}
			d := from
			in.scene.Date = &d
			plan = append(plan, fmt.Sprintf("date: %q → %q", current.Date, from))

		case "urls":
			if !allow(field) {
				continue
			}
			if len(diff.Added) == 0 {
				continue
			}
			in.scene.URLs = subtractStrings(current.URLs, diff.Added)
			plan = append(plan, fmt.Sprintf("urls: -%v", diff.Added))

		case "tags":
			if !allow(field) {
				continue
			}
			if len(diff.Added) == 0 {
				continue
			}
			in.removeTagNames = diff.Added
			plan = append(plan, fmt.Sprintf("tags: -%v", diff.Added))

		case "performers":
			if !allow(field) {
				continue
			}
			if len(diff.Added) == 0 {
				continue
			}
			in.removePerfNames = diff.Added
			plan = append(plan, fmt.Sprintf("performers: -%v", diff.Added))

		case "details":
			skipped = append(skipped, "details: only a 60-char preview was recorded; can't restore original")

		case "cover":
			skipped = append(skipped, "cover: original URL was never recorded; can't restore")
		}
	}

	// Sort plan/skipped lines for stable output regardless of map iteration order.
	sort.Strings(plan)
	sort.Strings(skipped)
	return in, plan, skipped
}

// stringValue extracts a string from an `any` field that came back via JSON.
// JSON unmarshalling into `any` produces string for JSON strings, so this is
// the common case. Returns "" for nil or non-string types.
func stringValue(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// subtractStrings returns `from` with any element of `remove` filtered out.
// Order of `from` is preserved.
func subtractStrings(from, remove []string) []string {
	if len(remove) == 0 {
		return from
	}
	skip := make(map[string]bool, len(remove))
	for _, s := range remove {
		skip[s] = true
	}
	out := make([]string, 0, len(from))
	for _, s := range from {
		if !skip[s] {
			out = append(out, s)
		}
	}
	return out
}

// subtractIDs is identical to subtractStrings but named for readability at
// call sites that work with Stash entity IDs.
func subtractIDs(from, remove []string) []string {
	return subtractStrings(from, remove)
}

func currentTagIDs(s stash.StashScene) []string {
	ids := make([]string, len(s.Tags))
	for i, t := range s.Tags {
		ids[i] = t.ID
	}
	return ids
}

func currentPerfIDs(s stash.StashScene) []string {
	ids := make([]string, len(s.Performers))
	for i, p := range s.Performers {
		ids[i] = p.ID
	}
	return ids
}

// resolveExistingTagIDs returns the Stash IDs for tags whose names are in the
// list. Names with no matching tag are silently dropped (nothing to remove).
func resolveExistingTagIDs(ctx context.Context, client *stash.Client, names []string) ([]string, error) {
	var ids []string
	for _, n := range names {
		id, found, err := client.FindTagByName(ctx, n)
		if err != nil {
			return nil, err
		}
		if found {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

func resolveExistingPerfIDs(ctx context.Context, client *stash.Client, names []string) ([]string, error) {
	var ids []string
	for _, n := range names {
		id, found, err := client.FindPerformerByName(ctx, n)
		if err != nil {
			return nil, err
		}
		if found {
			ids = append(ids, id)
		}
	}
	return ids, nil
}
