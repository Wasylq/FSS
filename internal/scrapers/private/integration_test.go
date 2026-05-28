//go:build integration

package private

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

// Main catalogue (/scenes/) — the workhorse listing every user will hit.
func TestLivePrivate(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.private.com/scenes", 4)
}

// Per-sub-site listing — exercises the /site/{slug}/ path and the
// Series-from-slug derivation.
func TestLivePrivateSite(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.private.com/site/private-stars/", 4)
}

// Sister landing-page domain — exercises the hostRewrite table.
func TestLivePrivateSisterDomain(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://analintroductions.com/", 4)
}
