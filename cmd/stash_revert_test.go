package cmd

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Wasylq/FSS/stash"
)

func TestParseRevertFields(t *testing.T) {
	t.Run("empty returns nil", func(t *testing.T) {
		got, err := parseRevertFields(nil)
		if err != nil {
			t.Fatal(err)
		}
		if got != nil {
			t.Errorf("got %v, want nil (= all fields)", got)
		}
	})

	t.Run("valid fields parsed", func(t *testing.T) {
		got, err := parseRevertFields([]string{"tags", "urls"})
		if err != nil {
			t.Fatal(err)
		}
		if !got["tags"] || !got["urls"] || got["title"] {
			t.Errorf("got %v", got)
		}
	})

	t.Run("rejects details (not revertable)", func(t *testing.T) {
		_, err := parseRevertFields([]string{"details"})
		if err == nil {
			t.Error("expected error for non-revertable field 'details'")
		}
	})

	t.Run("rejects cover (not revertable)", func(t *testing.T) {
		_, err := parseRevertFields([]string{"cover"})
		if err == nil {
			t.Error("expected error for non-revertable field 'cover'")
		}
	})

	t.Run("rejects unknown", func(t *testing.T) {
		_, err := parseRevertFields([]string{"banana"})
		if err == nil {
			t.Error("expected error for unknown field")
		}
	})
}

func TestSubtractStrings(t *testing.T) {
	cases := []struct {
		from   []string
		remove []string
		want   []string
	}{
		{[]string{"a", "b", "c"}, []string{"b"}, []string{"a", "c"}},
		{[]string{"a", "b", "c"}, []string{"x"}, []string{"a", "b", "c"}},
		{[]string{"a", "b"}, []string{"a", "b"}, []string{}},
		{[]string{}, []string{"x"}, []string{}},
		{[]string{"a", "b"}, nil, []string{"a", "b"}}, // empty remove = identity
	}
	for _, c := range cases {
		got := subtractStrings(c.from, c.remove)
		if len(got) == 0 && len(c.want) == 0 {
			continue // both empty
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("subtractStrings(%v, %v) = %v, want %v", c.from, c.remove, got, c.want)
		}
	}
}

func TestEntriesForScene(t *testing.T) {
	all := []changelogEntry{
		{StashSceneID: "1", Filename: "a"},
		{StashSceneID: "2", Filename: "b"},
		{StashSceneID: "1", Filename: "c"},
	}
	got := entriesForScene(all, "1")
	if len(got) != 2 || got[0].Filename != "a" || got[1].Filename != "c" {
		t.Errorf("got %+v", got)
	}
	if entriesForScene(all, "9") != nil {
		t.Errorf("expected nil for missing scene")
	}
}

func TestComputeRevert_titleAndDate(t *testing.T) {
	entry := changelogEntry{
		StashSceneID: "42",
		Changes: map[string]changelogFieldDiff{
			"title": {From: "Original", To: "Imported"},
			"date":  {From: "2025-01-15", To: "2024-06-01"},
		},
	}
	current := stash.StashScene{ID: "42", Title: "Imported", Date: "2024-06-01"}

	in, plan, skipped := computeRevert(entry, current, nil)

	if in.scene.Title == nil || *in.scene.Title != "Original" {
		t.Errorf("title not reverted: %+v", in.scene.Title)
	}
	if in.scene.Date == nil || *in.scene.Date != "2025-01-15" {
		t.Errorf("date not reverted: %+v", in.scene.Date)
	}
	if len(skipped) != 0 {
		t.Errorf("unexpected skipped: %v", skipped)
	}
	if len(plan) != 2 {
		t.Errorf("plan should have 2 lines, got %d: %v", len(plan), plan)
	}
}

func TestComputeRevert_urlsRemovesAdded(t *testing.T) {
	entry := changelogEntry{
		StashSceneID: "42",
		Changes: map[string]changelogFieldDiff{
			"urls": {Added: []string{"https://manyvids.com/foo", "https://c4s.com/bar"}},
		},
	}
	// Stash has the imported URLs plus a manually-added one.
	current := stash.StashScene{
		ID:   "42",
		URLs: []string{"https://manyvids.com/foo", "https://c4s.com/bar", "https://user-added.example/x"},
	}

	in, _, _ := computeRevert(entry, current, nil)

	want := []string{"https://user-added.example/x"}
	if !reflect.DeepEqual(in.scene.URLs, want) {
		t.Errorf("got %v, want %v", in.scene.URLs, want)
	}
}

