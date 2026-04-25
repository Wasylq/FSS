package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
