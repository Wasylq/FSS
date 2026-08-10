//go:build integration

package queensnake

import (
	"testing"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
)

func TestLiveScrapeAll(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://queensnake.com/", 5)
}
