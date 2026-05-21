package cmd

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Wasylq/FSS/match"
	"github.com/Wasylq/FSS/stash"
)

func TestAppendChangelog_freshStart(t *testing.T) {
	dir := t.TempDir()
	entries := []changelogEntry{
		{StashSceneID: "1", Timestamp: time.Now().UTC(), Filename: "a.mp4", MatchedTo: "Title A"},
	}

	if err := appendChangelog(dir, entries); err != nil {
		t.Fatalf("appendChangelog: %v", err)
	}

	got := readChangelog(t, dir)
	if len(got) != 1 || got[0].StashSceneID != "1" {
		t.Errorf("got %+v", got)
	}
}

func TestAppendChangelog_appendsToExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fss-stashbox-changelog.json")

	first := []changelogEntry{{StashSceneID: "1", Filename: "a.mp4"}}
	writeChangelog(t, path, first)

	second := []changelogEntry{{StashSceneID: "2", Filename: "b.mp4"}}
	if err := appendChangelog(dir, second); err != nil {
		t.Fatalf("appendChangelog: %v", err)
	}

	got := readChangelog(t, dir)
	if len(got) != 2 || got[0].StashSceneID != "1" || got[1].StashSceneID != "2" {
		t.Errorf("got %+v", got)
	}
}

func TestAppendChangelog_corruptFileBacksUp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fss-stashbox-changelog.json")

	if err := os.WriteFile(path, []byte("{not valid json"), 0o644); err != nil {
		t.Fatal(err)
	}

	entries := []changelogEntry{{StashSceneID: "new", Filename: "c.mp4"}}
	if err := appendChangelog(dir, entries); err != nil {
		t.Fatalf("appendChangelog: %v", err)
	}

	got := readChangelog(t, dir)
	if len(got) != 1 || got[0].StashSceneID != "new" {
		t.Errorf("expected only the new entry after corrupt backup, got %+v", got)
	}

	matches, err := filepath.Glob(filepath.Join(dir, "fss-stashbox-changelog.corrupt-*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected exactly one .corrupt-*.json backup, got %v", matches)
	}

	backup, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(backup), "{not valid json") {
		t.Errorf("backup did not preserve original corrupt content: %s", backup)
	}
}

func TestAppendChangelog_emptyFileTreatedAsCorrupt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fss-stashbox-changelog.json")

	if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	entries := []changelogEntry{{StashSceneID: "x"}}
	if err := appendChangelog(dir, entries); err != nil {
		t.Fatalf("appendChangelog: %v", err)
	}

	got := readChangelog(t, dir)
	if len(got) != 1 {
		t.Errorf("got %+v", got)
	}

	matches, _ := filepath.Glob(filepath.Join(dir, "fss-stashbox-changelog.corrupt-*.json"))
	if len(matches) != 1 {
		t.Errorf("expected backup of empty/corrupt file, got %v", matches)
	}
}

func readChangelog(t *testing.T, dir string) []changelogEntry {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "fss-stashbox-changelog.json"))
	if err != nil {
		t.Fatalf("reading changelog: %v", err)
	}
	var entries []changelogEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		t.Fatalf("unmarshalling changelog: %v\n%s", err, data)
	}
	return entries
}

