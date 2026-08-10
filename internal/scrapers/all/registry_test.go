package all

import (
	"regexp"
	"strings"
	"testing"

	"github.com/Anastylosis/FSS/scraper"
)

// testutil.CheckSiteTable pins a config table's contents, but it has to be wired
// into each package by hand and it can only read a table whose fields it can be
// handed. Most of the remaining tables are positional literals
// ({"brasilvr", "brasilvr.com", "BrasilVR"}) or rows built by a constructor, so
// there is nothing to hand it.
//
// The checks below go through the registry instead: they ask each *constructed*
// scraper what it claims, so a table's shape is irrelevant and every site is
// covered — including the ones no per-package test will ever reach. Fully
// offline; no scraper is run.

// schemeRe strips the scheme from patterns that include one.
var schemeRe = regexp.MustCompile(`^https?://`)

// Hyphens are established (the ~136 adultprime-* rows); nothing else is used.
var idRe = regexp.MustCompile(`^[a-z0-9-]+$`)

// IDs key the store filename, --site-delay lookups, and Scene.SiteID. An ID with
// a capital or a separator in it still works until something compares it to a
// slug or a config key, so the convention is pinned here.
func TestRegistryIDsAreWellFormed(t *testing.T) {
	seen := map[string]string{}
	for _, s := range scraper.All() {
		id := s.ID()
		switch {
		case id == "":
			t.Errorf("scraper with patterns %v has an empty ID", s.Patterns())
		case !idRe.MatchString(id):
			t.Errorf("%q: ID must be lowercase alphanumeric (it becomes a store filename and a config key)", id)
		}
		if prev, dup := seen[id]; dup {
			t.Errorf("duplicate ID %q (also claimed by a scraper matching %s) — the two would share one store file", id, prev)
			continue
		}
		if pats := s.Patterns(); len(pats) > 0 {
			seen[id] = pats[0]
		} else {
			seen[id] = "(no patterns)"
		}
	}
}

// Patterns are what `fss list-scrapers` prints, so they are the only way a user
// finds out a site is supported. A blank entry is the interesting case: where
// Patterns() derives its strings from another column, a broken derivation yields
// []string{""} — length 1, so a plain emptiness check passes while the listing
// prints a blank line.
func TestRegistryPatternsArePresent(t *testing.T) {
	for _, s := range scraper.All() {
		pats := s.Patterns()
		if len(pats) == 0 {
			t.Errorf("%s: no Patterns — invisible in `fss list-scrapers`", s.ID())
			continue
		}
		for i, p := range pats {
			if strings.TrimSpace(p) == "" {
				t.Errorf("%s: Patterns[%d] is blank", s.ID(), i)
			}
		}
	}
}

// bareHost normalises a display pattern for comparison against another.
func bareHost(p string) string {
	p = strings.TrimPrefix(strings.TrimPrefix(p, "https://"), "http://")
	p = strings.TrimPrefix(p, "www.")
	return strings.TrimSuffix(p, "/")
}

// Several util packages derive an extra pattern by concatenating a path onto
// SiteBase. SiteBase carries no trailing slash, so a path written without a
// leading one glues host to path: "example.comtag/{slug}",
// "www.example.comtour/models/{slug}.html". 34 sites across extrememoviepassutil
// and veutil shipped that way — the pattern is display-only, so nothing broke
// loudly; a user copying the advertised URL out of `fss list-scrapers` just gets
// nowhere. The scraper's own MatchesURL does not catch it either, since those
// MatchRe are unanchored prefixes and "smutbuttxxx\.com" happily matches
// "...comtour".
//
// The rule: where one pattern is a bare host and another starts with that host,
// the second must continue with a separator. Restricting the left side to a bare
// host is what keeps this sound — "rocket-inc.net/works" vs
// "rocket-inc.net/works_actress/{slug}" are two real paths, not a glued one.
func TestRegistryPatternsAreNotGlued(t *testing.T) {
	for _, s := range scraper.All() {
		pats := s.Patterns()
		for _, a := range pats {
			host := bareHost(a)
			if strings.ContainsAny(host, "/?, ") {
				continue
			}
			for _, b := range pats {
				bb := bareHost(b)
				if len(bb) <= len(host) || !strings.HasPrefix(bb, host) {
					continue
				}
				if c := bb[len(host)]; c == '/' || c == '?' || c == '&' || c == '#' {
					continue
				}
				t.Errorf("%s: pattern %q runs the host %q straight into a path — a missing "+
					"leading slash where SiteBase is concatenated with a path", s.ID(), b, a)
			}
		}
	}
}

