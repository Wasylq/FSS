//go:build integration

package brutalmaster

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

// New() is used rather than a locally-built config so the test exercises the
// shipped settings — an inline config here previously missed the raised
// Timeout, so the site timed out on every run while the real scraper was fine.
func TestLiveBrutalMaster(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://brutalmaster.com/", 2)
}
