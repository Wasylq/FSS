//go:build integration

package girlsrimming

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveGirlsRimming(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://girlsrimming.com/tour/", 3)
}
