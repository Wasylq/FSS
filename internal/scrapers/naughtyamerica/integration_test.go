//go:build integration

package naughtyamerica

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

// naughtyamerica advertises 7 patterns — the main catalogue, the pornstar mode
// and five sibling domains — but only the pornstar mode was covered. These add
// the main listing and one sibling; the remaining domains differ only by
// hostname and are left out deliberately rather than multiplying smoke runtime.

// liveStudioURL — a real performer with a stable catalogue.
// Pattern: https://www.naughtyamerica.com/pornstar/{slug}
const liveStudioURL = "https://www.naughtyamerica.com/pornstar/cherie-deville"

func TestLiveNaughtyAmerica(t *testing.T) {
	testutil.SkipIfPlaceholder(t, liveStudioURL)
	testutil.RunLiveScrape(t, New(), liveStudioURL, 2)
}

// The main catalogue: the default mode, which a bare studio URL hits.
func TestLiveNaughtyAmericaMainListing(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.naughtyamerica.com/", 2)
}

// A sibling domain, to prove the site filter resolves from the studio URL.
func TestLiveMyFriendsHotMom(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.myfriendshotmom.com/", 2)
}
