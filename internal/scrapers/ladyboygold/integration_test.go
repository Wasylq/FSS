//go:build integration

package ladyboygold

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveLadyboyGold(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://ladyboygold.com", 3)
}
