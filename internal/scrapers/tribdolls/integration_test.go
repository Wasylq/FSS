//go:build integration

package tribdolls

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveTribDolls(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.trib-dolls.com", 3)
}
