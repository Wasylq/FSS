//go:build integration

package loyalfans

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveLoyalFans(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.loyalfans.com/bettie_bondage", 2)
}
