//go:build integration

package fetishnetwork

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveFetishNetwork(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.fetishnetwork.com/t2/show.php?a=1765_1", 2)
}
