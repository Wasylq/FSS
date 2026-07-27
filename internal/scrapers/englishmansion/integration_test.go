//go:build integration

package englishmansion

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

// Limit is deliberately above the site's page size (8) so the scrape
// crosses a page boundary. Every other integration test stops at 2-5
// scenes, so pagination — page-2 URLs, cursor advance, the end-of-list
// signal — is otherwise only ever exercised against fixtures.
func TestLiveEnglishMansion(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.theenglishmansion.com/updates.html", 10)
}
