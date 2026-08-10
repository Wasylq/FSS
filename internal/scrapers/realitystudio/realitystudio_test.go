package realitystudio

import (
	"testing"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
)

// Config-only wrapper keyed on bare hostnames: parsing lives in the *util it
// delegates to, and the SiteBase is built as "https://www." + domain. What is
// not covered elsewhere is this table's integrity — see
// testutil.CheckSiteDomainTable.
func TestSiteTableIntegrity(t *testing.T) {
	rows := make([]testutil.DomainRow, 0, len(sites))
	for _, c := range sites {
		rows = append(rows, testutil.DomainRow{ID: c.id, Domain: c.domain, Studio: c.studio})
	}
	testutil.CheckSiteDomainTable(t, rows)
}
