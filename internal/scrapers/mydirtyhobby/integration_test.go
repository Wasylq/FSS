//go:build integration

package mydirtyhobby

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

// liveStudioURL — Dirty Tina (long-running profile).
// If gone, swap for any other active MDH profile.
const liveStudioURL = "https://www.mydirtyhobby.com/profil/2517040-Dirty-Tina"

func TestLiveMyDirtyHobby(t *testing.T) {
	testutil.SkipIfPlaceholder(t, liveStudioURL)
	testutil.RunLiveScrape(t, New(), liveStudioURL, 2)
}