func TestComputeRevert_tagsAndPerfsAreNamesNotIDs(t *testing.T) {
	// computeRevert can't resolve tag/performer NAMES → IDs (that needs a
	// Stash client). It just records the names; the apply path resolves
	// and subtracts. So in.scene.TagIDs/PerformerIDs stay empty here.
	entry := changelogEntry{
		StashSceneID: "42",
		Changes: map[string]changelogFieldDiff{
			"tags":       {Added: []string{"POV", "MILF"}},
			"performers": {Added: []string{"Alice"}},
		},
	}
	current := stash.StashScene{ID: "42"}

	in, plan, _ := computeRevert(entry, current, nil)

	if len(in.scene.TagIDs) != 0 || len(in.scene.PerformerIDs) != 0 {
		t.Errorf("computeRevert must not set IDs (needs the Stash client): %+v", in.scene)
	}
	if !reflect.DeepEqual(in.removeTagNames, []string{"POV", "MILF"}) {
		t.Errorf("removeTagNames = %v", in.removeTagNames)
	}
	if !reflect.DeepEqual(in.removePerfNames, []string{"Alice"}) {
		t.Errorf("removePerfNames = %v", in.removePerfNames)
	}
	// Both should appear in the plan.
	joined := strings.Join(plan, "|")
	if !strings.Contains(joined, "tags") || !strings.Contains(joined, "performers") {
		t.Errorf("plan missing tags or performers: %v", plan)
	}
}

func TestComputeRevert_skipsDetailsAndCover(t *testing.T) {
	entry := changelogEntry{
		StashSceneID: "42",
		Changes: map[string]changelogFieldDiff{
			"details": {From: "Old preview...", To: "New preview..."},
			"cover":   {To: "https://cdn.example/cover.jpg"},
		},
	}
	current := stash.StashScene{ID: "42"}

	in, _, skipped := computeRevert(entry, current, nil)

	if in.scene.Details != nil || in.scene.CoverImage != nil {
		t.Errorf("must not attempt to revert details/cover: %+v", in.scene)
	}
	if len(skipped) != 2 {
		t.Errorf("expected 2 skipped entries, got %d: %v", len(skipped), skipped)
	}
	sort.Strings(skipped)
	if !strings.Contains(skipped[0], "cover") || !strings.Contains(skipped[1], "details") {
		t.Errorf("skipped entries unexpected: %v", skipped)
	}
}

func TestComputeRevert_fieldsFilterRespected(t *testing.T) {
	entry := changelogEntry{
		StashSceneID: "42",
		Changes: map[string]changelogFieldDiff{
			"title": {From: "Original", To: "Imported"},
			"tags":  {Added: []string{"POV"}},
			"urls":  {Added: []string{"https://x"}},
		},
	}
	current := stash.StashScene{ID: "42", Title: "Imported", URLs: []string{"https://x"}}

	// Only allow tags.
	allowed := map[string]bool{"tags": true}
	in, plan, _ := computeRevert(entry, current, allowed)

	if in.scene.Title != nil {
		t.Error("title should not be reverted when not in allowed fields")
	}
	if in.scene.URLs != nil {
		t.Error("urls should not be reverted when not in allowed fields")
	}
	if len(in.removeTagNames) != 1 {
		t.Errorf("tags should be reverted: %+v", in.removeTagNames)
	}
	if len(plan) != 1 || !strings.Contains(plan[0], "tags") {
		t.Errorf("plan should only mention tags: %v", plan)
	}
}

func TestComputeRevert_emptyAddedIsNoop(t *testing.T) {
	// A changelog entry where urls/tags/performers were tracked but the
	// Added list is empty (defensive — shouldn't normally happen).
	entry := changelogEntry{
		StashSceneID: "42",
		Changes: map[string]changelogFieldDiff{
			"urls":       {Added: []string{}},
			"tags":       {Added: nil},
			"performers": {},
		},
	}
	in, plan, _ := computeRevert(entry, stash.StashScene{ID: "42"}, nil)

	if in.scene.URLs != nil || in.removeTagNames != nil || in.removePerfNames != nil {
		t.Errorf("empty Added lists should be no-op: %+v", in)
	}
	if len(plan) != 0 {
		t.Errorf("plan should be empty: %v", plan)
	}
}

func TestLoadChangelog_missingFileError(t *testing.T) {
	_, err := loadChangelog(t.TempDir())
	if err == nil {
		t.Error("expected error when changelog is missing")
	}
	if !strings.Contains(err.Error(), "no changelog") {
		t.Errorf("error should mention missing file, got: %v", err)
	}
}

func TestLoadChangelog_emptyFileReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "fss-stashbox-changelog.json"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := loadChangelog(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("empty file should yield empty entries, got %v", got)
	}
}

func TestLoadChangelog_roundTrip(t *testing.T) {
	dir := t.TempDir()
	want := []changelogEntry{
		{
			StashSceneID: "1",
			Timestamp:    time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC),
			Filename:     "a.mp4",
			Changes: map[string]changelogFieldDiff{
				"title": {From: "old", To: "new"},
				"tags":  {Added: []string{"POV"}},
			},
		},
	}
	writeChangelog(t, filepath.Join(dir, "fss-stashbox-changelog.json"), want)

	got, err := loadChangelog(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d entries, want 1", len(got))
	}
	if got[0].StashSceneID != "1" || got[0].Changes["title"].From != "old" {
		t.Errorf("round-trip mismatch: %+v", got)
	}
}
