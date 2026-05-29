package parseutil

import "regexp"

// ordinalSuffixRe matches an ASCII ordinal suffix following a digit
// run: `1st`, `22nd`, `3RD`, `4Th`. Lower- and upper-case both match.
// Used by date strings like "8th May 2026" or "May 8th, 2026" where
// the suffix isn't part of any time.Parse layout — strip it before
// parsing.
var ordinalSuffixRe = regexp.MustCompile(`(?i)(\d+)(?:st|nd|rd|th)`)

// StripOrdinalSuffix removes English ordinal suffixes (`st`, `nd`, `rd`,
// `th`, case-insensitive) immediately following a digit run. Example:
//
//	"8th May 2026"        → "8 May 2026"
//	"May 8th, 2026"       → "May 8, 2026"
//	"22nd September 2024" → "22 September 2024"
//
// Digits without a suffix are left alone. Use this as a pre-pass before
// `time.Parse` against a layout that uses bare day numbers.
func StripOrdinalSuffix(s string) string {
	return ordinalSuffixRe.ReplaceAllString(s, "$1")
}
