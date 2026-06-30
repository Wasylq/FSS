//go:build integration

package bondagecafe

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveBondageCafe(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.bondagecafe.com/updates/page_1.html", 3)
}
