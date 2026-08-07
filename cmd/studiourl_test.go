package cmd

import (
	"testing"

	"github.com/Wasylq/FSS/internal/store"
	"github.com/Wasylq/FSS/models"
)

func TestSameStudioKey(t *testing.T) {
	same := []string{
		"http://www.example.com",
		"https://www.example.com",
		"https://www.example.com/",
		"https://WWW.Example.com/",
		"http://WWW.EXAMPLE.COM",
	}
	want := sameStudioKey(same[0])
	for _, u := range same[1:] {
		if got := sameStudioKey(u); got != want {
			t.Errorf("sameStudioKey(%q) = %q, want %q", u, got, want)
		}
	}

	// Paths are meaningful and case-sensitive; different studios stay different.
	different := [][2]string{
		{"https://example.com/studio/1", "https://example.com/studio/2"},
		{"https://example.com/Studio", "https://example.com/studio"},
		{"https://a.example.com", "https://b.example.com"},
	}
	for _, pair := range different {
		if sameStudioKey(pair[0]) == sameStudioKey(pair[1]) {
			t.Errorf("%q and %q collapsed to the same key", pair[0], pair[1])
		}
	}
}

// The warning fires only for a cosmetic variant of an already-tracked URL —
// which is exactly the case that silently forks one catalogue into two studios.
func TestWarnStudioURLVariant(t *testing.T) {
	db := newImportDB(t)
	stored := "http://www.example.com"
	if err := db.UpsertStudio(models.Studio{URL: stored, SiteID: "x"}); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		url  string
		warn bool
	}{
		{"https://www.example.com/", true},   // scheme + trailing slash
		{"https://WWW.Example.com", true},    // host casing
		{stored, false},                      // the stored spelling itself
		{"https://other.example.com", false}, // a genuinely different studio
	}
	for _, c := range cases {
		out := captureStderr(t, func() { warnStudioURLVariant(db, c.url) })
		if got := out != ""; got != c.warn {
			t.Errorf("%s: warned=%v, want %v (%q)", c.url, got, c.warn, out)
		}
	}
}

// The flat store tracks no studios, so there is nothing to compare and nothing
// to say.
func TestWarnStudioURLVariantSilentOnFlatStore(t *testing.T) {
	flat := store.NewFlat(t.TempDir(), []string{"json"})
	if out := captureStderr(t, func() { warnStudioURLVariant(flat, "https://x.example.com/") }); out != "" {
		t.Errorf("flat store produced a warning: %q", out)
	}
}