func writeChangelog(t *testing.T, path string, entries []changelogEntry) {
	t.Helper()
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	defer func() { os.Stderr = orig }()

	fn()

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestPrintFailureSummary_emptyIsNoOp(t *testing.T) {
	out := captureStderr(t, func() {
		printFailureSummary(nil)
		printFailureSummary([]importFailure{})
	})
	if out != "" {
		t.Errorf("expected no output for empty failures, got: %q", out)
	}
}

func TestPrintFailureSummary_groupsByScene(t *testing.T) {
	failures := []importFailure{
		{SceneID: "10", Filename: "a.mp4", Op: "tag", Name: "POV", Err: errors.New("network blip")},
		{SceneID: "10", Filename: "a.mp4", Op: "performer", Name: "Alice", Err: errors.New("alias collision")},
		{SceneID: "20", Filename: "b.mp4", Op: "studio", Name: "SomeStudio", Err: errors.New("not found")},
		{SceneID: "30", Filename: "c.mp4", Op: "update", Err: errors.New("timeout")},
	}

	out := captureStderr(t, func() { printFailureSummary(failures) })

	// Header reflects 4 ops across 3 scenes.
	if !strings.Contains(out, "Failures (4 operations across 3 scenes)") {
		t.Errorf("missing or wrong header: %s", out)
	}
	// Each scene header appears once.
	for _, want := range []string{"scene 10 (a.mp4)", "scene 20 (b.mp4)", "scene 30 (c.mp4)"} {
		if strings.Count(out, want) != 1 {
			t.Errorf("expected exactly one occurrence of %q, got: %s", want, out)
		}
	}
	// Named ops are quoted.
	if !strings.Contains(out, `tag "POV": network blip`) {
		t.Errorf("missing tag failure detail: %s", out)
	}
	if !strings.Contains(out, `performer "Alice": alias collision`) {
		t.Errorf("missing performer failure detail: %s", out)
	}
	// Update op has no Name and should not be quoted.
	if !strings.Contains(out, "- update: timeout") {
		t.Errorf("update failure should appear without quoted name: %s", out)
	}
	// Scene 10's two failures are nested under the same header (no second 'scene 10' line).
	if strings.Count(out, "scene 10") != 1 {
		t.Errorf("scene 10 should be grouped, got duplicated header: %s", out)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	fn()

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestPrintWouldCreateSummary_emptyIsNoOp(t *testing.T) {
	l := &entityLookup{
		tags:       map[string]bool{},
		performers: map[string]bool{},
		studios:    map[string]bool{},
	}
	out := captureStdout(t, func() { printWouldCreateSummary(l) })
	if out != "" {
		t.Errorf("expected no output, got: %q", out)
	}
}

func TestPrintWouldCreateSummary_skipsExistingShowsMissing(t *testing.T) {
	l := &entityLookup{
		tags: map[string]bool{
			"POV":               true,  // exists, should not appear
			"Female Domination": false, // would create
			"4K Available":      false, // would create
		},
		performers: map[string]bool{
			"Alice": false,
			"Bob":   true,
		},
		studios: map[string]bool{
			"NewStudio":      false,
			"ExistingStudio": true,
		},
	}
	out := captureStdout(t, func() { printWouldCreateSummary(l) })

	// Sections present.
	if !strings.Contains(out, "Would create on apply:") {
		t.Errorf("missing header: %s", out)
	}

	// Sorted alphabetically — "4K Available" comes before "Female Domination".
	idx4K := strings.Index(out, "4K Available")
	idxFD := strings.Index(out, "Female Domination")
	if idx4K == -1 || idxFD == -1 || idx4K > idxFD {
		t.Errorf("tags should be sorted alphabetically: %s", out)
	}

	// Existing entries do not appear.
	for _, banned := range []string{"POV", "Bob", "ExistingStudio"} {
		if strings.Contains(out, banned) {
			t.Errorf("existing entity %q should not appear: %s", banned, out)
		}
	}

	// Each type prefixed correctly.
	if !strings.Contains(out, `+ tag       "4K Available"`) {
		t.Errorf("missing tag line: %s", out)
	}
	if !strings.Contains(out, `+ performer "Alice"`) {
		t.Errorf("missing performer line: %s", out)
	}
	if !strings.Contains(out, `+ studio    "NewStudio"`) {
		t.Errorf("missing studio line: %s", out)
	}
}

func TestResolveCoverEnabled(t *testing.T) {
	cases := []struct {
		name          string
		flag          bool
		allowedFields map[string]bool
		want          bool
	}{
		{"flag set, no fields filter", true, nil, true},
		{"flag set, fields excludes cover", true, map[string]bool{"title": true}, true},
		{"flag set, fields includes cover", true, map[string]bool{"cover": true}, true},
		{"flag unset, no fields filter (legacy default)", false, nil, false},
		{"flag unset, fields excludes cover", false, map[string]bool{"title": true}, false},
		{"flag unset, fields includes cover (implicit enable)", false, map[string]bool{"cover": true}, true},
		{"flag unset, fields includes cover plus others", false, map[string]bool{"cover": true, "tags": true}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := resolveCoverEnabled(c.flag, c.allowedFields); got != c.want {
				t.Errorf("got %v, want %v", got, c.want)
			}
		})
	}
}

func TestPrintWouldCreateSummary_onlyExistingIsNoOp(t *testing.T) {
	l := &entityLookup{
		tags:       map[string]bool{"POV": true, "MILF": true},
		performers: map[string]bool{"Alice": true},
		studios:    map[string]bool{"S": true},
	}
	out := captureStdout(t, func() { printWouldCreateSummary(l) })
	if out != "" {
		t.Errorf("expected no output when nothing would be created, got: %q", out)
	}
}

func TestPrintFailureSummary_preservesInsertionOrder(t *testing.T) {
	failures := []importFailure{
		{SceneID: "C", Filename: "c.mp4", Op: "tag", Name: "x", Err: errors.New("e1")},
		{SceneID: "A", Filename: "a.mp4", Op: "tag", Name: "x", Err: errors.New("e2")},
		{SceneID: "B", Filename: "b.mp4", Op: "tag", Name: "x", Err: errors.New("e3")},
	}
	out := captureStderr(t, func() { printFailureSummary(failures) })

	idxC := strings.Index(out, "scene C")
	idxA := strings.Index(out, "scene A")
	idxB := strings.Index(out, "scene B")
	if idxC >= idxA || idxA >= idxB {
		t.Errorf("expected insertion order C → A → B, got positions C=%d A=%d B=%d\noutput: %s", idxC, idxA, idxB, out)
	}
}

func TestDiffStrings(t *testing.T) {
	got := diffStrings([]string{"a", "b", "c"}, []string{"b", "c", "d", "e"})
	want := []string{"d", "e"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestDiffStrings_allNew(t *testing.T) {
	got := diffStrings([]string{"a"}, []string{"b", "c"})
	want := []string{"b", "c"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestDiffStrings_noneNew(t *testing.T) {
	got := diffStrings([]string{"a", "b"}, []string{"a", "b"})
	if len(got) != 0 {
		t.Errorf("got %v, want empty", got)
	}
}

func TestExtractTagIDs(t *testing.T) {
	tags := []stash.StashTag{{ID: "10", Name: "POV"}, {ID: "20", Name: "MILF"}}
	got := extractTagIDs(tags)
	want := []string{"10", "20"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestExtractPerfIDs(t *testing.T) {
	perfs := []stash.StashPerf{{ID: "1", Name: "Alice"}, {ID: "2", Name: "Bob"}}
	got := extractPerfIDs(perfs)
	want := []string{"1", "2"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestDedup(t *testing.T) {
	got := dedup([]string{"a", "b", "a", "c", "b"})
	want := []string{"a", "b", "c"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestDedup_noDups(t *testing.T) {
	got := dedup([]string{"x", "y", "z"})
	want := []string{"x", "y", "z"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestTruncate_short(t *testing.T) {
	got := truncate("hello", 10)
	if got != "hello" {
		t.Errorf("got %q, want %q", got, "hello")
	}
}

func TestTruncate_exact(t *testing.T) {
	got := truncate("hello", 5)
	if got != "hello" {
		t.Errorf("got %q, want %q", got, "hello")
	}
}

func TestTruncate_long(t *testing.T) {
	got := truncate("hello world", 8)
	if got != "hello..." {
		t.Errorf("got %q, want %q", got, "hello...")
	}
}

func TestParseFieldsFlag_valid(t *testing.T) {
	got, err := parseFieldsFlag([]string{"title", "tags", "cover"})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"title": true, "tags": true, "cover": true}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParseFieldsFlag_invalid(t *testing.T) {
	_, err := parseFieldsFlag([]string{"title", "bogus"})
	if err == nil {
		t.Fatal("expected error for unknown field")
	}
	if !strings.Contains(err.Error(), "bogus") {
		t.Errorf("error should mention the unknown field, got: %v", err)
	}
}

func TestParseFieldsFlag_empty(t *testing.T) {
	got, err := parseFieldsFlag(nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Errorf("got %v, want nil", got)
	}
}

func TestFieldAllowed_nilAllowsAll(t *testing.T) {
	if !fieldAllowed(nil, "anything") {
		t.Error("nil map should allow all fields")
	}
}

func TestFieldAllowed_presentField(t *testing.T) {
	m := map[string]bool{"title": true}
	if !fieldAllowed(m, "title") {
		t.Error("present field should be allowed")
	}
}

func TestFieldAllowed_absentField(t *testing.T) {
	m := map[string]bool{"title": true}
	if fieldAllowed(m, "tags") {
		t.Error("absent field should not be allowed")
	}
}

func TestBuildChanges_titleChange(t *testing.T) {
	ss := stash.StashScene{ID: "1", Title: "Old Title"}
	merged := match.MergedScene{Title: "New Title"}
	changes := buildChanges(ss, merged, nil, nil, false)
	diff, ok := changes["title"]
	if !ok {
		t.Fatal("expected title change")
	}
	if diff.From != "Old Title" || diff.To != "New Title" {
		t.Errorf("got from=%v to=%v", diff.From, diff.To)
	}
}

func TestBuildChanges_noChanges(t *testing.T) {
	ss := stash.StashScene{ID: "1", Title: "Same", Details: "Desc", Date: "2026-01-01"}
	merged := match.MergedScene{
		Title:       "Same",
		Description: "Desc",
		Date:        time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	changes := buildChanges(ss, merged, nil, nil, false)
	if len(changes) != 0 {
		t.Errorf("expected no changes, got %v", changes)
	}
}

func TestBuildChanges_coverEnabled(t *testing.T) {
	ss := stash.StashScene{ID: "1"}
	merged := match.MergedScene{Thumbnail: "https://example.com/thumb.jpg"}
	changes := buildChanges(ss, merged, nil, nil, true)
	diff, ok := changes["cover"]
	if !ok {
		t.Fatal("expected cover change")
	}
	if diff.To == nil || diff.To == "" {
		t.Error("cover To should be set")
	}
}

func TestBuildChanges_addedTags(t *testing.T) {
	ss := stash.StashScene{
		ID:   "1",
		Tags: []stash.StashTag{{ID: "10", Name: "POV"}},
	}
	merged := match.MergedScene{}
	changes := buildChanges(ss, merged, nil, []string{"POV", "MILF", "Threesome"}, false)
	diff, ok := changes["tags"]
	if !ok {
		t.Fatal("expected tags change")
	}
	want := []string{"MILF", "Threesome"}
	if !reflect.DeepEqual(diff.Added, want) {
		t.Errorf("got added=%v, want %v", diff.Added, want)
	}
}
