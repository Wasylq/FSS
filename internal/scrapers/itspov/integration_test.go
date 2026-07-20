//go:build integration

package itspov

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveItsPOV(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://itspov.com", 3)
}

// The nine brands are collection facets on the main listing, not sites of their
// own — their apex domains serve a splash page with no scene links.
func TestLiveItsPOVChannel(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://itspov.com/channels/intimatepov", 3)
}
