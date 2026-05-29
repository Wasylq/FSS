package parseutil

import "testing"

func TestStripOrdinalSuffix(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"1st", "1"},
		{"2nd", "2"},
		{"3rd", "3"},
		{"4th", "4"},
		{"22ND", "22"},
		{"8th May 2026", "8 May 2026"},
		{"May 8th, 2026", "May 8, 2026"},
		{"22nd September 2024", "22 September 2024"},
		// Bare digits left alone.
		{"123", "123"},
		// Empty input.
		{"", ""},
		// Only the suffix-following digits get touched — words containing
		// `st`/`nd`/`rd`/`th` without a leading digit run are safe.
		{"northwest", "northwest"},
		{"Released: 1st Jan 2024 — 3rd edition", "Released: 1 Jan 2024 — 3 edition"},
	}
	for _, c := range cases {
		if got := StripOrdinalSuffix(c.in); got != c.want {
			t.Errorf("StripOrdinalSuffix(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
