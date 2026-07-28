package testutil

import (
	"net/url"
	"regexp"
	"strings"
	"testing"
)

// SiteRow is one entry of a table-driven scraper's site list, flattened so the
// integrity checks can be shared. The concrete SiteConfig types differ per
// *util package, so each caller maps its own rows into this shape.
type SiteRow struct {
	// Table names the source slice (e.g. "modernSites"), for error messages
	// when a package has several.
	Table    string
	ID       string
	Base     string
	Studio   string
	Patterns []string
	MatchRe  *regexp.Regexp
}

// CheckSiteTable runs integrity checks over a table-driven scraper's site list.
//
// Config-only wrappers are exempt from unit tests by convention — the *util
// they delegate to carries the parsing tests — but a table of dozens of rows
// has a failure mode of its own that no other test covers: a copy-pasted entry
// whose MatchRe still names the previous site's domain. That scraper then
// claims another site's URLs and never matches its own. The offline suite does
// not exercise these sites, and a live smoke test only proves the one URL it
// was handed, so nothing else would notice.
//
// CheckSiteTable holds for every table: every field populated, IDs unique, Base
// absolute with no trailing slash.
//
// The domain-consistency checks are deliberately NOT here — see
// CheckSiteTableDomains, which does not apply to every table.
func CheckSiteTable(t *testing.T, rows []SiteRow) {
	t.Helper()

	if len(rows) == 0 {
		t.Fatal("site table is empty")
	}
	checkComplete(t, rows)
	checkUniqueIDs(t, rows)
	checkBases(t, rows)
}

// CheckSiteTableDomains adds the checks that catch a copy-pasted row: each
// MatchRe must match its own Base and no other row's, and each pattern must
// name a host that row's MatchRe claims.
//
// Only valid for tables where one row is one domain. It does NOT apply to
// network-style tables where several sub-studios share the network's Base and
// each carries a MatchRe for its own vanity domain — bronetwork has three rows
// on https://thebronetwork.com with masqulin.com / menofmontreal.com regexes,
// which is intentional, not a copy-paste slip. Call CheckSiteTable alone there.
func CheckSiteTableDomains(t *testing.T, rows []SiteRow) {
	t.Helper()
	checkMatchRe(t, rows)
	checkPatterns(t, rows)
}

func (r SiteRow) label() string {
	if r.Table == "" {
		return r.ID
	}
	return r.Table + "/" + r.ID
}

func checkComplete(t *testing.T, rows []SiteRow) {
	t.Helper()
	for _, r := range rows {
		if r.ID == "" {
			t.Errorf("%s: entry with empty ID (base %q)", r.Table, r.Base)
			continue
		}
		if r.Base == "" {
			t.Errorf("%s: empty Base", r.label())
		}
		if r.Studio == "" {
			t.Errorf("%s: empty Studio", r.label())
		}
		if len(r.Patterns) == 0 {
			t.Errorf("%s: no Patterns — it would be invisible in `fss list-scrapers`", r.label())
		}
		if r.MatchRe == nil {
			t.Errorf("%s: nil MatchRe", r.label())
		}
	}
}

// IDs key --site-delay and the store, and scraper.Register panics on a
// duplicate — caught here with a readable message rather than as an init()
// panic in an unrelated package's test binary.
func checkUniqueIDs(t *testing.T, rows []SiteRow) {
	t.Helper()
	seen := map[string]string{}
	for _, r := range rows {
		if r.ID == "" {
			continue
		}
		if prev, ok := seen[r.ID]; ok {
			t.Errorf("duplicate site ID %q in %s and %s", r.ID, prev, r.Table)
			continue
		}
		seen[r.ID] = r.Table
	}
}

func checkMatchRe(t *testing.T, rows []SiteRow) {
	t.Helper()
	for _, r := range rows {
		if r.MatchRe == nil || r.Base == "" {
			continue // reported by checkComplete
		}
		if !r.MatchRe.MatchString(r.Base) {
			t.Errorf("%s: MatchRe %q does not match its own Base %q", r.label(), r.MatchRe, r.Base)
		}
		for _, other := range rows {
			if other.ID == r.ID || other.Base == "" {
				continue
			}
			if r.MatchRe.MatchString(other.Base) {
				t.Errorf("%s: MatchRe %q also matches %s's base %q",
					r.label(), r.MatchRe, other.ID, other.Base)
			}
		}
	}
}

