//go:build integration

package africanbucks

import (
	"testing"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
)

// africanbucks advertises 15 sibling domains, all served by one API whose base
// the scraper derives from the studio URL.
//
// Only one domain is covered live, deliberately. A second-domain test was added
// and then removed: africanfucktour.com took 60s and failed intermittently (this
// API is already the slowest in the suite — see the 90s client timeout in
// New()), so it made smoke runs unreliable for no new signal. The base
// derivation it would have exercised is already covered offline by TestAPIBase.
// If a specific sibling domain is ever suspect, point liveStudioURL at it rather
// than adding a second live test.

const liveStudioURL = "https://africancasting.com/"

func TestLiveAfricanBucks(t *testing.T) {
	testutil.SkipIfPlaceholder(t, liveStudioURL)
	testutil.RunLiveScrape(t, New(), liveStudioURL, 2)
}
