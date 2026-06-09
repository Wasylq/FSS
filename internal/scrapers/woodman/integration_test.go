//go:build integration

package woodman

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveWoodmanCastingX(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.woodmancastingx.com/", 2)
}

func TestLiveWoodmanGirl(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.woodmancastingx.com/girl/scarlett-spark_10518", 2)
}
