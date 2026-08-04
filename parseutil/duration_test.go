package parseutil

import (
	"strings"
	"testing"
)

func TestParseDurationColon(t *testing.T) {
	tests := []struct {
		in   string
		want int
	}{
		{"30:00", 1800},
		{"1:30", 90},
		{"00:28:51", 1731},
		{"01:00:00", 3600},
		{"1:02:03", 3723},
		{"45", 45},
		{"", 0},
		{"  30:00  ", 1800},
		{" 10 : 20 ", 620},
	}
	for _, tt := range tests {
		if got := ParseDurationColon(tt.in); got != tt.want {
			t.Errorf("ParseDurationColon(%q) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestParseDurationISO(t *testing.T) {
	tests := []struct {
		in   string
		want int
	}{
		{"PT1H2M3S", 3723},
		{"PT30M", 1800},
		{"PT45S", 45},
		{"PT1H", 3600},
		{"PT1H30S", 3630},
		{"PT2M30S", 150},
		{"", 0},
		{"null", 0},
		{"  PT10M5S  ", 605},
	}
	for _, tt := range tests {
		if got := ParseDurationISO(tt.in); got != tt.want {
			t.Errorf("ParseDurationISO(%q) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

// L5 / H-dates: ISO durations arrive in mixed case from different feeds, and the
// literals must be case-insensitive.
//
// The failure this guards was not "hours are dropped" but something sharper: because
// the prefix was trimmed before the string was uppercased, "pt45s" failed the TrimPrefix,
// became "PT45S", and the seconds parse then read "PT45" and returned **0** — the entire
// duration lost. Single-component lowercase durations were the worst case; a zero
// Duration is not validated anywhere downstream, so it stored silently.
func TestParseDurationISOIsCaseInsensitive(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"PT1H2M3S", 3723},
		{"pt1h2m3s", 3723},
		{"Pt1H2M3S", 3723},
		{"pT1h2M3s", 3723},

		// Single components: these returned 0 before the fix, not a partial value.
		{"PT2H", 7200},
		{"pt2h", 7200},
		{"PT30M", 1800},
		{"pt30m", 1800},
		{"PT45S", 45},
		{"pt45s", 45},

		// Unchanged behaviour.
		{"", 0},
		{"null", 0},
		{"garbage", 0},
		{"  PT1H  ", 3600},
	}
	for _, c := range cases {
		if got := ParseDurationISO(c.in); got != c.want {
			t.Errorf("ParseDurationISO(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

// Every mixed-case spelling of one duration must agree — stated as a property so a
// future refactor cannot reintroduce a case-sensitive path for some component only.
func TestParseDurationISOCaseVariantsAgree(t *testing.T) {
	for _, base := range []string{"PT1H", "PT1M", "PT1S", "PT1H30M", "PT1H30M15S"} {
		want := ParseDurationISO(base)
		if want == 0 {
			t.Fatalf("%q parsed as 0; the baseline itself is broken", base)
		}
		for _, variant := range []string{
			strings.ToLower(base),
			strings.ToUpper(base),
			"pT" + strings.ToLower(base[2:]),
		} {
			if got := ParseDurationISO(variant); got != want {
				t.Errorf("ParseDurationISO(%q) = %d, want %d (same duration as %q)",
					variant, got, want, base)
			}
		}
	}
}
