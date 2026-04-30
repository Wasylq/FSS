//go:build integration

package venusav

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveVenusAVAll(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://venus-av.com/all/", 2)
}

func TestLiveVenusAVNewRelease(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://venus-av.com/new-release/", 2)
}
