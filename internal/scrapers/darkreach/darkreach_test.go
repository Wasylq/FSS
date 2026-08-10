package darkreach

import (
	"testing"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
)

// darkreach is a config-only wrapper — 48 site entries across four util
// families, registered by four loops in init(). There is no parsing logic to
// unit-test, but a table that size has its own failure mode; see
// testutil.CheckSiteTable for what is checked and why.
func TestSiteTable(t *testing.T) {
	var rows []testutil.SiteRow
	for _, c := range modernSites {
		rows = append(rows, testutil.SiteRow{Table: "modernSites", ID: c.ID, Base: c.SiteBase, Studio: c.Studio, Patterns: c.Patterns, MatchRe: c.MatchRe})
	}
	for _, c := range updateItemSites {
		rows = append(rows, testutil.SiteRow{Table: "updateItemSites", ID: c.ID, Base: c.SiteBase, Studio: c.Studio, Patterns: c.Patterns, MatchRe: c.MatchRe})
	}
	for _, c := range updatesMarketingSites {
		rows = append(rows, testutil.SiteRow{Table: "updatesMarketingSites", ID: c.ID, Base: c.SiteBase, Studio: c.Studio, Patterns: c.Patterns, MatchRe: c.MatchRe})
	}
	for _, c := range classicSites {
		rows = append(rows, testutil.SiteRow{Table: "classicSites", ID: c.ID, Base: c.SiteBase, Studio: c.Studio, Patterns: c.Patterns, MatchRe: c.MatchRe})
	}
	testutil.CheckSiteTable(t, rows)
	// One row per domain here, so the domain-consistency checks apply.
	testutil.CheckSiteTableDomains(t, rows)
}
