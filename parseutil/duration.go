package parseutil

import (
	"strconv"
	"strings"
)

// ParseDurationColon parses colon-separated durations like "MM:SS" or
// "HH:MM:SS" and returns the total seconds. Handles any number of
// colon-separated parts. Returns 0 for empty or unparseable input.
const maxDurationSeconds = 24 * 3600 // 24 hours; anything beyond is garbage input

func ParseDurationColon(s string) int {
	parts := strings.Split(strings.TrimSpace(s), ":")
	if len(parts) > 3 {
		return 0
	}
	total := 0
	for _, p := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil || n < 0 {
			n = 0
		}
		total = total*60 + n
		if total > maxDurationSeconds {
			return 0
		}
	}
	return total
}

// ParseDurationISO parses ISO 8601 durations like "PT1H2M3S" and returns the
// total seconds. Returns 0 for empty or unparseable input.
func ParseDurationISO(s string) int {
	s = strings.TrimSpace(s)
	if s == "" || s == "null" {
		return 0
	}
	s = strings.TrimPrefix(s, "PT")
	s = strings.ToUpper(s)

	var total int
	if i := strings.Index(s, "H"); i >= 0 {
		if n, err := strconv.Atoi(s[:i]); err == nil && n > 0 && n <= 24 {
			total += n * 3600
		}
		s = s[i+1:]
	}
	if i := strings.Index(s, "M"); i >= 0 {
		if n, err := strconv.Atoi(s[:i]); err == nil && n > 0 && n <= 1440 {
			total += n * 60
		}
		s = s[i+1:]
	}
	if i := strings.Index(s, "S"); i >= 0 {
		if n, err := strconv.Atoi(s[:i]); err == nil && n > 0 && n <= maxDurationSeconds {
			total += n
		}
	}
	if total > maxDurationSeconds {
		return 0
	}
	return total
}
