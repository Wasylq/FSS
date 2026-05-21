package parseutil

import (
	"strconv"
	"strings"
)

// ParseDurationColon parses colon-separated durations like "MM:SS" or
// "HH:MM:SS" and returns the total seconds. Handles any number of
// colon-separated parts. Returns 0 for empty or unparseable input.
func ParseDurationColon(s string) int {
	parts := strings.Split(strings.TrimSpace(s), ":")
	total := 0
	for _, p := range parts {
		n, _ := strconv.Atoi(strings.TrimSpace(p))
		total = total*60 + n
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
		if n, err := strconv.Atoi(s[:i]); err == nil {
			total += n * 3600
		}
		s = s[i+1:]
	}
	if i := strings.Index(s, "M"); i >= 0 {
		if n, err := strconv.Atoi(s[:i]); err == nil {
			total += n * 60
		}
		s = s[i+1:]
	}
	if i := strings.Index(s, "S"); i >= 0 {
		if n, err := strconv.Atoi(s[:i]); err == nil {
			total += n
		}
	}
	return total
}
