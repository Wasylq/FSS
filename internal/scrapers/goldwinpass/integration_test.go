//go:build integration

package goldwinpass

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveGoldwinPass(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.goldwinpass.com/tour/", 3)
}
