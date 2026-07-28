package bronetwork

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

// bronetwork is a network-style table: three rows share the network's SiteBase
// (thebronetwork.com) and carry MatchRe for their own vanity domains, so only
// the universal checks apply — CheckSiteTableDomains would flag that shared base
// as a copy-paste slip, which it is not.
func TestSiteTableIntegrity(t *testing.T) {
	rows := make([]testutil.SiteRow, 0, len(sites))
	for _, c := range sites {
		rows = append(rows, testutil.SiteRow{
			ID: c.ID, Base: c.SiteBase, Studio: c.Studio,
			Patterns: c.Patterns, MatchRe: c.MatchRe,
		})
	}
	testutil.CheckSiteTable(t, rows)
}
