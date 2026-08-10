//go:build integration

package next11

import (
	"testing"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
)

// Limit is deliberately above the 100-item page size so the scrape crosses a
// page boundary. This is the live regression test for T-F8: the listing
// paginates by POSTed form fields, and while the scraper GET-walked `?pageno=`
// the site returned page one every time, so crossing the boundary produced
// duplicate IDs instead of new scenes.
func TestLiveNext11(t *testing.T) {
	const u = "https://next11.co.jp/"
	testutil.SkipIfPlaceholder(t, u)
	testutil.RunLiveScrape(t, New(), u, 105)
}
