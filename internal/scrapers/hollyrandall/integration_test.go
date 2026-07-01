//go:build integration

package hollyrandall

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveHollyRandall(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://hollyrandall.com/categories/updates_1_p.html", 3)
}
