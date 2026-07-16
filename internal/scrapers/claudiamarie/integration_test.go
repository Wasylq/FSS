//go:build integration

package claudiamarie

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveClaudiaMarie(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://claudiamarie.com/tour/", 3)
}
