package scraper

import (
	"net/url"
	"strings"
)

// HostMatches reports whether rawURL's host is one of hosts, ignoring a leading
// "www." on either side.
//
// It compares parsed hosts rather than searching the raw URL text, so a
// look-alike domain that merely contains one of hosts does not match. See
// CONTRIBUTING.md § "Host regexes".
func HostMatches(rawURL string, hosts ...string) bool {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || u.Host == "" {
		return false
	}
	got := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	if got == "" {
		return false
	}
	for _, h := range hosts {
		want := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(h)), "www.")
		if want != "" && got == want {
			return true
		}
	}
	return false
}
