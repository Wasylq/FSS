//go:build integration

package dezyred

import (
	"testing"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
)

func TestIntegration(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://dezyred.com/", 5)
}