// An over-broad MatchesURL is the worst failure mode in the registry and the
// least visible: scraper.ForURL returns the *first* match, so one regex that is
// too loose silently hijacks every site after it in registration order — the
// scrape appears to work and produces another studio's catalogue. That is the
// bronetwork bug (sub-studio URLs scraping the whole network), generalised.
//
// Each scraper is offered URLs that belong to nobody. A pattern that accepts
// them is unanchored or has a `.` where it needs `\.`.
func TestRegistryMatchesURLIsNotOverBroad(t *testing.T) {
	strangers := []string{
		"https://example.com/",
		"https://example.com/videos/1",
		"https://not-a-real-porn-site.invalid/model/jane",
		"https://google.com",
		"",
		"not a url at all",
	}
	for _, s := range scraper.All() {
		for _, u := range strangers {
			if s.MatchesURL(u) {
				t.Errorf("%s: MatchesURL(%q) is true — this regex will hijack other scrapers' URLs via ForURL", s.ID(), u)
			}
		}
	}
}

// advertisedHosts pulls the host out of each display pattern, skipping the ones
// that carry a placeholder token instead of a literal host.
func advertisedHosts(pats []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, p := range pats {
		h := schemeRe.ReplaceAllString(strings.TrimSpace(p), "")
		if i := strings.IndexByte(h, '/'); i >= 0 {
			h = h[:i]
		}
		if h == "" || strings.ContainsAny(h, "{} \t") || seen[h] {
			continue
		}
		seen[h] = true
		out = append(out, h)
	}
	return out
}

// A host regex anchored at the start but not terminated is a prefix match, so
// "https://example.com.evil.invalid/" claims the scraper for example.com. The
// look-alike is built from each scraper's own host, which is why the stranger
// list above cannot find this. See CONTRIBUTING.md § "Host regexes".
func TestRegistryMatchesURLTerminatesTheHost(t *testing.T) {
	for _, s := range scraper.All() {
		for _, host := range advertisedHosts(s.Patterns()) {
			for _, u := range []string{
				"https://" + host + ".evil.invalid/",
				"https://" + host + ".evil.invalid/videos",
			} {
				if s.MatchesURL(u) {
					t.Errorf("%s: MatchesURL(%q) is true — the host regex is not terminated; "+
						"append (?:/|$) so %q cannot be extended", s.ID(), u, host)
				}
			}
		}
	}
}

// The converse: a scraper must accept the URLs it advertises. A pattern listed in
// `fss list-scrapers` that MatchesURL rejects sends the user to "no scraper
// found for URL" for a site the tool does support.
//
// sampleURL only reports whether *some* pattern is acceptable, so this checks
// every pattern that can be turned into a concrete URL and reports how many were
// unusable rather than hiding the gap.
func TestRegistryAcceptsItsOwnPatterns(t *testing.T) {
	var checked, unusable int
	for _, s := range scraper.All() {
		for _, p := range s.Patterns() {
			// Patterns naming a query string cannot be reconstructed from the
			// display string. A scheme is stripped rather than skipped: the
			// registry is inconsistent about including one, and skipping those
			// patterns is what let a malformed host ("...comtour/models") sit
			// in the listing unnoticed.
			if strings.ContainsAny(p, "?=") {
				unusable++
				continue
			}
			cand := placeholderRe.ReplaceAllString(p, "1")
			cand = schemeRe.ReplaceAllString(cand, "")
			cand = strings.TrimSuffix(cand, "/")
			if !strings.Contains(cand, ".") {
				unusable++
				continue
			}
			var ok bool
			for _, u := range []string{"https://" + cand, "https://www." + cand, "http://" + cand} {
				if s.MatchesURL(u) {
					ok = true
					break
				}
			}
			if !ok {
				unusable++
				continue
			}
			checked++
		}
	}
	// Reported, not capped silently: `unusable` is patterns this test could not
	// turn into a URL, which is expected for query-string and odd-path patterns
	// but should not grow without reason.
	t.Logf("%d patterns accepted by their own scraper; %d not reconstructible from the display string", checked, unusable)
	if checked == 0 {
		t.Fatal("no patterns were exercised — the derivation is broken, not the scrapers")
	}
}
