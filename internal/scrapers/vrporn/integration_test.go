//go:build integration

package vrporn

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveVRPornStudio(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://vrporn.com/studio/hotkinkyjo-hkjvr-virtual-reality-vr180/", 3)
}
