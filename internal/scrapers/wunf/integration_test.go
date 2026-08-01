//go:build integration

package wunf

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveWUNF(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.wakeupnfuck.com/", 2)
}

// The actor mode dispatches through actorRe to runActor — a separate parser and
// pagination path from the main listing, and previously untested, so a change to
// either the regex or the actor-page markup would have gone unnoticed.
func TestLiveWUNFActor(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.wakeupnfuck.com/actor/dido_angel_3066", 2)
}
