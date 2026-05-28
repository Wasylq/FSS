//go:build integration

package sindrive

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

// TestLiveSinDrive scrapes the main "videos/all" listing on sindrive.com.
// The same catalogue is also served by sinx.com and madsexparty.com.
func TestLiveSinDrive(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.sindrive.com/videos/all", 4)
}

// TestLiveSinXChannel scrapes a single SinX channel page — the form most of
// the stashdb sub-studios use (e.g. Backstage-Bangers, Bikini-Beach-Balls).
func TestLiveSinXChannel(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.sinx.com/Bikini-Beach-Balls", 4)
}
