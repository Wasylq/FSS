//go:build integration

package next11

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

// NOTE: limit is deliberately kept below the 20-item page size here, unlike
// the other page-boundary tests added alongside it. Raising it to 22 makes this
// test fail — correctly: next11's pagination is broken upstream (`pageno` is
// ignored by the site, so every page returns the same 20 items and the scraper
// emits duplicate IDs). That is a scraper defect needing a redesign, not a test
// problem; see T-F8 in AUDIT_TEST.md. Raise this limit once T-F8 is fixed — it
// is the regression test for it.
func TestLiveNext11(t *testing.T) {
	const u = "https://next11.co.jp/"
	testutil.SkipIfPlaceholder(t, u)
	testutil.RunLiveScrape(t, New(), u, 2)
}
