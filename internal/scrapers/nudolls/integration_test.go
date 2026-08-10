//go:build integration

package nudolls

import (
	"testing"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
)

func TestLiveNudolls(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://nudolls.com/videos.html", 3)
}
