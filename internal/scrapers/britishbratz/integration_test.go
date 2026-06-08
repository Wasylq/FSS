//go:build integration

package britishbratz

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveBritishBratz(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.britishbratz.com/", 3)
}
