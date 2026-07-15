//go:build integration

package boynapped

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveBoyNapped(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.boynapped.com/", 3)
}
