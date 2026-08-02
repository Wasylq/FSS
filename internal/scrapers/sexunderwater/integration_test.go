//go:build integration

package sexunderwater

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

// 12 > pageSize 10, so this crosses from page 1 to page 2. Chosen because the
// page size is the smallest among the untested-boundary scrapers, making the
// crossing cheap — see T-F1 for why this is a canary subset rather than a sweep.
func TestLiveSexUnderwater(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://sexunderwater.com/categories/SexUnderwater_1_d.html", 12)
}
