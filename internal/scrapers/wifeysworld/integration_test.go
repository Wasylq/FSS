//go:build integration

package wifeysworld

import (
	"testing"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
)

const liveStudioURL = "https://wifeysworld.com/"

func TestLiveWifeysWorld(t *testing.T) {
	testutil.RunLiveScrape(t, New(), liveStudioURL, 2)
}

// A single product URL skips the store listing.
func TestLiveWifeysWorldProduct(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://wifeysworld.com/store/product.php?slug=adonis-remaster", 1)
}
