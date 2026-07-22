//go:build integration

package fetishkitsch

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveFetishKitsch(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://fetishkitsch.com", 3)
}
