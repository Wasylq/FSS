//go:build integration

package bbwhighway

import (
	"testing"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
)

func TestLiveBBWHighway(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://bbwhighway.com/tour/", 2)
}
