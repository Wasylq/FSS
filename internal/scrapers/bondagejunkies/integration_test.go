//go:build integration

package bondagejunkies

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveBondageJunkies(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://bondagejunkies.com/updates", 3)
}
