//go:build integration

package hobybuchanon

import (
	"testing"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
)

func TestLiveHobyBuchanon(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://hobybuchanon.com/", 2)
}

// A model page is a separate mode: one unpaginated payload rather than the
// paginated /updates walk.
func TestLiveHobyBuchanonModel(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://hobybuchanon.com/hobyshotties/rebel-rhyder", 1)
}
