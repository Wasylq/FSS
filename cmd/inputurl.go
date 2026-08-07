package cmd

import (
	"regexp"
	"strings"
)

var schemeRe = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+.\-]*://`)

// normalizeInputURL prefixes https:// when the input names no scheme. Scraper
// MatchesURL regexes are anchored on ^https?://, so a bare host matches none.
// See docs/usage.md § "fss check".
func normalizeInputURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || schemeRe.MatchString(raw) {
		return raw
	}
	return "https://" + raw
}
