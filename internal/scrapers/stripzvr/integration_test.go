//go:build integration

package stripzvr

import (
	"testing"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
)

func TestLiveStripzVR(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.stripzvr.com/", 2)
}

// A single scene URL is a mode of its own: it skips the sitemap walk.
func TestLiveStripzVRScene(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.stripzvr.com/tiny-tina/cum-back-for-you/", 1)
}
