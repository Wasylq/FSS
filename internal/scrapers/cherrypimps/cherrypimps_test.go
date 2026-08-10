package cherrypimps

import (
	"testing"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
)

func TestSiteCount(t *testing.T) {
	if len(sites) != 2 {
		t.Errorf("got %d sites, want 2", len(sites))
	}
}

func TestUniqueSiteIDs(t *testing.T) {
	seen := make(map[string]bool)
	for _, s := range sites {
		if seen[s.ID] {
			t.Errorf("duplicate site ID: %s", s.ID)
		}
		seen[s.ID] = true
	}
}

// Config-only wrapper: parsing tests live in the *util it delegates to. What is
// not covered there is this table's own integrity — see testutil.CheckSiteTable.
// Every row here is its own domain, so the domain-consistency checks apply too.
func TestSiteTableIntegrity(t *testing.T) {
	rows := make([]testutil.SiteRow, 0, len(sites))
	for _, c := range sites {
		rows = append(rows, testutil.SiteRow{
			ID: c.ID, Base: c.SiteBase, Studio: c.Studio,
			Patterns: c.Patterns, MatchRe: c.MatchRe,
		})
	}
	testutil.CheckSiteTable(t, rows)
	testutil.CheckSiteTableDomains(t, rows)
}
