//go:build integration

package lovinglyhandmade

import (
	"testing"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
)

func TestLiveScrape(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://lovinglyhandmadepornography.com/", 3)
}
