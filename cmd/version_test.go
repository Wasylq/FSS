package cmd

import (
	"strings"
	"testing"
)

// The v-prefix handling is the reason this is worth pinning: the tags carry one
// and buildVersion may or may not, so a comparison on the raw strings reports an
// update on every run for a user who is already current.
func TestUpdateLines(t *testing.T) {
	cases := []struct {
		name    string
		current string
		latest  string
		want    string // substring the first line must contain
		wantURL bool
	}{
		{"same version, both prefixed", "v1.2.3", "v1.2.3", "latest version", false},
		{"same version, current unprefixed", "1.2.3", "v1.2.3", "latest version", false},
		{"same version, tag unprefixed", "v1.2.3", "1.2.3", "latest version", false},
		{"newer release available", "v1.2.3", "v1.3.0", "Update available: v1.2.3 → v1.3.0", true},
		{"dev build", "dev", "v1.3.0", "running dev build", false},
		{"dev build, prefixed", "vdev", "v1.3.0", "running dev build", false},

		// Not "up to date" and not a bogus update — see updateLines.
		{"empty tag", "v1.2.3", "", "Could not determine", false},
		{"whitespace tag", "v1.2.3", "   ", "Could not determine", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := updateLines(c.current, c.latest)
			if len(got) == 0 {
				t.Fatal("no output lines")
			}
			if !strings.Contains(got[0], c.want) {
				t.Errorf("first line = %q, want it to contain %q", got[0], c.want)
			}
			hasURL := false
			for _, l := range got {
				if strings.Contains(l, "releases/latest") {
					hasURL = true
				}
			}
			if hasURL != c.wantURL {
				t.Errorf("release URL present = %v, want %v (lines: %q)", hasURL, c.wantURL, got)
			}
		})
	}
}

// A user who is up to date must not be told to go somewhere, and a user who is
// behind must be given the link. Stated separately from the table because it is
// the property that matters rather than the wording.
func TestUpdateLinesOnlyLinksWhenBehind(t *testing.T) {
	if lines := updateLines("v2.0.0", "v2.0.0"); len(lines) != 1 {
		t.Errorf("an up-to-date build printed %d lines, want 1: %q", len(lines), lines)
	}
	if lines := updateLines("v1.0.0", "v2.0.0"); len(lines) != 2 {
		t.Errorf("a stale build printed %d lines, want 2 (notice + link): %q", len(lines), lines)
	}
}
