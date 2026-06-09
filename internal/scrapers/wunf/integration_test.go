//go:build integration

package wunf

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveWUNF(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.wakeupnfuck.com/", 2)
}
