//go:build integration

package smqueenroad

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveSMQueenRoad(t *testing.T) {
	const u = "https://www.smqr.com/"
	testutil.SkipIfPlaceholder(t, u)
	testutil.RunLiveScrape(t, New(), u, 2)
}
