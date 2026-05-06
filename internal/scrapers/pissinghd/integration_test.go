//go:build integration

package pissinghd

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLive_PissingHD(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://tour.pissinghd.com/videos", 5)
}
