package adultdoorway

import (
	"testing"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
)

// Config-only wrapper: parsing tests live in the *util it delegates to. What is
// not covered there is this table's own integrity — see testutil.CheckSiteTable.
func TestSiteTable(t *testing.T) {
	rows := make([]testutil.SiteRow, 0, len(sites))
	for _, c := range sites {
		rows = append(rows, testutil.SiteRow{
			ID: c.ID, Base: c.SiteBase, Studio: c.Studio,
			Patterns: c.Patterns, MatchRe: c.MatchRe,
		})
	}
	testutil.CheckSiteTable(t, rows)
	// One row per domain here, so the domain-consistency checks apply.
	testutil.CheckSiteTableDomains(t, rows)
}
