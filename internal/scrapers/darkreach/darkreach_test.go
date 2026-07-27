package darkreach

import (
	"net/url"
	"regexp"
	"strings"
	"testing"
)

// darkreach is a config-only wrapper — 48 site entries across four util
// families, registered by four loops in init(). There is no parsing logic here
// to unit-test, but a table that size has its own failure mode: a copy-pasted
// entry whose MatchRe still names the previous site's domain. That scraper then
// claims URLs belonging to another site and never matches its own, and nothing
// else in the repo would notice — the offline suite does not exercise these
// sites, and a live smoke test only proves the *one* URL it was given works.
//
// The four SiteConfig types are distinct (one per util package) but share these
// field names, so the checks are expressed once over a flattened view.
type siteRow struct {
	table    string
	id       string
	base     string
	studio   string
	patterns []string
	matchRe  *regexp.Regexp
}

func allRows() []siteRow {
	var rows []siteRow
	for _, c := range modernSites {
		rows = append(rows, siteRow{"modernSites", c.ID, c.SiteBase, c.Studio, c.Patterns, c.MatchRe})
	}
	for _, c := range updateItemSites {
		rows = append(rows, siteRow{"updateItemSites", c.ID, c.SiteBase, c.Studio, c.Patterns, c.MatchRe})
	}
	for _, c := range updatesMarketingSites {
		rows = append(rows, siteRow{"updatesMarketingSites", c.ID, c.SiteBase, c.Studio, c.Patterns, c.MatchRe})
	}
	for _, c := range classicSites {
		rows = append(rows, siteRow{"classicSites", c.ID, c.SiteBase, c.Studio, c.Patterns, c.MatchRe})
	}
	return rows
}

func TestSiteConfigsAreComplete(t *testing.T) {
	rows := allRows()
	if len(rows) == 0 {
		t.Fatal("no site configs registered")
	}
	for _, r := range rows {
		if r.id == "" {
			t.Errorf("%s: entry with empty ID (base %q)", r.table, r.base)
			continue
		}
		if r.base == "" {
			t.Errorf("%s/%s: empty SiteBase", r.table, r.id)
		}
		if r.studio == "" {
			t.Errorf("%s/%s: empty Studio", r.table, r.id)
		}
		if len(r.patterns) == 0 {
			t.Errorf("%s/%s: no Patterns — it would be invisible in `fss list-scrapers`", r.table, r.id)
		}
		if r.matchRe == nil {
			t.Errorf("%s/%s: nil MatchRe", r.table, r.id)
		}
	}
}

// IDs are the key `fss scrape --site-delay` and the store use, and
// scraper.Register panics on a duplicate — catch it here with a readable
// message rather than as an init() panic in an unrelated package's test.
func TestSiteIDsAreUnique(t *testing.T) {
	seen := map[string]string{}
	for _, r := range allRows() {
		if prev, ok := seen[r.id]; ok {
			t.Errorf("duplicate site ID %q in %s and %s", r.id, prev, r.table)
			continue
		}
		seen[r.id] = r.table
	}
}

// The copy-paste bug this file exists for: every MatchRe must match its own
// SiteBase, and must not match any other entry's.
func TestMatchReMatchesItsOwnSiteOnly(t *testing.T) {
	rows := allRows()
	for _, r := range rows {
		if r.matchRe == nil || r.base == "" {
			continue // reported by TestSiteConfigsAreComplete
		}
		if !r.matchRe.MatchString(r.base) {
			t.Errorf("%s/%s: MatchRe %q does not match its own SiteBase %q",
				r.table, r.id, r.matchRe, r.base)
		}
		for _, other := range rows {
			if other.id == r.id || other.base == "" {
				continue
			}
			if r.matchRe.MatchString(other.base) {
				t.Errorf("%s/%s: MatchRe %q also matches %s's base %q",
					r.table, r.id, r.matchRe, other.id, other.base)
			}
		}
	}
}

// Patterns are display strings, but a pattern naming a host the entry does not
// actually claim is the same copy-paste slip from another angle: it advertises
// a URL in `fss list-scrapers` that the scraper would refuse.
//
// Checked against MatchRe rather than SiteBase, because an entry may legitimately
// serve several domains — girlskissxxx covers the rebranded iKissGirls and both
// domains are live, which is exactly what an earlier, stricter version of this
// test wrongly flagged.
func TestPatternsNameAHostTheEntryClaims(t *testing.T) {
	for _, r := range allRows() {
		if r.matchRe == nil || len(r.patterns) == 0 {
			continue
		}
		for _, p := range r.patterns {
			host := p
			if i := strings.IndexAny(host, "/"); i >= 0 {
				host = host[:i]
			}
			if host == "" {
				t.Errorf("%s/%s: empty pattern", r.table, r.id)
				continue
			}
			if !r.matchRe.MatchString("https://" + host) {
				t.Errorf("%s/%s: pattern %q names host %q, which its own MatchRe %q does not claim",
					r.table, r.id, p, host, r.matchRe)
			}
		}
	}
}

// SiteBase must be a usable absolute URL — it is concatenated into every
// request the util packages build.
func TestSiteBaseIsAbsoluteURL(t *testing.T) {
	for _, r := range allRows() {
		if r.base == "" {
			continue // reported by TestSiteConfigsAreComplete
		}
		u, err := url.Parse(r.base)
		if err != nil {
			t.Errorf("%s/%s: SiteBase %q does not parse: %v", r.table, r.id, r.base, err)
			continue
		}
		if u.Scheme == "" || u.Host == "" {
			t.Errorf("%s/%s: SiteBase %q is not absolute", r.table, r.id, r.base)
		}
		if strings.HasSuffix(r.base, "/") {
			t.Errorf("%s/%s: SiteBase %q has a trailing slash — paths are appended directly",
				r.table, r.id, r.base)
		}
	}
}
