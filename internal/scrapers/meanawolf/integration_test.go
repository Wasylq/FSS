//go:build integration

package meanawolf

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveMeanaWolf(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://meanawolf.com/updates/page_1.html", 3)
}
