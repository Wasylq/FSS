//go:build integration

package jayspov

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLive(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.jayspov.net/jays-pov-updates.html", 3)
}