// Patterns are checked against MatchRe rather than Base: an entry may serve
// several domains legitimately (a rebrand keeping both live), and the invariant
// that matters is that a pattern advertised by `fss list-scrapers` names a URL
// the scraper would actually accept.
func checkPatterns(t *testing.T, rows []SiteRow) {
	t.Helper()
	for _, r := range rows {
		if r.MatchRe == nil {
			continue
		}
		for _, p := range r.Patterns {
			host := p
			if i := strings.IndexByte(host, '/'); i >= 0 {
				host = host[:i]
			}
			if host == "" {
				t.Errorf("%s: empty pattern", r.label())
				continue
			}
			if !r.MatchRe.MatchString("https://" + host) {
				t.Errorf("%s: pattern %q names host %q, which its own MatchRe %q does not claim",
					r.label(), p, host, r.MatchRe)
			}
		}
	}
}

func checkBases(t *testing.T, rows []SiteRow) {
	t.Helper()
	for _, r := range rows {
		if r.Base == "" {
			continue
		}
		u, err := url.Parse(r.Base)
		if err != nil {
			t.Errorf("%s: Base %q does not parse: %v", r.label(), r.Base, err)
			continue
		}
		if u.Scheme == "" || u.Host == "" {
			t.Errorf("%s: Base %q is not absolute", r.label(), r.Base)
		}
		if strings.HasSuffix(r.Base, "/") {
			t.Errorf("%s: Base %q has a trailing slash — paths are appended directly", r.label(), r.Base)
		}
	}
}

// DomainRow is one entry of a table-driven scraper keyed on a bare hostname
// rather than a full base URL and regex. Several packages build both the base
// (`"https://www." + domain`) and the URL match from that one field, so the
// hostname is the whole configuration and the only thing worth checking.
type DomainRow struct {
	Table  string
	ID     string
	Domain string
	Studio string
}

// CheckSiteDomainTable runs integrity checks over a domain-keyed site table.
//
// The failure mode differs from CheckSiteTable's: with no regex to get wrong,
// what a copy-pasted row breaks here is the *hostname* — a duplicate makes two
// scrapers claim the same site, and a hostname carrying a scheme or path yields
// a malformed base like "https://www.https://x.com".
func CheckSiteDomainTable(t *testing.T, rows []DomainRow) {
	t.Helper()

	if len(rows) == 0 {
		t.Fatal("site table is empty")
	}
	seenID := map[string]bool{}
	seenDomain := map[string]string{}
	for _, r := range rows {
		label := r.ID
		if r.Table != "" {
			label = r.Table + "/" + r.ID
		}
		if r.ID == "" {
			t.Errorf("%s: entry with empty ID (domain %q)", r.Table, r.Domain)
			continue
		}
		if seenID[r.ID] {
			t.Errorf("duplicate site ID %q", r.ID)
		}
		seenID[r.ID] = true

		if r.Studio == "" {
			t.Errorf("%s: empty Studio", label)
		}
		switch {
		case r.Domain == "":
			t.Errorf("%s: empty Domain", label)
			continue
		case strings.Contains(r.Domain, "://"):
			t.Errorf("%s: Domain %q includes a scheme — callers prefix it themselves", label, r.Domain)
		case strings.ContainsAny(r.Domain, "/ "):
			t.Errorf("%s: Domain %q is not a bare hostname", label, r.Domain)
		case !strings.Contains(r.Domain, "."):
			t.Errorf("%s: Domain %q has no dot", label, r.Domain)
		case r.Domain != strings.ToLower(r.Domain):
			t.Errorf("%s: Domain %q is not lowercase", label, r.Domain)
		}
		if prev, ok := seenDomain[r.Domain]; ok {
			t.Errorf("%s: Domain %q is already used by %s — both would claim the same site", label, r.Domain, prev)
		}
		seenDomain[r.Domain] = r.ID
	}
}
