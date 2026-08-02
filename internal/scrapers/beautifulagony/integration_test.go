//go:build integration

package beautifulagony

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

// 22 > pageSize 20, so this crosses a page boundary.
func TestLiveScrape(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://beautifulagony.com", 22)
}
