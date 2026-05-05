//go:build integration

package wankitnowvr

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveWankItNowVR(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://wankitnowvr.com/", 2)
}
