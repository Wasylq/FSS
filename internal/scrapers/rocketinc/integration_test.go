//go:build integration

package rocketinc

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

const liveStudioURL = "https://rocket-inc.net/works/"

func TestLiveRocketInc(t *testing.T) {
	testutil.RunLiveScrape(t, New(), liveStudioURL, 2)
}

func TestLiveRocketIncActress(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://rocket-inc.net/works_actress/%e6%96%b0%e6%9d%91%e3%81%82%e3%81%8b%e3%82%8a/", 2)
}
