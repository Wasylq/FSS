//go:build integration

package teencoreclub

import (
	"testing"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
)

const liveStudioURL = "https://teencoreclub.com"

func TestLiveTeenCoreClub(t *testing.T) {
	testutil.SkipIfPlaceholder(t, liveStudioURL)
	testutil.RunLiveScrape(t, New(), liveStudioURL, 2)
}
