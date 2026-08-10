//go:build integration

package littlemutt

import (
	"testing"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
)

// Limit is deliberately above the site's page size (7) so the scrape
// crosses a page boundary. Every other integration test stops at 2-5
// scenes, so pagination — page-2 URLs, cursor advance, the end-of-list
// signal — is otherwise only ever exercised against fixtures.
func TestLiveLittleMutt(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "http://tour.littlemutt.com/videos/", 9)
}
