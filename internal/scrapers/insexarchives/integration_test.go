//go:build integration

package insexarchives

import (
	"testing"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
)

// Limit is deliberately above the site's page size (10) so the scrape
// crosses a page boundary. Every other integration test stops at 2-5
// scenes, so pagination — page-2 URLs, cursor advance, the end-of-list
// signal — is otherwise only ever exercised against fixtures.
func TestLiveInsexArchives(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "http://www.insexarchives.com/updates_new.php?start=0", 12)
}
